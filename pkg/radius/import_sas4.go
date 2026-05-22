package radius

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
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

	baseURL := strings.TrimSuffix(req.URL, "/") + "/"
	loginURL := baseURL + "admin/api/index.php/api/login"
	userListURL := baseURL + "admin/api/index.php/api/index/user"

	// 1. Authenticate
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

	resp, err := postSAS4(loginURL, encryptedPayload, "")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "SAS4 Login failed: " + err.Error()})
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

	// 2. Fetch Users
	// EXCEL_HEADERS = ['id', 'username', 'firstname', 'lastname', 'phone', 'balance', 'expiration', 'static_ip', 'enabled', 'profile_name', 'ct_password', 'created_at']
	cols := []string{"id", "username", "firstname", "lastname", "phone", "balance", "expiration", "static_ip", "enabled", "profile_name", "ct_password", "created_at"}
	fetchPayloadObj := map[string]interface{}{
		"page":    1,
		"count":   5000, // Large count to fetch most users
		"columns": cols,
	}
	fetchJSON, _ := json.Marshal(fetchPayloadObj)
	encryptedFetch, _ := encryptSaltedSAS4(string(fetchJSON), SAS_PASSPHRASE)

	userResp, err := postSAS4(userListURL, encryptedFetch, token)
	if err != nil {
		fmt.Printf("[sas4-import] Fetch users FAILED: %v\n", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch users: " + err.Error()})
	}

	usersData, ok := userResp["data"].([]interface{})
	if !ok {
		fmt.Printf("[sas4-import] Unexpected format: 'data' is not an array\n")
		return c.Status(500).JSON(fiber.Map{"error": "Unexpected users response format"})
	}

	fmt.Printf("[sas4-import] Fetched %d potential users. Starting deep sync (fetching passwords)...\n", len(usersData))

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
				detailURL := baseURL + "admin/api/index.php/api/user/overview/" + id
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
		profileName := getString(u, "profile_name")
		expiration := getString(u, "expiration")
		balance := getFloat(u, "balance")

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
			err := ensureProfileExists(profileName)
			if err == nil {
				profilesSeen[profileName] = true
				profileCount++
			}
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
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
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

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("SAS4 GET returned status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
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
	// 1. Create a default group entry for FreeRADIUS logic (if not exists)
	_, _ = DB.Exec("INSERT IGNORE INTO radgroupcheck (groupname, attribute, op, value) VALUES (?, 'Simultaneous-Use', ':=', '1')", name)

	// 2. Create metadata entry for SASMAN UI visibility (if not exists)
	_, err := DB.Exec("INSERT IGNORE INTO radius_profile_meta (groupname, validity_days, price) VALUES (?, 30, 0)", name)

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
