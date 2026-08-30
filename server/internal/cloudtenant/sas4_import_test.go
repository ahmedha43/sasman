package cloudtenant

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// decryptSaltedSAS4 decrypts OpenSSL Salted AES-256-CBC payloads from mock requests
func decryptSaltedSAS4(b64Text string, passphrase string) (string, error) {
	b64Text = strings.TrimSpace(b64Text)
	data, err := base64.StdEncoding.DecodeString(b64Text)
	if err != nil {
		data, err = base64.URLEncoding.DecodeString(b64Text)
		if err != nil {
			return "", err
		}
	}
	if len(data) < 16 || string(data[:8]) != "Salted__" {
		if len(data) >= 8 {
			return "", fmt.Errorf("invalid Salted__ header (header got: %q)", data[:8])
		}
		return "", fmt.Errorf("invalid Salted__ header")
	}

	salt := data[8:16]
	ciphertext := data[16:]

	pw := []byte(passphrase)
	var hashes []byte
	var last []byte
	for len(hashes) < 48 {
		h := md5.New()
		h.Write(last)
		h.Write(pw)
		h.Write(salt)
		last = h.Sum(nil)
		hashes = append(hashes, last...)
	}

	key := hashes[:32]
	iv := hashes[32:48]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plain := make([]byte, len(ciphertext))
	mode.CryptBlocks(plain, ciphertext)

	// Remove PKCS7 padding
	if len(plain) > 0 {
		pad := int(plain[len(plain)-1])
		if pad > 0 && pad <= aes.BlockSize && pad <= len(plain) {
			plain = plain[:len(plain)-pad]
		}
	}
	return string(plain), nil
}

func TestSAS4MigrationEngine(t *testing.T) {
	// 1. Create a mock SAS4 HTTP Server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check required browser headers
		ua := r.Header.Get("User-Agent")
		if !strings.Contains(ua, "Mozilla/5.0") {
			t.Logf("⚠️ Mock Server: Missing Browser User-Agent header (Got: %s)", ua)
			http.Error(w, "Forbidden - Invalid User Agent", http.StatusForbidden)
			return
		}

		var decryptedJSON string
		if r.Method == "POST" {
			bodyBytes, _ := io.ReadAll(r.Body)
			var reqObj map[string]string
			_ = json.Unmarshal(bodyBytes, &reqObj)
			encryptedPayload := reqObj["payload"]

			var err error
			decryptedJSON, err = decryptSaltedSAS4(encryptedPayload, SAS_PASSPHRASE)
			if err != nil {
				t.Logf("⚠️ Mock Server: Payload decryption failed for payload %q (len=%d): %v", encryptedPayload, len(encryptedPayload), err)
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
		}

		// Endpoint 1: Login
		if strings.HasSuffix(r.URL.Path, "/api/login") {
			var loginFields map[string]string
			_ = json.Unmarshal([]byte(decryptedJSON), &loginFields)
			if loginFields["username"] == "demo_admin" && loginFields["password"] == "demo_pass123" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status":  true,
					"message": "Login Successful",
					"token":   "mock_sas4_token_998877665544332211",
				})
				return
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Endpoint 2: User List
		if strings.HasSuffix(r.URL.Path, "/api/index/user") {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "Bearer mock_sas4_token_998877665544332211" {
				http.Error(w, "Unauthorized token", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": true,
				"data": []map[string]interface{}{
					{
						"id":           "101",
						"username":     "user_test_01",
						"ct_password":  "pass001",
						"profile_name": "Standard_10M",
						"balance":      1500.0,
						"expiration":   "2026-12-31 23:59:59",
						"firstname":    "Ahmed",
						"lastname":     "Ali",
						"phone":        "07701234567",
					},
					{
						"id":           "102",
						"username":     "user_test_02",
						"ct_password":  "pass002",
						"profile_name": "Premium_50M",
						"balance":      3000.0,
						"expiration":   "2026-12-31 23:59:59",
						"firstname":    "Omar",
						"lastname":     "Hassan",
						"phone":        "07801234567",
					},
				},
			})
			return
		}

		// Endpoint 3: User Detail Overview
		if strings.Contains(r.URL.Path, "/api/user/overview/") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": true,
				"data": map[string]interface{}{
					"enabled": "1",
				},
			})
			return
		}

		http.NotFound(w, r)
	}))
	defer mockServer.Close()

	t.Logf("✅ Step 1: Mock SAS4 Server running at %s", mockServer.URL)

	// 2. Prepare Temp Tenant DB
	tempDir, err := os.MkdirTemp("", "sas4_import_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "tenant_test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open temp db: %v", err)
	}
	defer db.Close()

	// Initialize tables in mock tenant DB
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS radcheck (id INTEGER PRIMARY KEY, username TEXT, attribute TEXT, op TEXT, value TEXT)`)
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS radusergroup (id INTEGER PRIMARY KEY, username TEXT, groupname TEXT, priority INTEGER)`)
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS radgroupcheck (id INTEGER PRIMARY KEY, groupname TEXT, attribute TEXT, op TEXT, value TEXT)`)
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS radius_profile_meta (groupname TEXT PRIMARY KEY, validity_days INTEGER, price REAL, updated_at DATETIME)`)
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS radius_user_meta (username TEXT PRIMARY KEY, balance REAL, expiration_unix INTEGER, full_name TEXT, phone TEXT, enabled INTEGER, updated_at DATETIME)`)

	// 3. Execute Migration against Mock Server
	req := SAS4MigrationRequest{
		URL:      mockServer.URL + "/admin/api/index.php/api/login",
		Username: "demo_admin",
		Password: "demo_pass123",
	}

	result, err := RunSAS4Migration(db, req)
	if err != nil {
		t.Fatalf("❌ SAS4 Migration failed: %v", err)
	}

	t.Logf("Migration Result: %s (Users Imported: %d, Profiles Seen: %d)", result.Message, result.UsersImported, result.ProfilesSeen)

	// 4. Verify DB contents
	var userCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM radius_user_meta").Scan(&userCount)
	if userCount != 2 {
		t.Fatalf("❌ Expected 2 imported users in DB, got: %d", userCount)
	}

	var pass1 string
	_ = db.QueryRow("SELECT value FROM radcheck WHERE username='user_test_01' AND attribute='Cleartext-Password'").Scan(&pass1)
	if pass1 != "pass001" {
		t.Fatalf("❌ User password mismatch: expected pass001, got %s", pass1)
	}

	t.Log("SAS4 MIGRATION TEST PASSED WITH 100% SUCCESS!")
}
