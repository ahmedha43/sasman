package radius

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

const SAS_PASSPHRASE = "abcdefghijuklmno0123456789012345"

// encryptSaltedSAS4 implements the OpenSSL-style "Salted__" encryption (AES-256-CBC)
func encryptSaltedSAS4(plainText string, passphrase string) (string, error) {
	salt := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", err
	}

	// Key/IV Derivation (EVP_BytesToKey equivalent)
	// We need 48 bytes (32 for key, 16 for IV)
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

	// PKCS7 Padding
	plainBytes := []byte(plainText)
	blockSize := aes.BlockSize
	padding := blockSize - (len(plainBytes) % blockSize)
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	plainBytes = append(plainBytes, padText...)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	encrypted := make([]byte, len(plainBytes))
	mode.CryptBlocks(encrypted, plainBytes)

	// Combine: "Salted__" + salt + encrypted
	final := append([]byte("Salted__"), salt...)
	final = append(final, encrypted...)

	return base64.StdEncoding.EncodeToString(final), nil
}

type SAS4MigrationRequest struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func ImportFromSAS4(c *fiber.Ctx) error {
	var req SAS4MigrationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.URL == "" || req.Username == "" || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "URL, Username, and Password are required"})
	}

	rawURL := strings.TrimSuffix(req.URL, "/")
	cleanBase := rawURL
	cleanBase = strings.TrimSuffix(cleanBase, "/admin")
	cleanBase = strings.TrimSuffix(cleanBase, "/api")
	cleanBase = strings.TrimSuffix(cleanBase, "/index.php")
	cleanBase = strings.TrimSuffix(cleanBase, "/")

	type sas4Endpoint struct {
		loginURL    string
		userListURL string
		overviewURL string
	}

	endpoints := []sas4Endpoint{
		{
			loginURL:    cleanBase + "/admin/api/index.php/api/login",
			userListURL: cleanBase + "/admin/api/index.php/api/index/user",
			overviewURL: cleanBase + "/admin/api/index.php/api/user/overview/",
		},
		{
			loginURL:    cleanBase + "/api/index.php/api/login",
			userListURL: cleanBase + "/api/index.php/api/index/user",
			overviewURL: cleanBase + "/api/index.php/api/user/overview/",
		},
		{
			loginURL:    rawURL + "/api/login",
			userListURL: rawURL + "/api/index/user",
			overviewURL: rawURL + "/api/user/overview/",
		},
	}

	loginPayloadObj := map[string]string{
		"username": req.Username,
		"password": req.Password,
		"language": "en",
	}
	loginJSON, _ := json.Marshal(loginPayloadObj)
	encryptedPayload, err := encryptSaltedSAS4(string(loginJSON), SAS_PASSPHRASE)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Encryption error: " + err.Error()})
	}

	var resp map[string]interface{}
	var lastErr error
	var activeEp sas4Endpoint

	for _, ep := range endpoints {
		resp, err = postSAS4(ep.loginURL, encryptedPayload, "")
		if err == nil {
			activeEp = ep
			lastErr = nil
			break
		}
		lastErr = err
	}

	if lastErr != nil || resp == nil {
		return c.Status(500).JSON(fiber.Map{"error": "SAS4 Login failed: " + lastErr.Error()})
	}

	token, ok := resp["token"].(string)
	if !ok {
		// Fallback for nested data structure
		if data, ok := resp["data"].(map[string]interface{}); ok {
			token, _ = data["token"].(string)
		}
	}

	if token == "" {
		fmt.Printf("[sas4-import] Auth FAILED: Missing token in response\n")
		return c.Status(401).JSON(fiber.Map{"error": "Failed to obtain authentication token from SAS4"})
	}

	fmt.Printf("[sas4-import] Login SUCCESS. Fetching user list...\n")

	userListURL := activeEp.userListURL
	overviewBaseURL := activeEp.overviewURL

	// 2. Fetch All Users Across Pages
	cols := []string{"id", "username", "firstname", "lastname", "phone", "balance", "expiration", "static_ip", "enabled", "profile_name", "ct_password", "created_at"}
	var usersData []interface{}
	pageSize := 200

	for page := 1; page <= 50; page++ {
		fetchPayloadObj := map[string]interface{}{
			"page":    page,
			"count":   pageSize,
			"columns": cols,
		}
		fetchJSON, _ := json.Marshal(fetchPayloadObj)
		encryptedFetch, _ := encryptSaltedSAS4(string(fetchJSON), SAS_PASSPHRASE)

		userResp, err := postSAS4(userListURL, encryptedFetch, token)
		if err != nil {
			if page == 1 {
				fmt.Printf("[sas4-import] Fetch users FAILED: %v\n", err)
				return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch users: " + err.Error()})
			}
			break
		}

		pageUsers, ok := userResp["data"].([]interface{})
		if !ok || len(pageUsers) == 0 {
			break
		}

		usersData = append(usersData, pageUsers...)

		if len(pageUsers) < pageSize {
			break // Final page reached
		}
	}

	if len(usersData) == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "No subscribers found in SAS4 account"})
	}

	fmt.Printf("[sas4-import] Fetched total %d users across pages. Starting deep sync (fetching passwords)...\n", len(usersData))

	// 3. Process and Save (with concurrent detail fetching)
	importCount := 0
	profileCount := 0
	profilesSeen := make(map[string]bool)

	// Use a worker pool to fetch details concurrently
	type job struct {
		u map[string]interface{}
	}
	type result struct {
		u   map[string]interface{}
		err error
	}

	jobs := make(chan job, len(usersData))
	results := make(chan result, len(usersData))

	for w := 1; w <= 10; w++ { // 10 workers
		go func(workerID int) {
			for j := range jobs {
				id := getString(j.u, "id")
				if id == "" {
					id = fmt.Sprintf("%v", j.u["id"])
					id = strings.TrimSuffix(id, ".0")
				}
				detailURL := overviewBaseURL + id
				detail, err := getSAS4(detailURL, token)
				if err == nil {
					if data, ok := detail["data"].(map[string]interface{}); ok {
						// Merge detail into user object
						for k, v := range data {
							j.u[k] = v
						}
					}
				}
				results <- result{u: j.u, err: err}
			}
		}(w)
	}

	for _, uObj := range usersData {
		u, ok := uObj.(map[string]interface{})
		if ok {
			jobs <- job{u: u}
		} else {
			results <- result{err: fmt.Errorf("invalid object")}
		}
	}
	close(jobs)

	for i := 0; i < len(usersData); i++ {
		res := <-results
		if res.err != nil {
			continue
		}

		u := res.u
		username := getString(u, "username")
		password := getString(u, "password")
		if password == "" {
			password = getString(u, "ct_password")
		}
		profileName := getSAS4ProfileName(u)
		expiration := getString(u, "expiration")
		balance := getFloat(u, "balance")

		// DEBUG: Log profile name to see what we're getting
		if profileName != "" {
			fmt.Printf("[sas4-import] User %s has profile_name: '%s'\n", username, profileName)
		} else {
			fmt.Printf("[sas4-import] User %s has EMPTY profile_name\n", username)
			// Let's see what fields we do have
			fmt.Printf("[sas4-import] User %s fields: %v\n", username, u)
		}

		// Metadata
		firstName := getString(u, "firstname")
		lastName := getString(u, "lastname")
		fullName := strings.TrimSpace(firstName + " " + lastName)
		phone := getString(u, "phone")

		if username == "" || password == "" {
			continue
		}

		// Ensure profile exists
		if profileName != "" && !profilesSeen[profileName] {
			fmt.Printf("[sas4-import] Creating profile: '%s'\n", profileName)
			err := ensureProfileExists(profileName)
			if err == nil {
				profilesSeen[profileName] = true
				profileCount++
				fmt.Printf("[sas4-import] Profile '%s' created successfully\n", profileName)
			} else {
				fmt.Printf("[sas4-import] Failed to create profile '%s': %v\n", profileName, err)
			}
		} else if profileName == "" {
			fmt.Printf("[sas4-import] Skipping profile creation for user %s - empty profile name\n", username)
		}

		// Save User
		err := importUser(username, password, profileName, expiration, balance, fullName, phone)
		if err == nil {
			importCount++
		} else {
			fmt.Printf("[sas4-import] Sync FAILED for %s: %v\n", username, err)
		}
	}

	return c.JSON(fiber.Map{
		"message":        fmt.Sprintf("Migration completed: %d users and %d profiles processed.", importCount, profileCount),
		"users_imported": importCount,
		"profiles_seen":  profileCount,
	})
}

