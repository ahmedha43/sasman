package firebase

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"mikrotik-manager/pkg/shared"
)

type RemoteAccess struct {
	NgrokWebURL   string `json:"ngrok_web_url,omitempty"`
	NgrokTCPURL   string `json:"ngrok_tcp_url,omitempty"`
}

type remoteAccessProvider func() RemoteAccess

var httpClient = &http.Client{Timeout: 10 * time.Second}

func Enabled() bool {
	cfg := loadConfig()
	return cfg.Enabled && cfg.DatabaseURL != ""
}

func StartBackgroundSync(provider remoteAccessProvider) {
	if !Enabled() {
		return
	}

	go func() {
		time.Sleep(20 * time.Second)
		Sync("container_started", provider())

		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			Sync("heartbeat", provider())
		}
	}()
}

func SyncAsync(event string, remote RemoteAccess) {
	if !Enabled() {
		return
	}
	go Sync(event, remote)
}

func Sync(event string, remote RemoteAccess) {
	if !Enabled() {
		return
	}

	payload := buildPayload(event, remote)
	installationID := documentID(payload)
	if installationID == "" {
		log.Printf("[Firebase] skipped sync: missing installation id")
		return
	}

	if err := patchRealtimeDatabase(installationID, payload); err != nil {
		log.Printf("[Firebase] sync failed: %v", err)
		return
	}
	log.Printf("[Firebase] synced installation %s (%s)", installationID, event)
}

func buildPayload(event string, remote RemoteAccess) map[string]interface{} {
	now := time.Now().UTC()
	hostname, _ := os.Hostname()
	routerHost, routerPort := splitHostPort(shared.RouterConfigState.Address)
	license := parseLicense(shared.RouterConfigState.License)

	licensePayload := map[string]interface{}{
		"present":        shared.RouterConfigState.License != "",
		"serial":         firstNonEmpty(license.Serial, shared.RouterConfigState.Serial),
		"issued_at":      formatUnix(license.IssuedAt),
		"expires_at":     formatUnix(license.ExpiresAt),
		"expires_unix":   license.ExpiresAt,
		"is_expired":     license.ExpiresAt > 0 && now.After(time.Unix(license.ExpiresAt, 0)),
		"license_sha256": sha256Hex(shared.RouterConfigState.License),
	}

	cfg := loadConfig()
	if cfg.UploadSecrets {
		licensePayload["license_key"] = shared.RouterConfigState.License
	}

	mikrotikPayload := map[string]interface{}{
		"address":  shared.RouterConfigState.Address,
		"host":     routerHost,
		"api_port": routerPort,
		"username": shared.RouterConfigState.Username,
		"serial":   shared.RouterConfigState.Serial,
	}
	if cfg.UploadSecrets {
		mikrotikPayload["password"] = shared.RouterConfigState.Password
	}

	return map[string]interface{}{
		"installation_id": firstNonEmpty(shared.RouterConfigState.Serial, hostname),
		"last_event":      event,
		"updated_at":      now.Format(time.RFC3339),
		"app": map[string]interface{}{
			"name":    "SASMAN MikroTik Manager",
			"version": firstNonEmpty(os.Getenv("SASMAN_VERSION"), "v5"),
		},
		"container": map[string]interface{}{
			"hostname":     hostname,
			"container_id": hostname,
			"image":        os.Getenv("SASMAN_IMAGE"),
		},
		"mikrotik":      mikrotikPayload,
		"license":       licensePayload,
		"remote_access": remote,
	}
}

func patchRealtimeDatabase(id string, payload map[string]interface{}) error {
	cfg := loadConfig()
	baseURL := strings.TrimRight(cfg.DatabaseURL, "/")
	collection := strings.Trim(cfg.Collection, "/")
	if collection == "" {
		collection = "sasman_installations"
	}

	endpoint := fmt.Sprintf("%s/%s/%s.json", baseURL, url.PathEscape(collection), url.PathEscape(id))
	if token := cfg.AuthToken; token != "" {
		endpoint += "?auth=" + url.QueryEscape(token)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("realtime database returned %s", resp.Status)
	}
	return nil
}

type parsedLicense struct {
	Serial    string
	IssuedAt  int64
	ExpiresAt int64
}

func parseLicense(key string) parsedLicense {
	if strings.TrimSpace(key) == "" {
		return parsedLicense{}
	}

	claims := jwt.MapClaims{}
	_, _, err := new(jwt.Parser).ParseUnverified(key, claims)
	if err != nil {
		return parsedLicense{}
	}

	return parsedLicense{
		Serial:    claimString(claims, "serial"),
		IssuedAt:  claimUnix(claims, "iat"),
		ExpiresAt: claimUnix(claims, "exp"),
	}
}

func claimString(claims jwt.MapClaims, key string) string {
	if v, ok := claims[key].(string); ok {
		return v
	}
	return ""
}

func claimUnix(claims jwt.MapClaims, key string) int64 {
	switch v := claims[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case json.Number:
		n, _ := v.Int64()
		return n
	default:
		return 0
	}
}

func formatUnix(value int64) string {
	if value <= 0 {
		return ""
	}
	return time.Unix(value, 0).UTC().Format(time.RFC3339)
}

func splitHostPort(address string) (string, string) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", ""
	}
	if host, port, err := net.SplitHostPort(address); err == nil {
		return host, port
	}
	parts := strings.Split(address, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return address, ""
}

func documentID(payload map[string]interface{}) string {
	if raw, ok := payload["installation_id"].(string); ok {
		return sanitizeID(raw)
	}
	return ""
}

func sanitizeID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer(".", "_", "#", "_", "$", "_", "[", "_", "]", "_", "/", "_")
	return replacer.Replace(value)
}

func sha256Hex(value string) string {
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}


