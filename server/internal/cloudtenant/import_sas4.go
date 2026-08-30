package cloudtenant

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/tls"
	"database/sql"
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
)

const SAS_PASSPHRASE = "abcdefghijuklmno0123456789012345"

// encryptSaltedSAS4 implements the OpenSSL-style "Salted__" encryption (AES-256-CBC)
func encryptSaltedSAS4(plainText string, passphrase string) (string, error) {
	salt := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", err
	}

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

	final := append([]byte("Salted__"), salt...)
	final = append(final, encrypted...)

	return base64.StdEncoding.EncodeToString(final), nil
}

type SAS4MigrationRequest struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type SAS4MigrationResult struct {
	Message       string `json:"message"`
	UsersImported int    `json:"users_imported"`
	ProfilesSeen  int    `json:"profiles_seen"`
}

func RunSAS4Migration(db *sql.DB, req SAS4MigrationRequest) (*SAS4MigrationResult, error) {
	if req.URL == "" || req.Username == "" || req.Password == "" {
		return nil, fmt.Errorf("الرابط واسم المستخدم وكلمة المرور مطلوبة")
	}

	rawURL := strings.TrimSuffix(req.URL, "/")
	cleanBase := rawURL
	for _, s := range []string{"/admin/api/index.php/api/login", "/api/login", "/api/index/user", "/admin/api", "/api", "/index.php", "/admin"} {
		cleanBase = strings.TrimSuffix(cleanBase, s)
	}
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
		return nil, fmt.Errorf("خطأ في تشفير البيانات: %v", err)
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
		return nil, fmt.Errorf("فشل تسجيل الدخول إلى SAS4: %v", lastErr)
	}

	token, ok := resp["token"].(string)
	if !ok {
		if data, ok := resp["data"].(map[string]interface{}); ok {
			token, _ = data["token"].(string)
		}
	}

	if token == "" {
		return nil, fmt.Errorf("فشل الحصول على رمز الدخول من SAS4 (تحقق من صحة الحساب)")
	}

	userListURL := activeEp.userListURL
	overviewBaseURL := activeEp.overviewURL

	// 2. Fetch Users
	cols := []string{"id", "username", "firstname", "lastname", "phone", "balance", "expiration", "static_ip", "enabled", "profile_name", "ct_password", "created_at"}
	fetchPayloadObj := map[string]interface{}{
		"page":    1,
		"count":   10000,
		"columns": cols,
	}
	fetchJSON, _ := json.Marshal(fetchPayloadObj)
	encryptedFetch, _ := encryptSaltedSAS4(string(fetchJSON), SAS_PASSPHRASE)

	userResp, err := postSAS4(userListURL, encryptedFetch, token)
	if err != nil {
		return nil, fmt.Errorf("فشل جلب قائمة المشتركين من SAS4: %v", err)
	}

	usersData, ok := userResp["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("صيغة استجابة غير صالحة من SAS4")
	}

	importCount := 0
	profileCount := 0
	profilesSeen := make(map[string]bool)

	type job struct {
		u map[string]interface{}
	}
	type result struct {
		u   map[string]interface{}
		err error
	}

	jobs := make(chan job, len(usersData))
	results := make(chan result, len(usersData))

	for w := 1; w <= 10; w++ {
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

		firstName := getString(u, "firstname")
		lastName := getString(u, "lastname")
		fullName := strings.TrimSpace(firstName + " " + lastName)
		phone := getString(u, "phone")

		if username == "" || password == "" {
			continue
		}

		if profileName != "" && !profilesSeen[profileName] {
			err := ensureProfileExistsCloud(db, profileName)
			if err == nil {
				profilesSeen[profileName] = true
				profileCount++
			}
		}

		err := importUserCloud(db, username, password, profileName, expiration, balance, fullName, phone)
		if err == nil {
			importCount++
		}
	}

	return &SAS4MigrationResult{
		Message:       fmt.Sprintf("تم استيراد %d مشترك و %d باقة بنجاح من SAS4", importCount, profileCount),
		UsersImported: importCount,
		ProfilesSeen:  profileCount,
	}, nil
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
			return nil, fmt.Errorf("رمز الاستجابة %d (%s)", resp.StatusCode, errMsg)
		}
		return nil, fmt.Errorf("رمز الاستجابة %d", resp.StatusCode)
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
			return nil, fmt.Errorf("رمز الاستجابة %d (%s)", resp.StatusCode, errMsg)
		}
		return nil, fmt.Errorf("رمز الاستجابة %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

func sas4TLSInsecure() bool {
	return os.Getenv("SAS4_INSECURE_TLS") != "0"
}

var sas4HTTPTransport = &http.Transport{
	TLSClientConfig: &tls.Config{InsecureSkipVerify: sas4TLSInsecure()},
}

var sas4InsecureLogged sync.Once

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
		"profile_name", "profile", "profileName", "profile_title", "profileTitle",
		"user_profile", "userProfile", "user_group", "userGroup", "groupname",
		"group_name", "group", "package", "package_name", "service", "service_name",
		"subscription", "subscription_name",
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

func ensureProfileExistsCloud(db *sql.DB, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}

	_, _ = db.Exec(`
		INSERT INTO radgroupcheck (groupname, attribute, op, value)
		SELECT ?, 'Simultaneous-Use', ':=', '1'
		WHERE NOT EXISTS (
			SELECT 1 FROM radgroupcheck
			WHERE groupname=? AND attribute='Simultaneous-Use'
		)`, name, name)

	_, err := db.Exec(`
		INSERT INTO radius_profile_meta (groupname, validity_days, price, updated_at)
		VALUES (?, 30, 0, CURRENT_TIMESTAMP)
		ON CONFLICT(groupname) DO NOTHING`, name)

	return err
}

func importUserCloud(db *sql.DB, username, password, profile, expiration string, balance float64, fullName, phone string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. radcheck (Password)
	tx.Exec("DELETE FROM radcheck WHERE username = ? AND attribute = 'Cleartext-Password'", username)
	_, err = tx.Exec("INSERT INTO radcheck (username, attribute, op, value) VALUES (?, 'Cleartext-Password', ':=', ?)", username, password)
	if err != nil {
		return err
	}

	// 2. radusergroup
	if profile != "" {
		tx.Exec("DELETE FROM radusergroup WHERE username = ?", username)
		tx.Exec("INSERT INTO radusergroup (username, groupname, priority) VALUES (?, ?, 1)", username, profile)
	}

	// 3. Radius User Meta & Expiration
	var expUnix interface{}
	if expiration != "" {
		t, err := time.Parse("2006-01-02 15:04:05", expiration)
		if err == nil {
			eu := t.Unix()
			expUnix = eu

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
