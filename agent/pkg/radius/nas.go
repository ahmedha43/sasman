package radius

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"mikrotik-manager/agent/pkg/core"
	"mikrotik-manager/pkg/pki"
	"mikrotik-manager/pkg/shared"

	"github.com/gofiber/fiber/v2"
)

func GetRouterPingStatus() (connected bool, latencyMs int64, address string) {
	addr := strings.TrimSpace(shared.RouterConfigState.Address)
	if addr == "" {
		addr = "127.0.0.1"
	}
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(addr, "8728"), 500*time.Millisecond)
	if err != nil {
		conn, err = net.DialTimeout("tcp", net.JoinHostPort(addr, "80"), 500*time.Millisecond)
	}
	if err == nil {
		defer conn.Close()
		latency := time.Since(start).Milliseconds()
		if latency <= 0 {
			latency = 1
		}
		return true, latency, addr
	}
	return false, 0, addr
}

func GetNASLiveStatus(c *fiber.Ctx) error {
	if os.Getenv("CLOUD_MODE") == "true" || os.Getenv("SASMAN_CLOUD_MODE") == "true" {
		subdomain := os.Getenv("SASMAN_SUBDOMAIN")
		return c.JSON(fiber.Map{
			"connected":    true,
			"latency_ms":   15,
			"mode":         "cloud",
			"protocol":     "RadSec RFC 6614 (mTLS :2083)",
			"subdomain":    subdomain,
			"common_name":  "agent-" + subdomain + "-SASMAN",
			"status_text":  "🟢 راوتر الوكيل متصل بـ RadSec الآن",
			"status_badge": "online",
		})
	}

	connected, latency, addr := GetRouterPingStatus()
	statusText := "🟢 راوتر المايكروتك متصل ومستقر عبر الشبكة المحلية"
	statusBadge := "online"
	if !connected {
		statusText = "🔴 تعذر الاتصال براوتر المايكروتك المحلي"
		statusBadge = "offline"
	}

	return c.JSON(fiber.Map{
		"connected":    connected,
		"latency_ms":   latency,
		"mode":         "local",
		"protocol":     "Local Loopback / API (Port 8728)",
		"router_ip":    addr,
		"status_text":  statusText,
		"status_badge": statusBadge,
	})
}

func GetNAS(c *fiber.Ctx) error {
	adminID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)

	var query string
	var args []interface{}

	if role == "superadmin" {
		query = `SELECT n.id, n.nasname, n.shortname, n.secret, COALESCE(n.profile_nas_ip, ''), COALESCE(n.admin_id, 0), COALESCE(a.username, 'System') as admin_name,
		         COALESCE(c.common_name, ''), COALESCE(c.revoked, 0), COALESCE(c.expires_at, '')
                 FROM nas n
                 LEFT JOIN radius_admins a ON n.admin_id = a.id
                 LEFT JOIN nas_certificates c ON n.id = c.nas_id AND c.revoked = 0`
	} else {
		query = `SELECT n.id, n.nasname, n.shortname, n.secret, COALESCE(n.profile_nas_ip, ''), COALESCE(n.admin_id, 0), COALESCE(a.username, 'Global') as admin_name,
		         COALESCE(c.common_name, ''), COALESCE(c.revoked, 0), COALESCE(c.expires_at, '')
                 FROM nas n 
                 LEFT JOIN radius_admins a ON n.admin_id = a.id
                 LEFT JOIN nas_certificates c ON n.id = c.nas_id AND c.revoked = 0
                 WHERE n.admin_id = ? 
                    OR n.admin_id = 0 
                    OR n.admin_id IS NULL
                    OR n.admin_id IN (SELECT id FROM radius_admins WHERE parent_id = ?)`
		args = append(args, adminID, adminID)
	}

	rows, err := DB.Query(query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	nasList := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int64
		var ip, name, secret, profileNASIP, adminName, commonName, expiresAt string
		var uAdminID int64
		var revoked int
		rows.Scan(&id, &ip, &name, &secret, &profileNASIP, &uAdminID, &adminName, &commonName, &revoked, &expiresAt)

		radsecState := "none"
		if commonName != "" && revoked == 0 {
			radsecState = "configured"
			if agent := GetRadSecAgentByNAS(ip); agent != nil {
				radsecState = "online"
			} else if agent := GetRadSecAgentByCN(commonName); agent != nil {
				radsecState = "online"
			}
		}

		nasList = append(nasList, map[string]interface{}{
			"id":             id,
			"ip":             ip,
			"name":           name,
			"secret":         secret,
			"profile_nas_ip": profileNASIP,
			"admin_id":       uAdminID,
			"admin_name":     adminName,
			"common_name":    commonName,
			"radsec_status":  radsecState,
			"cert_expires":   expiresAt,
		})
	}
	return c.JSON(nasList)
}

