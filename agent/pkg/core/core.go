package core

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mikrotik-manager/agent/pkg/firebase"
	"mikrotik-manager/pkg/shared"

	"github.com/go-routeros/routeros/v3"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

var (
	sharedClient   *routeros.Client
	sharedClientMu sync.Mutex
	lastTested     time.Time
	// commandMu ensures only one goroutine talks to the MikroTik at a time.
	// The routeros package is NOT thread-safe for concurrent Run calls on a single Client.
	commandMu sync.Mutex
)

func GetSharedClient() (*routeros.Client, error) {
	sharedClientMu.Lock()
	client := sharedClient
	tested := lastTested
	sharedClientMu.Unlock()

	if client != nil {
		// Only test connection if it's been more than 10 seconds since last test
		if time.Since(tested) < 10*time.Second {
			return client, nil
		}

		// Test the connection (thread-safe)
		commandMu.Lock()
		_, err := client.Run("/system/identity/print")
		commandMu.Unlock()

		if err == nil {
			sharedClientMu.Lock()
			lastTested = time.Now()
			sharedClientMu.Unlock()
			return client, nil
		}

		// If test failed, reconnect
		sharedClientMu.Lock()
		if sharedClient == client {
			fmt.Println("[core] Persistent connection lost. Reconnecting...")
			sharedClient.Close()
			sharedClient = nil
		}
		sharedClientMu.Unlock()
	}

	// Second check: try to establish or return existing
	sharedClientMu.Lock()
	defer sharedClientMu.Unlock()

	if sharedClient != nil {
		return sharedClient, nil
	}

	newClient, err := Connect()
	if err != nil {
		return nil, err
	}

	sharedClient = newClient
	lastTested = time.Now()
	fmt.Println("[core] Established new persistent connection to MikroTik.")
	return sharedClient, nil
}

// ResetSharedClient forces the next call to GetSharedClient to create a new connection.
// Useful when router credentials are changed.
func ResetSharedClient() {
	sharedClientMu.Lock()
	defer sharedClientMu.Unlock()
	if sharedClient != nil {
		sharedClient.Close()
		sharedClient = nil
	}
	fmt.Println("[core] Persistent connection reset requested.")
}

func Connect() (*routeros.Client, error) {
	addr := strings.TrimSpace(shared.RouterConfigState.Address)
	if addr == "" {
		addr = strings.TrimSpace(os.Getenv("ROUTER_ADDRESS"))
	}
	if addr == "" {
		addr = strings.TrimSpace(os.Getenv("ROUTER_IP"))
	}
	if addr == "" {
		return nil, fiber.ErrUnauthorized
	}

	user := shared.RouterConfigState.Username
	if user == "" {
		user = os.Getenv("ROUTER_USER")
	}
	pass := shared.RouterConfigState.Password
	if pass == "" {
		pass = os.Getenv("ROUTER_PASS")
	}

	// Try primary address
	client, err := routeros.Dial(addr, user, pass)
	if err == nil && client != nil {
		return client, nil
	}

	// Fallback to standard router gateway if container bridged (e.g. 172.17.0.1, 192.168.88.1)
	host := addr
	port := "8728"
	if strings.Contains(addr, ":") {
		h, p, e := net.SplitHostPort(addr)
		if e == nil {
			host = h
			port = p
		}
	}

	for _, fbHost := range []string{"172.17.0.1", "192.168.88.1", "127.0.0.1", "172.16.0.1"} {
		if fbHost == host {
			continue
		}
		fbAddr := net.JoinHostPort(fbHost, port)
		fbClient, fbErr := routeros.Dial(fbAddr, user, pass)
		if fbErr == nil && fbClient != nil {
			log.Printf("[core] Connected to fallback RouterOS API at %s", fbAddr)
			return fbClient, nil
		}
	}

	return nil, err
}

func ConnectOrReply(c *fiber.Ctx) (*routeros.Client, bool) {
	client, err := GetSharedClient()
	if err != nil {
		if err == fiber.ErrUnauthorized {
			_ = c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":         "No router config provided. Login first.",
				"authenticated": false,
			})
			return nil, true
		}
		_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		return nil, true
	}
	return client, false
}

