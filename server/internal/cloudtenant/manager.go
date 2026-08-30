package cloudtenant

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"mikrotik-manager/pkg/pki"
	"mikrotik-manager/server/internal/storage"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var validSubdomainRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{1,28}[a-z0-9])?$`)

var reservedSubdomains = map[string]bool{
	"admin":    true,
	"api":      true,
	"cloud":    true,
	"www":      true,
	"central":  true,
	"sasman":   true,
	"radius":   true,
	"portal":   true,
	"static":   true,
	"pki":      true,
	"system":   true,
	"hotspot":  true,
	"support":  true,
	"root":     true,
	"internal": true,
}

type RadSecDisconnector interface {
	DisconnectUser(subdomainOrNAS string, username string, sessionID string, framedIP string) error
}

type Manager struct {
	repo         *storage.SQLiteRepository
	pool         *TenantDBPool
	domain       string
	jwtSecret    []byte
	disconnector RadSecDisconnector
}

func NewManager(repo *storage.SQLiteRepository, pool *TenantDBPool, domain string, jwtSecret []byte) *Manager {
	if len(jwtSecret) == 0 {
		jwtSecret = []byte("SASMAN_CLOUD_SECRET_KEY_9977_SECURE")
	}
	if domain == "" {
		domain = "sas-man.net"
	}
	mgr := &Manager{
		repo:      repo,
		pool:      pool,
		domain:    domain,
		jwtSecret: jwtSecret,
	}
	mgr.StartAllWinboxForwarders()
	return mgr
}

func (m *Manager) StartAllWinboxForwarders() {
	if m.repo == nil {
		return
	}
	subNames, err := m.repo.ListAllSubdomains()
	if err != nil {
		return
	}

	usedPorts := make(map[int]bool)
	var needPort []*storage.Subdomain

	for _, name := range subNames {
		subObj, err := m.repo.GetSubdomainByName(name)
		if err == nil && subObj != nil {
			if subObj.WinboxPort >= 10001 {
				usedPorts[subObj.WinboxPort] = true
			} else {
				needPort = append(needPort, subObj)
			}
		}
	}

	// Allocate free ports for those with port <= 0
	nextPort := 10001
	for _, subObj := range needPort {
		for usedPorts[nextPort] {
			nextPort++
		}
		subObj.WinboxPort = nextPort
		usedPorts[nextPort] = true
		_ = m.repo.UpdateSubdomainStatus(subObj.Subdomain, subObj.Token, subObj.WinboxPort)
		log.Printf("[Winbox-Cloud] 🏷️ Allocated dedicated Winbox Port %d for [%s]", subObj.WinboxPort, subObj.Subdomain)
	}

	// Start all listeners
	for _, name := range subNames {
		subObj, err := m.repo.GetSubdomainByName(name)
		if err == nil && subObj != nil && subObj.WinboxPort > 0 {
			_ = GetGlobalWinboxProxyMgr().StartForwarder(subObj.Subdomain, subObj.WinboxPort)
		}
	}
}

func (m *Manager) SetDisconnector(d RadSecDisconnector) {
	m.disconnector = d
}

func (m *Manager) DisconnectCloudUser(subdomain string, username string, sessionID string, framedIP string) error {
	if m.disconnector == nil {
		return fmt.Errorf("disconnector not initialized")
	}
	return m.disconnector.DisconnectUser(subdomain, username, sessionID, framedIP)
}

func (m *Manager) GetPool() *TenantDBPool {
	return m.pool
}

func (m *Manager) IsSubdomainAvailable(subdomain string) (bool, string) {
	sub := strings.ToLower(strings.TrimSpace(subdomain))
	if len(sub) < 3 {
		return false, "اسم النطاق يجب أن يكون 3 أحرف على الأقل"
	}
	if len(sub) > 30 {
		return false, "اسم النطاق يجب ألا يتجاوز 30 حرفاً"
	}
	if !validSubdomainRegex.MatchString(sub) {
		return false, "اسم النطاق يجب أن يحتوي على أحرف إنجليزية وأرقام وشرطة فقط"
	}
	if reservedSubdomains[sub] {
		return false, "هذا النطاق محجوز للنظام"
	}

	// Check central database
	if m.repo != nil {
		available, err := m.repo.IsSubdomainAvailable(sub)
		if err != nil {
			return false, "خطأ في التحقق من النطاق: " + err.Error()
		}
		if !available {
			return false, "هذا النطاق مستخدم بالفعل"
		}
	}

	// Check if directory exists
	tenantDir := m.pool.GetTenantDir(sub)
	if _, err := os.Stat(tenantDir); err == nil {
		return false, "هذا النطاق مستخدم بالفعل"
	}

	return true, ""
}

func (m *Manager) RegisterTenant(req RegisterRequest) (*CloudTenant, error) {
	sub := strings.ToLower(strings.TrimSpace(req.Subdomain))
	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := strings.TrimSpace(req.Password)

	if ok, reason := m.IsSubdomainAvailable(sub); !ok {
		return nil, fmt.Errorf("%s", reason)
	}

	if email == "" || !strings.Contains(email, "@") {
		return nil, fmt.Errorf("البريد الإلكتروني غير صالح")
	}

	if len(password) < 6 {
		return nil, fmt.Errorf("كلمة المرور يجب أن تكون 6 خانات على الأقل")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("فشل تشفير كلمة المرور: %w", err)
	}

	// 1. Create Tenant Directory
	tenantDir := m.pool.GetTenantDir(sub)
	_ = os.MkdirAll(tenantDir, 0755)

	// 2. Generate mTLS Certificates for MikroTik RadSec Client
	_ = pki.InitPKI()
	commonName := fmt.Sprintf("agent-%s-SASMAN", sub)
	certBundle, err := pki.GenerateClientCertificate(commonName, 365*5) // 5 years validity
	if err != nil {
		return nil, fmt.Errorf("فشل توليد شهادات التشفير: %w", err)
	}

	// Save to PKI repository and tenant certs directory
	pkiAgentDir := filepath.Join("data", "pki", "agents", sub)
	if _, e := os.Stat("/app/data"); e == nil {
		pkiAgentDir = filepath.Join("/app/data", "pki", "agents", sub)
	}
	_ = os.MkdirAll(pkiAgentDir, 0755)
	_ = os.WriteFile(filepath.Join(pkiAgentDir, "agent.crt"), []byte(certBundle.CertPEM), 0644)
	_ = os.WriteFile(filepath.Join(pkiAgentDir, "agent.key"), []byte(certBundle.KeyPEM), 0600)

	tenantCertsDir := filepath.Join(tenantDir, "certs")
	_ = os.MkdirAll(tenantCertsDir, 0755)
	_ = os.WriteFile(filepath.Join(tenantCertsDir, "agent.crt"), []byte(certBundle.CertPEM), 0644)
	_ = os.WriteFile(filepath.Join(tenantCertsDir, "agent.key"), []byte(certBundle.KeyPEM), 0600)

	// 3. Initialize Tenant Isolated SQLite Database
	tenantDB, err := m.pool.Get(sub)
	if err != nil {
		return nil, fmt.Errorf("فشل تهيئة قاعدة بيانات المستأجر: %w", err)
	}

	// Insert or update initial administrator
	adminPassHash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	_, _ = tenantDB.Exec(`
		INSERT OR REPLACE INTO radius_admins (username, password, role, name, phone, is_active)
		VALUES (?, ?, 'superadmin', ?, ?, 1)
	`, "admin", string(adminPassHash), req.OwnerName, req.Phone)

	// 4. Save Customer and Subdomain in Central Storage
	customerID := fmt.Sprintf("cust_%s", sub)
	licenseID := fmt.Sprintf("lic_%s", sub)
	now := time.Now()

	if m.repo != nil {
		_ = m.repo.SaveCustomer(storage.Customer{
			ID:          customerID,
			Name:        req.OwnerName,
			Phone:       req.Phone,
			Email:       email,
			CompanyName: sub,
			Status:      "active",
			CreatedAt:   now,
			UpdatedAt:   now,
		})

		_ = m.repo.SaveLicense(storage.License{
			ID:         licenseID,
			CustomerID: customerID,
			LicenseKey: generateRandomToken(24),
			PlanName:   "cloud_pro",
			Status:     "unlicensed",
			IssuedAt:   now,
			ExpiresAt:  &now,
			Metadata:   `{"type":"cloud_tenant"}`,
			CreatedAt:  now,
			UpdatedAt:  now,
		})

		_, _ = m.repo.CreateOrGetSubdomain(customerID, licenseID, sub)
		_ = m.repo.UpdateSubdomainOwner(sub, req.OwnerName, req.Phone, sub)
	}

	tenant := &CloudTenant{
		ID:           customerID,
		Subdomain:    sub,
		Email:        email,
		PasswordHash: string(hashedPassword),
		OwnerName:    req.OwnerName,
		Phone:        req.Phone,
		Status:       "unlicensed",
		Plan:         "cloud_pro",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	log.Printf("[cloudtenant] ✅ Successfully registered Cloud Tenant [%s] (%s)", sub, email)
	return tenant, nil
}

func (m *Manager) SpawnTenantAgent(subdomain, token string) error {
	sub := strings.ToLower(strings.TrimSpace(subdomain))
	containerName := fmt.Sprintf("sasman-cloud-%s", sub)
	dataDir := m.pool.GetTenantDir(sub)

	// 1. Check if Docker CLI is available on host
	if _, err := exec.LookPath("docker"); err == nil {
		_ = exec.Command("docker", "rm", "-f", containerName).Run()

		imageName := "sasman:latest"
		if os.Getenv("SASMAN_AGENT_DOCKER_IMAGE") != "" {
			imageName = os.Getenv("SASMAN_AGENT_DOCKER_IMAGE")
		} else if _, err := exec.Command("docker", "inspect", "sasman-manager:latest").Output(); err == nil {
			imageName = "sasman-manager:latest"
		} else if _, err := exec.Command("docker", "inspect", "sasman:amd64").Output(); err == nil {
			imageName = "sasman:amd64"
		}

		centralURL := "ws://127.0.0.1:8080/api/tunnel/ws"
		if os.Getenv("SASMAN_INTERNAL_WS_URL") != "" {
			centralURL = os.Getenv("SASMAN_INTERNAL_WS_URL")
		}

		args := []string{
			"run", "-d",
			"--name", containerName,
			"--restart", "unless-stopped",
			"--network", "host",
			"-e", fmt.Sprintf("SASMAN_SUBDOMAIN=%s", sub),
			"-e", fmt.Sprintf("SASMAN_TUNNEL_TOKEN=%s", token),
			"-e", fmt.Sprintf("SASMAN_CENTRAL_URL=%s", centralURL),
			"-e", "CLOUD_MODE=true",
			"-e", "SQLITE_DB_PATH=/app/data/radius.db",
			"-v", fmt.Sprintf("%s:/app/data", dataDir),
			imageName,
		}

		cmd := exec.Command("docker", args...)
		output, err := cmd.CombinedOutput()
		if err == nil {
			log.Printf("[cloudtenant] 🚀 Successfully spawned Docker agent container [%s]", containerName)
			return nil
		}
		log.Printf("[cloudtenant] Docker spawn output for [%s]: %s (%v)", sub, string(output), err)
	}

	// 2. Binary execution (built directly inside container or host)
	agentBins := []string{
		"/app/sasman-agent",
		"./sasman-agent",
		"sasman-agent",
		"mikrotik-manager.exe",
		"./mikrotik-manager.exe",
		"sasman-agent-linux-amd64",
		"./sasman-agent-linux-amd64",
	}

	for _, bin := range agentBins {
		if _, err := os.Stat(bin); err == nil {
			cmd := exec.Command(bin)
			cmd.Env = append(os.Environ(),
				fmt.Sprintf("SASMAN_SUBDOMAIN=%s", sub),
				fmt.Sprintf("SASMAN_TUNNEL_TOKEN=%s", token),
				"SASMAN_CENTRAL_URL=ws://127.0.0.1:8080/api/tunnel/ws",
				"CLOUD_MODE=true",
				fmt.Sprintf("SQLITE_DB_PATH=%s", m.pool.GetTenantDBPath(sub)),
			)
			if err := cmd.Start(); err == nil {
				log.Printf("[cloudtenant] 🚀 Successfully spawned cloud agent process for [%s] using [%s]", sub, bin)
				return nil
			}
		}
	}

	return nil
}

func (m *Manager) EnsureAllCloudAgentsRunning() {
	if m.repo == nil {
		return
	}
	subdomains, err := m.repo.ListAllSubdomains()
	if err != nil {
		return
	}
	for _, sub := range subdomains {
		tenantDir := m.pool.GetTenantDir(sub)
		if _, err := os.Stat(tenantDir); err == nil {
			log.Printf("[cloudtenant] 🔄 Auto-resuming cloud agent for tenant [%s]", sub)
			_ = m.SpawnTenantAgent(sub, generateRandomToken(16))
		}
	}
}

func (m *Manager) AuthenticateTenant(loginID, password string) (*CloudTenant, string, error) {
	loginID = strings.ToLower(strings.TrimSpace(loginID))
	password = strings.TrimSpace(password)

	subdomain := loginID
	tenantDB, err := m.pool.Get(subdomain)
	if err != nil {
		return nil, "", fmt.Errorf("المستأجر السحابي غير موجود")
	}

	// Verify against tenant's admin table
	var hash string
	var role string
	var name, phone sql.NullString
	err = tenantDB.QueryRow("SELECT password, role, name, phone FROM radius_admins WHERE username = 'admin'").Scan(&hash, &role, &name, &phone)
	if err != nil {
		return nil, "", fmt.Errorf("بيانات تسجيل الدخول غير صحيحة")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, "", fmt.Errorf("كلمة المرور غير صحيحة")
	}

	// Generate JWT Token
	claims := jwt.MapClaims{
		"subdomain": subdomain,
		"role":      role,
		"type":      "cloud",
		"exp":       time.Now().Add(7 * 24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(m.jwtSecret)
	if err != nil {
		return nil, "", fmt.Errorf("فشل إصدار رمز الدخول: %w", err)
	}

	tenant := &CloudTenant{
		Subdomain: subdomain,
		OwnerName: name.String,
		Phone:     phone.String,
		Status:    "active",
		Plan:      "cloud_pro",
	}

	return tenant, tokenString, nil
}

func (m *Manager) VerifyCloudUser(subdomain, username, password string) (bool, string, string, string, error) {
	subdomain = strings.ToLower(strings.TrimSpace(subdomain))
	username = strings.TrimSpace(username)
	password = strings.TrimRight(strings.TrimSpace(password), "\x00")

	tenantDB, err := m.pool.Get(subdomain)
	if err != nil {
		return false, "", "", "قاعدة بيانات المستأجر غير متاحة", err
	}

	// 0. Check Tenant License Status in Central Repo
	if m.repo != nil {
		lic, err := m.repo.GetAgentLicenseInfo(subdomain)
		if err == nil && lic != nil && (lic.Status == "unlicensed" || lic.Status == "suspended" || lic.IsExpired) {
			log.Printf("[cloudtenant] ⛔ Tenant [%s] license check: Status=%s, Expired=%t, rejecting user [%s]", subdomain, lic.Status, lic.IsExpired, username)
			return false, "", "", fmt.Sprintf("اشتراك السحابة (%s) غير مفعّل أو منتهي الصلاحية، يرجى تفعيله من لوحة إدارة SASMAN", subdomain), fmt.Errorf("tenant unlicensed")
		}
	}

	// 1. Strip realm if provided
	lookupUser := username
	if strings.Contains(username, "@") {
		lookupUser = strings.Split(username, "@")[0]
	}

	// 2. Query radcheck for password
	var dbPass string
	err = tenantDB.QueryRow("SELECT value FROM radcheck WHERE (username = ? OR username = ?) AND attribute = 'Cleartext-Password'", username, lookupUser).Scan(&dbPass)
	if err != nil {
		// Check voucher
		var isUsed int
		var profileName string
		vErr := tenantDB.QueryRow("SELECT is_used, profile_name FROM radius_vouchers WHERE code = ? OR code = ?", username, lookupUser).Scan(&isUsed, &profileName)
		if vErr == nil {
			// Voucher exists
			rateLimit := "10M/10M"
			_ = tenantDB.QueryRow("SELECT value FROM radgroupreply WHERE groupname = ? AND attribute = 'Mikrotik-Rate-Limit'", profileName).Scan(&rateLimit)
			return true, rateLimit, username, "OK", nil
		}
		return false, "", "", "المستخدم غير مسجل", fmt.Errorf("user not found")
	}

	dbPass = strings.TrimRight(strings.TrimSpace(dbPass), "\x00")
	if password != "" && password != dbPass {
		return false, "", "", "كلمة المرور غير صحيحة", nil
	}

	// 3. Check expiration and active status in radius_user_meta
	var enabled int
	var expUnix sql.NullInt64
	metaErr := tenantDB.QueryRow("SELECT enabled, expiration_unix FROM radius_user_meta WHERE username = ? OR username = ?", username, lookupUser).Scan(&enabled, &expUnix)
	if metaErr == nil {
		if enabled == 0 {
			return false, "", "", "الحساب معطل", nil
		}
		if expUnix.Valid && expUnix.Int64 > 0 && expUnix.Int64 < time.Now().Unix() {
			return false, "", "", "انتهى اشتراك المستخدم", nil
		}
	}

	// 4. Query Rate Limit
	rateLimit := "10M/10M"
	var grpRate string
	err = tenantDB.QueryRow(`
		SELECT rgr.value 
		FROM radusergroup rug
		JOIN radgroupreply rgr ON rug.groupname = rgr.groupname
		WHERE (rug.username = ? OR rug.username = ?) AND rgr.attribute = 'Mikrotik-Rate-Limit'
		LIMIT 1
	`, username, lookupUser).Scan(&grpRate)
	if err == nil && grpRate != "" {
		rateLimit = grpRate
	}

	return true, rateLimit, dbPass, "OK", nil
}

func (m *Manager) RecordCloudAccounting(subdomain string, p CloudAccountingPayload) error {
	tenantDB, err := m.pool.Get(subdomain)
	if err != nil {
		return err
	}

	p.Username = strings.TrimSpace(p.Username)
	now := time.Now().Format("2006-01-02 15:04:05")

	switch p.StatusType {
	case "Start":
		_, err = tenantDB.Exec(`
			INSERT INTO radacct (
				acctsessionid, username, nasipaddress, acctstarttime, acctupdatetime,
				framedipaddress, callingstationid, acctinputoctets, acctoutputoctets
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, p.SessionID, p.Username, p.NasIP, now, now, p.UserIP, p.UserMAC, p.BytesIn, p.BytesOut)
	case "Stop":
		_, err = tenantDB.Exec(`
			UPDATE radacct SET 
				acctstoptime = ?,
				acctsessiontime = ?,
				acctinputoctets = ?,
				acctoutputoctets = ?,
				acctterminatecause = ?
			WHERE acctsessionid = ? OR (username = ? AND acctstoptime IS NULL)
		`, now, p.SessionTimeSec, p.BytesIn, p.BytesOut, p.TerminateCause, p.SessionID, p.Username)
	case "Interim-Update":
		res, err := tenantDB.Exec(`
			UPDATE radacct SET 
				acctupdatetime = ?,
				acctsessiontime = ?,
				acctinputoctets = ?,
				acctoutputoctets = ?,
				framedipaddress = CASE WHEN ? != '' THEN ? ELSE framedipaddress END,
				callingstationid = CASE WHEN ? != '' THEN ? ELSE callingstationid END
			WHERE acctsessionid = ? OR (username = ? AND acctstoptime IS NULL)
		`, now, p.SessionTimeSec, p.BytesIn, p.BytesOut, p.UserIP, p.UserIP, p.UserMAC, p.UserMAC, p.SessionID, p.Username)
		if err == nil {
			if rows, _ := res.RowsAffected(); rows == 0 {
				_, _ = tenantDB.Exec(`
					INSERT INTO radacct (
						acctsessionid, username, nasipaddress, acctstarttime, acctupdatetime,
						framedipaddress, callingstationid, acctinputoctets, acctoutputoctets, acctsessiontime
					) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				`, p.SessionID, p.Username, p.NasIP, now, now, p.UserIP, p.UserMAC, p.BytesIn, p.BytesOut, p.SessionTimeSec)
			}
		}
	}

	return err
}