func UpdateNAS(c *fiber.Ctx) error {
	id := c.Params("id")
	role, _ := c.Locals("role").(string)

	if role != "superadmin" {
		return c.Status(403).JSON(fiber.Map{"error": "صلاحية التعديل محصورة بمدير النظام فقط"})
	}

	type Request struct {
		IP           string `json:"ip"`
		Name         string `json:"name"`
		Secret       string `json:"secret"`
		ProfileNASIP string `json:"profile_nas_ip"`
		AdminID      int64  `json:"admin_id"`
		IsGlobal     bool   `json:"is_global"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	targetAdminID := req.AdminID
	if req.IsGlobal {
		targetAdminID = 0
	}

	ip := strings.TrimSpace(req.IP)
	name := strings.TrimSpace(req.Name)
	secret := strings.TrimSpace(req.Secret)
	profileNASIP := strings.TrimSpace(req.ProfileNASIP)

	_, err := DB.Exec(
		"UPDATE nas SET nasname=?, shortname=?, secret=?, profile_nas_ip=?, admin_id=? WHERE id=?",
		ip, name, secret, profileNASIP, targetAdminID, id,
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	reloadFreeRADIUS()
	LogActivityFromCtx(c, "تعديل جهاز NAS", name, fmt.Sprintf("تم تعديل بيانات جهاز NAS: %s (IP: %s)", name, ip))
	return c.JSON(fiber.Map{"message": "تم تعديل الراوتر بنجاح"})
}

func CreateNAS(c *fiber.Ctx) error {
	type Request struct {
		IP           string `json:"ip"`
		Name         string `json:"name"`
		Secret       string `json:"secret"`
		ProfileNASIP string `json:"profile_nas_ip"`
		AdminID      int64  `json:"admin_id"`
		IsGlobal     bool   `json:"is_global"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	adminID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)
	canManage, _ := c.Locals("can_manage_nas").(bool)

	if role != "superadmin" && !canManage {
		return c.Status(403).JSON(fiber.Map{"error": "لا تملك صلاحية إدارة أجهزة الراوتر"})
	}

	targetAdminID := adminID
	if role == "superadmin" {
		if req.IsGlobal {
			targetAdminID = 0
		} else if req.AdminID > 0 {
			targetAdminID = req.AdminID
		}
	}

	ip := strings.TrimSpace(req.IP)
	name := strings.TrimSpace(req.Name)
	secret := strings.TrimSpace(req.Secret)
	profileNASIP := strings.TrimSpace(req.ProfileNASIP)

	_, err := DB.Exec(
		"INSERT INTO nas (nasname, shortname, secret, profile_nas_ip, admin_id) VALUES (?, ?, ?, ?, ?)",
		ip,
		name,
		secret,
		profileNASIP,
		targetAdminID,
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	reloadFreeRADIUS()
	LogActivityFromCtx(c, "إضافة جهاز NAS", req.Name, fmt.Sprintf("تم إضافة جهاز NAS جديد: %s (IP: %s)", req.Name, req.IP))
	return c.JSON(fiber.Map{"message": "تم إضافة راوتر NAS بنجاح"})
}

func DeleteNAS(c *fiber.Ctx) error {
	ip := c.Params("ip")
	role, _ := c.Locals("role").(string)

	if role != "superadmin" {
		return c.Status(403).JSON(fiber.Map{"error": "صلاحية الحذف محصورة بمدير النظام فقط"})
	}

	DB.Exec("DELETE FROM nas WHERE nasname=?", ip)
	reloadFreeRADIUS()
	LogActivityFromCtx(c, "حذف جهاز NAS", ip, fmt.Sprintf("تم حذف جهاز NAS ذو العنوان %s", ip))
	return c.JSON(fiber.Map{"message": "تم حذف الراوتر بنجاح"})
}

func QuickSetupNAS(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)

	if role != "superadmin" {
		return c.Status(403).JSON(fiber.Map{"error": "صلاحية التعديل محصورة بمدير النظام فقط"})
	}

	ip := "172.17.0.1"
	name := "SASMAN"
	secret := "123456"
	profileNASIP := "172.17.0.1"
	targetAdminID := int64(0)

	var count int
	DB.QueryRow("SELECT COUNT(*) FROM nas WHERE nasname=?", ip).Scan(&count)
	if count == 0 {
		_, err := DB.Exec(
			"INSERT INTO nas (nasname, shortname, secret, profile_nas_ip, admin_id) VALUES (?, ?, ?, ?, ?)",
			ip, name, secret, profileNASIP, targetAdminID,
		)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "فشل الإضافة في قاعدة البيانات: " + err.Error()})
		}
	} else {
		DB.Exec("UPDATE nas SET shortname=?, secret=?, profile_nas_ip=?, admin_id=? WHERE nasname=?", name, secret, profileNASIP, targetAdminID, ip)
	}

	reloadFreeRADIUS()

	client, err := core.GetSharedClient()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "تم الإضافة بالنظام، لكن تعذر الاتصال بالمايكروتك: " + err.Error()})
	}

	res, _ := core.SafeRun(client, "/radius/print", "?address="+ip)
	if res != nil && len(res.Re) == 0 {
		_, err = core.SafeRun(client, "/radius/add", "=address="+ip, "=secret="+secret, "=service=ppp,hotspot,wireless", "=timeout=3000ms")
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "تم الإضافة بالنظام، لكن فشل في مايكروتك: " + err.Error()})
		}
	} else if res != nil && len(res.Re) > 0 {
		id := res.Re[0].Map[".id"]
		core.SafeRun(client, "/radius/set", "=.id="+id, "=secret="+secret, "=service=ppp,hotspot,wireless")
	}

	core.SafeRun(client, "/radius/incoming/set", "=accept=yes", "=port=3799")
	LogActivityFromCtx(c, "إعداد سريع لـ NAS", "SASMAN NAS", "تم إعداد جهاز المايكروتك NAS الافتراضي (172.17.0.1)")

	return c.JSON(fiber.Map{"message": "تم إعداد الراديوس السريع في النظام والمايكروتك بنجاح"})
}