func RestrictedConnectOrReply(c *fiber.Ctx) (*routeros.Client, bool) {
	client, err := GetSharedClient()
	if err != nil {
		if err == fiber.ErrUnauthorized {
			_ = c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":         "No router config provided. Login first.",
				"authenticated": false,
			})
			return nil, true
		}
		_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		return nil, true
	}

	serial, err := GetRouterSerial(client)
	if err != nil {
		// Do NOT close shared client here
		_ = c.Status(500).JSON(fiber.Map{"error": "فشل التحقق من هوية الراوتر: " + err.Error()})
		return nil, true
	}

	valid, msg, _ := VerifyLicense(shared.RouterConfigState.License, serial)
	if !valid {
		// Do NOT close shared client here
		_ = c.Status(403).JSON(fiber.Map{
			"error":            msg,
			"license_required": true,
		})
		return nil, true
	}

	return client, false
}

func SafeRun(client *routeros.Client, args ...string) (*routeros.Reply, error) {
	commandMu.Lock()
	defer commandMu.Unlock()

	reply, err := client.Run(args...)
	if err != nil && strings.Contains(err.Error(), "!empty") {
		return &routeros.Reply{}, nil
	}
	return reply, err
}

func GetRouterSerial(client *routeros.Client) (string, error) {
	commandMu.Lock()
	res, err := client.Run("/system/routerboard/print")
	commandMu.Unlock()

	if err != nil {
		return "", err
	}
	if res == nil {
		return "", fmt.Errorf("no response from router")
	}
	if len(res.Re) > 0 {
		return res.Re[0].Map["serial-number"], nil
	}
	return "", fmt.Errorf("could not find serial number")
}

func VerifyLicense(licenseKey string, currentSerial string) (bool, string, time.Time) {
	// 1. Check Central Cloud-Managed License Lease
	if shared.RouterConfigState.CloudLicenseStatus != "" || shared.RouterConfigState.TunnelSubdomain != "" {
		if shared.RouterConfigState.CloudLicenseStatus == "" {
			licPath := filepath.Join(shared.GetDataDir(), "cloud_license.json")
			if data, err := os.ReadFile(licPath); err == nil {
				var lease struct {
					Status        string `json:"status"`
					ExpiresAt     string `json:"expires_at"`
					DaysRemaining int    `json:"days_remaining"`
					IsExpired     bool   `json:"is_expired"`
					Valid         bool   `json:"valid"`
				}
				if json.Unmarshal(data, &lease) == nil {
					shared.RouterConfigState.CloudLicenseStatus = lease.Status
					shared.RouterConfigState.CloudLicenseExpiresAt = lease.ExpiresAt
					shared.RouterConfigState.CloudLicenseDaysLeft = lease.DaysRemaining
					shared.RouterConfigState.CloudLicenseValid = lease.Valid
				}
			}
		}

		if shared.RouterConfigState.CloudLicenseStatus == "suspended" {
			return false, "تم إيقاف وتجميد اشتراك هذا الوكيل من قبل الإدارة المركزية", time.Time{}
		}

		if shared.RouterConfigState.CloudLicenseExpiresAt != "" {
			var expTime time.Time
			var err error
			expTime, err = time.Parse("2006-01-02 15:04:05", shared.RouterConfigState.CloudLicenseExpiresAt)
			if err != nil {
				expTime, err = time.Parse(time.RFC3339, shared.RouterConfigState.CloudLicenseExpiresAt)
			}
			if err == nil {
				if time.Now().UTC().After(expTime.UTC()) {
					return false, "انتهت صلاحية اشتراك هذا الحساب، يرجى التجديد من الإدارة", expTime
				}
				if shared.RouterConfigState.CloudLicenseStatus == "active" || shared.RouterConfigState.CloudLicenseValid {
					return true, "اشتراك سحابي مفعل", expTime
				}
			}
		}

		if shared.RouterConfigState.CloudLicenseStatus == "active" {
			return true, "اشتراك سحابي مفعل", time.Now().Add(30 * 24 * time.Hour)
		}
	}

	// 2. Fallback to Offline JWT Key
	if licenseKey == "" {
		return false, "نظام غير مفعل", time.Time{}
	}

	pubKeyBytes, _ := hex.DecodeString(shared.PUBLIC_KEY_HEX)
	publicKey := ed25519.PublicKey(pubKeyBytes)

	token, err := jwt.Parse(licenseKey, func(token *jwt.Token) (interface{}, error) {
		if token.Method.Alg() != "EdDSA" {
			return nil, fmt.Errorf("طريقة التوقيع غير متوافقة: %v", token.Header["alg"])
		}
		return publicKey, nil
	})

	if err != nil {
		return false, "مفتاح التنشيط غير صالح", time.Time{}
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		licenseSerial, _ := claims["serial"].(string)
		if licenseSerial != currentSerial && currentSerial != "" {
			return false, "هذا المفتاح مخصص لجهاز آخر (Serial Mismatch)", time.Time{}
		}

		expFloat, _ := claims["exp"].(float64)
		expTime := time.Unix(int64(expFloat), 0)
		if time.Now().After(expTime) {
			return false, "انتهت صلاحية المفتاح", expTime
		}

		return true, "نظام مفعل", expTime
	}

	return false, "بيانات الترخيص غير صالحة", time.Time{}
}