func postSAS4(url string, payload string, token string) (map[string]interface{}, error) {
	body := map[string]string{"payload": payload}
	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,ar;q=0.8")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := sas4HTTPClient(30 * time.Second).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		errMsg := strings.TrimSpace(string(respBody))
		if len(errMsg) > 200 {
			errMsg = errMsg[:200]
		}
		if errMsg != "" {
			return nil, fmt.Errorf("SAS4 POST returned status %d (%s)", resp.StatusCode, errMsg)
		}
		return nil, fmt.Errorf("SAS4 POST returned status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

func getSAS4(url string, token string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,ar;q=0.8")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := sas4HTTPClient(10 * time.Second).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		errMsg := strings.TrimSpace(string(respBody))
		if len(errMsg) > 200 {
			errMsg = errMsg[:200]
		}
		if errMsg != "" {
			return nil, fmt.Errorf("SAS4 GET returned status %d (%s)", resp.StatusCode, errMsg)
		}
		return nil, fmt.Errorf("SAS4 GET returned status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// sas4TLSInsecure reports whether to skip TLS certificate verification when
// talking to the SAS4 server. SAS4 typically uses a self-signed certificate,
// so this defaults to true. Set SAS4_INSECURE_TLS=0 to enforce verification.
func sas4TLSInsecure() bool {
	insecure := os.Getenv("SAS4_INSECURE_TLS") != "0"
	if insecure {
		sas4InsecureLogged.Do(func() {
			fmt.Printf("[sas4] WARNING: TLS certificate verification is DISABLED for SAS4 connections (self-signed certificate). Set SAS4_INSECURE_TLS=0 to enforce verification.\n")
		})
	}
	return insecure
}

// sas4HTTPTransport is shared across all SAS4 requests so TLS connections and
// keep-alives are reused — the deep sync performs one request per subscriber.
var sas4HTTPTransport = &http.Transport{
	TLSClientConfig: &tls.Config{InsecureSkipVerify: sas4TLSInsecure()},
}

var sas4InsecureLogged sync.Once

// sas4HTTPClient returns a client for SAS4 requests with the given timeout,
// reusing the shared transport (and its connection pool).
func sas4HTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: sas4HTTPTransport}
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
		if f, ok := v.(float64); ok {
			if f == float64(int64(f)) {
				return strconv.FormatInt(int64(f), 10)
			}
			return strconv.FormatFloat(f, 'f', -1, 64)
		}
		if b, ok := v.(bool); ok {
			return strconv.FormatBool(b)
		}
	}
	return ""
}

func getSAS4ProfileName(u map[string]interface{}) string {
	keys := []string{
		"profile_name",
		"profile",
		"profileName",
		"profile_title",
		"profileTitle",
		"user_profile",
		"userProfile",
		"user_group",
		"userGroup",
		"groupname",
		"group_name",
		"group",
		"package",
		"package_name",
		"service",
		"service_name",
		"subscription",
		"subscription_name",
	}

	for _, key := range keys {
		if name := getString(u, key); name != "" {
			return name
		}
		if name := getNestedProfileName(u, key); name != "" {
			return name
		}
	}

	for key, value := range u {
		keyLower := strings.ToLower(key)
		if !strings.Contains(keyLower, "profile") &&
			!strings.Contains(keyLower, "group") &&
			!strings.Contains(keyLower, "package") &&
			!strings.Contains(keyLower, "service") &&
			!strings.Contains(keyLower, "subscription") {
			continue
		}
		obj, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		if name := firstStringFromMap(obj, "name", "profile_name", "title", "label", "groupname"); name != "" {
			return name
		}
	}

	return ""
}

func getNestedProfileName(u map[string]interface{}, key string) string {
	value, ok := u[key]
	if !ok {
		return ""
	}
	obj, ok := value.(map[string]interface{})
	if !ok {
		return ""
	}
	return firstStringFromMap(obj, "name", "profile_name", "title", "label", "groupname")
}

func firstStringFromMap(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value := getString(m, key); value != "" {
			return value
		}
	}
	return ""
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
		if s, ok := v.(string); ok {
			var f float64
			fmt.Sscanf(s, "%f", &f)
			return f
		}
	}
	return 0
}