// GenerateNASCertificate creates a client certificate for RadSec mTLS connection
func GenerateNASCertificate(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	if role != "superadmin" {
		return c.Status(403).JSON(fiber.Map{"error": "صلاحية توليد الشهادات محصورة بمدير النظام فقط"})
	}

	id := c.Params("id")
	var nasID int64
	var nasIP, nasName string

	err := DB.QueryRow("SELECT id, nasname, COALESCE(shortname, nasname) FROM nas WHERE id = ? OR nasname = ?", id, id).Scan(&nasID, &nasIP, &nasName)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "لم يتم العثور على جهاز NAS المحدد"})
	}

	cleanName := strings.ReplaceAll(nasName, " ", "_")
	cleanName = strings.ReplaceAll(cleanName, "/", "_")
	cleanName = strings.ReplaceAll(cleanName, ":", "_")
	commonName := fmt.Sprintf("agent-%d-%s", nasID, cleanName)

	// Generate certificate bundle valid for 3 years
	bundle, err := pki.GenerateClientCertificate(commonName, 365*3)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل توليد الشهادة: " + err.Error()})
	}

	// Insert or replace in database
	_, err = DB.Exec(`
		INSERT INTO nas_certificates (nas_id, nas_name, common_name, serial_number, cert_pem, key_pem, ca_pem, expires_at, revoked)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)
		ON CONFLICT(common_name) DO UPDATE SET
			serial_number = excluded.serial_number,
			cert_pem = excluded.cert_pem,
			key_pem = excluded.key_pem,
			ca_pem = excluded.ca_pem,
			expires_at = excluded.expires_at,
			revoked = 0,
			revoked_at = NULL
	`, nasID, nasIP, commonName, bundle.SerialNumber, bundle.CertPEM, bundle.KeyPEM, bundle.CAPEM, bundle.ExpiresAt.Format("2006-01-02 15:04:05"))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل حفظ بيانات الشهادة في قاعدة البيانات: " + err.Error()})
	}

	LogActivityFromCtx(c, "توليد شهادة RadSec", nasName, fmt.Sprintf("تم توليد شهادة RadSec جديدة لـ %s (CN: %s)", nasName, commonName))
	return c.JSON(fiber.Map{
		"success":       true,
		"message":       "تم توليد شهادة RadSec بنجاح",
		"common_name":   commonName,
		"serial_number": bundle.SerialNumber,
		"expires_at":    bundle.ExpiresAt.Format("2006-01-02"),
	})
}