func GetLicenseStatus(c *fiber.Ctx) error {
	client, responded := ConnectOrReply(c)
	if responded {
		return nil
	}

	serial, _ := GetRouterSerial(client)
	valid, msg, exp := VerifyLicense(shared.RouterConfigState.License, serial)

	expStr := ""
	if !exp.IsZero() {
		expStr = exp.Format("2006-01-02 15:04:05")
	} else if shared.RouterConfigState.CloudLicenseExpiresAt != "" {
		expStr = shared.RouterConfigState.CloudLicenseExpiresAt
	}

	return c.JSON(fiber.Map{
		"valid":          valid,
		"message":        msg,
		"expires":        expStr,
		"serial":         serial,
		"status":         shared.RouterConfigState.CloudLicenseStatus,
		"days_remaining": shared.RouterConfigState.CloudLicenseDaysLeft,
	})
}

func ActivateLicense(c *fiber.Ctx) error {
	type Request struct {
		Key string `json:"key"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	client, responded := ConnectOrReply(c)
	if responded {
		return nil
	}

	serial, _ := GetRouterSerial(client)
	valid, msg, _ := VerifyLicense(req.Key, serial)

	if !valid {
		return c.Status(403).JSON(fiber.Map{"error": msg})
	}

	shared.RouterConfigState.License = req.Key
	shared.RouterConfigState.Serial = serial
	shared.SaveConfig()
	firebase.SyncAsync("core_license_activated", firebase.RemoteAccess{})

	return c.JSON(fiber.Map{"message": "تم تفعيل النظام بنجاح!"})
}

// GetClientWithAuth returns a working routeros.Client, trying the shared client first, then fallback to provided credentials
func GetClientWithAuth(host, user, pass string) (*routeros.Client, func(), error) {
	client, err := GetSharedClient()
	if err == nil && client != nil {
		return client, func() {}, nil
	}

	// Try with provided explicit credentials
	if user != "" {
		targetHost := host
		if targetHost == "" {
			targetHost = strings.TrimSpace(shared.RouterConfigState.Address)
		}
		if targetHost == "" {
			targetHost = "192.168.88.1"
		}

		newClient, dialErr := routeros.Dial(targetHost, user, pass)
		if dialErr == nil && newClient != nil {
			// Update local config if successful
			shared.RouterConfigState.Address = targetHost
			shared.RouterConfigState.Username = user
			shared.RouterConfigState.Password = pass
			shared.SaveConfig()
			ResetSharedClient()
			return newClient, func() { newClient.Close() }, nil
		}

		// Also try standard fallbacks
		for _, fb := range []string{"172.17.0.1", "192.168.88.1", "127.0.0.1", "172.16.0.1"} {
			if fb == targetHost {
				continue
			}
			fbClient, fbErr := routeros.Dial(fb, user, pass)
			if fbErr == nil && fbClient != nil {
				shared.RouterConfigState.Address = fb
				shared.RouterConfigState.Username = user
				shared.RouterConfigState.Password = pass
				shared.SaveConfig()
				ResetSharedClient()
				return fbClient, func() { fbClient.Close() }, nil
			}
		}
	}

	return nil, func() {}, fmt.Errorf("router connection error: %w", err)
}

// RunCommand executes a single RouterOS command safely with commandMu and returns structured maps
func RunCommand(sentence ...string) ([]map[string]string, error) {
	return RunCommandWithAuth("", "", "", sentence...)
}

// RunCommandWithAuth executes a single RouterOS command with fallback credentials
func RunCommandWithAuth(host, user, pass string, sentence ...string) ([]map[string]string, error) {
	client, cleanup, err := GetClientWithAuth(host, user, pass)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	commandMu.Lock()
	reply, err := client.Run(sentence...)
	commandMu.Unlock()

	if err != nil {
		return nil, err
	}

	var results []map[string]string
	for _, re := range reply.Re {
		item := make(map[string]string)
		for _, pair := range re.List {
			item[pair.Key] = pair.Value
		}
		results = append(results, item)
	}
	return results, nil
}

// RunCommandsBatch executes multiple RouterOS commands and returns their combined results
func RunCommandsBatch(commands [][]string) (map[string]interface{}, error) {
	return RunCommandsBatchWithAuth("", "", "", commands)
}

// RunCommandsBatchWithAuth executes multiple RouterOS commands with fallback credentials
func RunCommandsBatchWithAuth(host, user, pass string, commands [][]string) (map[string]interface{}, error) {
	client, cleanup, err := GetClientWithAuth(host, user, pass)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	commandMu.Lock()
	defer commandMu.Unlock()

	output := make(map[string]interface{})
	for idx, cmd := range commands {
		if len(cmd) == 0 {
			continue
		}
		cmdKey := strings.Join(cmd, " ")
		if len(commands) > 1 {
			cmdKey = fmt.Sprintf("cmd_%d", idx)
		}

		reply, err := client.Run(cmd...)
		if err != nil {
			output[cmdKey] = map[string]interface{}{
				"command": cmd,
				"error":   err.Error(),
			}
			continue
		}

		var items []map[string]string
		for _, re := range reply.Re {
			item := make(map[string]string)
			for _, pair := range re.List {
				item[pair.Key] = pair.Value
			}
			items = append(items, item)
		}

		output[cmdKey] = map[string]interface{}{
			"command": cmd,
			"items":   items,
			"count":   len(items),
		}
	}

	return output, nil
}

// RunSystemAudit gathers a complete RouterOS health, firewall, resource, and interface audit snapshot
func RunSystemAudit() (map[string]interface{}, error) {
	return RunSystemAuditWithAuth("", "", "")
}

// RunSystemAuditWithAuth gathers a complete RouterOS snapshot with fallback credentials
func RunSystemAuditWithAuth(host, user, pass string) (map[string]interface{}, error) {
	client, cleanup, err := GetClientWithAuth(host, user, pass)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	commandMu.Lock()
	defer commandMu.Unlock()

	audit := make(map[string]interface{})

	queries := map[string][]string{
		"resource":        {"/system/resource/print"},
		"identity":        {"/system/identity/print"},
		"routerboard":     {"/system/routerboard/print"},
		"interfaces":      {"/interface/print"},
		"ip_addresses":    {"/ip/address/print"},
		"firewall_filter": {"/ip/firewall/filter/print"},
		"firewall_nat":    {"/ip/firewall/nat/print"},
		"firewall_mangle": {"/ip/firewall/mangle/print"},
		"dhcp_servers":    {"/ip/dhcp-server/print"},
		"dhcp_leases":     {"/ip/dhcp-server/lease/print"},
		"dns":             {"/ip/dns/print"},
		"ip_services":     {"/ip/service/print"},
		"queues":          {"/queue/simple/print"},
		"logs":            {"/log/print"},
	}

	for key, cmd := range queries {
		reply, err := client.Run(cmd...)
		if err != nil {
			audit[key] = []map[string]string{}
			continue
		}
		var items []map[string]string
		for _, re := range reply.Re {
			item := make(map[string]string)
			for _, pair := range re.List {
				item[pair.Key] = pair.Value
			}
			items = append(items, item)
		}
		audit[key] = items
	}

	return audit, nil
}