func ensureProfileExists(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}

	// DEBUG: Log when we're trying to create a profile
	fmt.Printf("[sas4-import] ensureProfileExists called with name: '%s'\n", name)

	// 1. Create a default group entry for FreeRADIUS logic (if not exists)
	_, _ = DB.Exec(`
		INSERT INTO radgroupcheck (groupname, attribute, op, value)
		SELECT ?, 'Simultaneous-Use', ':=', '1'
		WHERE NOT EXISTS (
			SELECT 1 FROM radgroupcheck
			WHERE groupname=? AND attribute='Simultaneous-Use'
		)`, name, name)

	// 2. Create metadata entry for SASMAN UI visibility (if not exists)
	_, err := DB.Exec(`
		INSERT INTO radius_profile_meta (groupname, validity_days, price, updated_at)
		VALUES (?, 30, 0, CURRENT_TIMESTAMP)
		ON CONFLICT(groupname) DO NOTHING`, name)

	if err != nil {
		fmt.Printf("[sas4-import] Error creating profile '%s': %v\n", name, err)
	} else {
		fmt.Printf("[sas4-import] Profile '%s' ensured in database\n", name)
	}

	return err
}

func ResetDatabase(c *fiber.Ctx) error {
	tables := []string{
		"radcheck", "radreply", "radusergroup", "radgroupcheck", "radgroupreply",
		"radacct", "radpostauth", "nas", "radius_user_meta", "radius_profile_meta",
		"radius_user_transactions", "radius_vouchers",
	}

	for _, table := range tables {
		_, err := DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s", table))
		if err != nil {
			// Fallback to DELETE if TRUNCATE fails (due to foreign keys, though standard radius doesn't have many)
			_, _ = DB.Exec(fmt.Sprintf("DELETE FROM %s", table))
			fmt.Printf("[system] Failed to truncate %s, used DELETE fallback: %v\n", table, err)
		}
	}

	LogActivityFromCtx(c, "تصفير النظام", "قاعدة البيانات", "تم تصفير وإعادة ضبط مصنع كافة جداول قاعدة البيانات بالكامل")
	return c.JSON(fiber.Map{"message": "System database has been reset successfully."})
}