func generateRandomToken(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (m *Manager) StartExpirationSweeper() {
	go func() {
		ticker := time.NewTicker(45 * time.Second)
		defer ticker.Stop()
		for {
			<-ticker.C
			m.runExpirationSweep()
		}
	}()
}

func (m *Manager) runExpirationSweep() {
	tenantsDir := m.pool.baseDir
	entries, err := os.ReadDir(tenantsDir)
	if err != nil {
		return
	}

	nowUnix := time.Now().Unix()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subdomain := entry.Name()
		db, err := m.pool.Get(subdomain)
		if err != nil {
			continue
		}

		rows, err := db.Query(`
			SELECT username FROM radius_user_meta 
			WHERE expiration_unix IS NOT NULL AND expiration_unix > 0 AND expiration_unix <= ?
		`, nowUnix)
		if err != nil {
			continue
		}

		expiredUsers := []string{}
		for rows.Next() {
			var u string
			if err := rows.Scan(&u); err == nil {
				expiredUsers = append(expiredUsers, u)
			}
		}
		rows.Close()

		for _, username := range expiredUsers {
			var sessionID, framedIP string
			err := db.QueryRow("SELECT COALESCE(acctsessionid, ''), COALESCE(framedipaddress, '') FROM radacct WHERE username = ? AND acctstoptime IS NULL ORDER BY radacctid DESC LIMIT 1", username).Scan(&sessionID, &framedIP)
			if err == nil && sessionID != "" {
				nowStr := time.Now().Format("2006-01-02 15:04:05")
				_, _ = db.Exec("UPDATE radacct SET acctstoptime = ? WHERE username = ? AND acctstoptime IS NULL", nowStr, username)

				_ = m.DisconnectCloudUser(subdomain, username, sessionID, framedIP)

				tenantLogPath := filepath.Join(m.pool.GetTenantDir(subdomain), "radius.log")
				line := fmt.Sprintf("[%s] RADIUS Disconnect-Request (Code 40) sent for expired user [%s] (Session: %s) ⏰\n",
					time.Now().Format("2006-01-02 15:04:05"), username, sessionID)
				f, err := os.OpenFile(tenantLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err == nil {
					_, _ = f.WriteString(line)
					_ = f.Close()
				}
			}
		}
	}
}
func (m *Manager) DeleteTenant(subdomain string) error {
	sub := strings.ToLower(strings.TrimSpace(subdomain))
	if sub == "" {
		return fmt.Errorf("empty subdomain")
	}

	log.Printf("[cloudtenant] 🗑️ Initiating complete deletion of tenant [%s]...", sub)

	// 1. Stop and remove Docker container if running
	containerName := fmt.Sprintf("sasman-cloud-%s", sub)
	_ = exec.Command("docker", "rm", "-f", containerName).Run()

	// 2. Close and remove DB connection from Pool
	if m.pool != nil {
		m.pool.Close(sub)
	}

	// 3. Remove Tenant Directory and SQLite database
	tenantDir := m.pool.GetTenantDir(sub)
	if err := os.RemoveAll(tenantDir); err != nil {
		log.Printf("[cloudtenant] Warning removing tenant dir [%s]: %v", tenantDir, err)
	}

	// 4. Remove PKI certificates
	_ = os.RemoveAll(filepath.Join("data", "pki", "agents", sub))
	_ = os.RemoveAll(filepath.Join("/app/data", "pki", "agents", sub))

	// 5. Delete from Central Repository (subdomains, licenses, customers)
	if m.repo != nil {
		_ = m.repo.DeleteSubdomain(sub)
	}

	log.Printf("[cloudtenant] ✅ Tenant [%s] and all associated data, databases, and subdomains have been permanently purged", sub)
	return nil
}