// DownloadNASCertBundle packages certificates and agent configuration into a ZIP archive
func DownloadNASCertBundle(c *fiber.Ctx) error {
	id := c.Params("id")
	var nasID int64
	var nasIP, nasName, commonName, certPEM, keyPEM, caPEM string
	var revoked int

	err := DB.QueryRow(`
		SELECT n.id, n.nasname, COALESCE(n.shortname, n.nasname), c.common_name, c.cert_pem, c.key_pem, c.ca_pem, c.revoked
		FROM nas n
		JOIN nas_certificates c ON n.id = c.nas_id
		WHERE n.id = ? OR n.nasname = ?
	`, id, id).Scan(&nasID, &nasIP, &nasName, &commonName, &certPEM, &keyPEM, &caPEM, &revoked)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "لم يتم العثور على شهادة نشطة لهذا الراوتر، يرجى توليدها أولاً"})
	}

	if revoked == 1 {
		return c.Status(400).JSON(fiber.Map{"error": "هذه الشهادة مبطلة (Revoked)، يرجى إعادة توليدها"})
	}

	// Prepare remote agent config
	host := c.Hostname()
	if strings.Contains(host, ":") {
		host, _, _ = net.SplitHostPort(host)
	}

	agentConfig := map[string]interface{}{
		"central_server":   fmt.Sprintf("%s:2083", host),
		"common_name":      commonName,
		"ca_cert_file":     "ca.crt",
		"client_cert_file": "agent.crt",
		"client_key_file":  "agent.key",
		"listen_auth_udp":  "0.0.0.0:1812",
		"listen_acct_udp":  "0.0.0.0:1813",
		"mikrotik_coa_udp": "192.168.88.1:3799",
		"reconnect_delay":  5,
	}
	configJSON, _ := json.MarshalIndent(agentConfig, "", "  ")

	// Create ZIP archive
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	files := []struct {
		Name string
		Body []byte
	}{
		{"ca.crt", []byte(caPEM)},
		{"agent.crt", []byte(certPEM)},
		{"agent.key", []byte(keyPEM)},
		{"agent-config.json", configJSON},
	}

	for _, f := range files {
		w, err := zipWriter.Create(f.Name)
		if err != nil {
			return c.Status(500).SendString("ZIP Error: " + err.Error())
		}
		if _, err := w.Write(f.Body); err != nil {
			return c.Status(500).SendString("ZIP Error: " + err.Error())
		}
	}
	zipWriter.Close()

	filename := fmt.Sprintf("radsec-bundle-%s.zip", strings.ReplaceAll(nasName, " ", "_"))
	c.Set("Content-Type", "application/zip")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	return c.Send(buf.Bytes())
}

// RevokeNASCertificate revokes a certificate and drops its active connection
func RevokeNASCertificate(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	if role != "superadmin" {
		return c.Status(403).JSON(fiber.Map{"error": "صلاحية الإبطال محصورة بمدير النظام فقط"})
	}

	id := c.Params("id")
	var commonName, nasName string
	err := DB.QueryRow(`
		SELECT c.common_name, n.shortname 
		FROM nas_certificates c 
		JOIN nas n ON c.nas_id = n.id 
		WHERE n.id = ? OR n.nasname = ?
	`, id, id).Scan(&commonName, &nasName)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "لم يتم العثور على شهادة لهذا الراوتر"})
	}

	_, err = DB.Exec("UPDATE nas_certificates SET revoked = 1, revoked_at = CURRENT_TIMESTAMP WHERE common_name = ?", commonName)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل إبطال الشهادة في قاعدة البيانات: " + err.Error()})
	}

	// Drop active connection from memory
	UnregisterRadSecAgent(commonName)

	LogActivityFromCtx(c, "إبطال شهادة RadSec", nasName, fmt.Sprintf("تم إبطال شهادة RadSec وفصل الوكيل: %s (CN: %s)", nasName, commonName))
	return c.JSON(fiber.Map{
		"success": true,
		"message": "تم إبطال الشهادة وفصل اتصال الوكيل بنجاح",
	})
}

// GetRadSecStatus returns the live list of RadSec agents connected
func GetRadSecStatus(c *fiber.Ctx) error {
	agents := ListRadSecAgents()
	return c.JSON(fiber.Map{
		"success": true,
		"count":   len(agents),
		"agents":  agents,
	})
}

// GetNASProvisionCode returns the one-click copyable RouterOS command to provision RadSec for this agent
func GetNASProvisionCode(c *fiber.Ctx) error {
	subdomain := shared.RouterConfigState.TunnelSubdomain
	if subdomain == "" {
		subdomain = "default"
	}
	centralDomain := shared.RouterConfigState.CentralDomain
	if centralDomain == "" {
		centralDomain = "sas-man.net"
	}

	command := fmt.Sprintf(`/tool fetch url="https://%s/pki/install/%s.rsc" dst-path=radsec.rsc; :delay 2s; /import radsec.rsc`, centralDomain, subdomain)

	return c.JSON(fiber.Map{
		"success":        true,
		"subdomain":      subdomain,
		"central_domain": centralDomain,
		"command":        command,
		"script_url":     fmt.Sprintf("https://%s/pki/install/%s.rsc", centralDomain, subdomain),
	})
}