func importUser(username, password, profile, expiration string, balance float64, fullName, phone string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. radcheck (Password)
	tx.Exec("DELETE FROM radcheck WHERE username = ? AND attribute = 'Cleartext-Password'", username)
	_, err = tx.Exec("INSERT INTO radcheck (username, attribute, op, value) VALUES (?, ?, ?, ?)",
		username, "Cleartext-Password", ":=", password)
	if err != nil {
		return err
	}

	// 2. radusergroup
	if profile != "" {
		tx.Exec("DELETE FROM radusergroup WHERE username = ?", username)
		tx.Exec("INSERT INTO radusergroup (username, groupname, priority) VALUES (?, ?, ?)",
			username, profile, 1)
	}

	// 3. Radius User Meta & Expiration Attribute
	var expUnix interface{}
	if expiration != "" {
		// SAS4 usually uses YYYY-MM-DD HH:MM:SS
		t, err := time.Parse("2006-01-02 15:04:05", expiration)
		if err == nil {
			eu := t.Unix()
			expUnix = eu

			// Insert Expiration explicitly into radcheck for FreeRADIUS
			tx.Exec("DELETE FROM radcheck WHERE username = ? AND attribute = 'Expiration'", username)
			tx.Exec("INSERT INTO radcheck (username, attribute, op, value) VALUES (?, 'Expiration', ':=', ?)",
				username, strconv.FormatInt(eu, 10))
		}
	}

	tx.Exec("DELETE FROM radius_user_meta WHERE username = ?", username)
	_, err = tx.Exec(`INSERT INTO radius_user_meta 
		(username, balance, expiration_unix, full_name, phone, enabled, updated_at) 
		VALUES (?, ?, ?, ?, ?, 1, CURRENT_TIMESTAMP)`,
		username, balance, expUnix, fullName, phone)
	if err != nil {
		return err
	}

	return tx.Commit()
}
