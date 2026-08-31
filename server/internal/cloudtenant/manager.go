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

// HasTenant returns true ONLY if this subdomain is a CLOUD tenant.
// Primary check: agent_mode = 'cloud' in subdomains table (set at registration time).
// Fallback: radius.db file exists on disk (for tenants registered before agent_mode was added).
func (m *Manager) HasTenant(subdomain string) bool {
	sub := strings.ToLower(strings.TrimSpace(subdomain))
	if sub == "" {
		return false
	}
	// Primary: authoritative DB check
	if m.repo != nil && m.repo.IsCloudAgent(sub) {
		return true
	}
	// Fallback: legacy cloud tenants registered before agent_mode column was added
	dbPath := m.pool.GetTenantDBPath(sub)
	_, err := os.Stat(dbPath)
	return err == nil
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
		// ✅ Mark as cloud tenant — critical for routing isolation
		_ = m.repo.SetAgentMode(sub, "cloud")
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
			"-e", "ADDR=",
			"-e", "PORT=",
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
			env := []string{}
			for _, e := range os.Environ() {
				if !strings.HasPrefix(e, "ADDR=") && !strings.HasPrefix(e, "PORT=") {
					env = append(env, e)
				}
			}
			env = append(env,
				fmt.Sprintf("SASMAN_SUBDOMAIN=%s", sub),
				fmt.Sprintf("SASMAN_TUNNEL_TOKEN=%s", token),
				"SASMAN_CENTRAL_URL=ws://127.0.0.1:8080/api/tunnel/ws",
				"CLOUD_MODE=true",
				"ADDR=",
				"PORT=",
				fmt.Sprintf("SQLITE_DB_PATH=%s", m.pool.GetTenantDBPath(sub)),
			)
			cmd.Env = env
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

type CloudAuthDetails struct {
	Allow               bool
	RateLimit           string
	MikrotikGroup       string
	FramedPool          string
	Password            string
	TotalLimit          uint32
	TotalLimitGigawords uint32
	RejectReason        string
	Err                 error
}

func (m *Manager) VerifyCloudUserDetails(subdomain, username, password string) CloudAuthDetails {
	subdomain = strings.ToLower(strings.TrimSpace(subdomain))
	username = strings.TrimSpace(username)
	password = strings.TrimRight(strings.TrimSpace(password), "\x00")

	tenantDB, err := m.pool.Get(subdomain)
	if err != nil {
		return CloudAuthDetails{Allow: false, RejectReason: "قاعدة بيانات المستأجر غير متاحة", Err: err}
	}

	// 0. Check Tenant License Status in Central Repo
	if m.repo != nil {
		lic, err := m.repo.GetAgentLicenseInfo(subdomain)
		if err == nil && lic != nil && (lic.Status == "unlicensed" || lic.Status == "suspended" || lic.IsExpired) {
			log.Printf("[cloudtenant] ⛔ Tenant [%s] license check: Status=%s, Expired=%t, rejecting user [%s]", subdomain, lic.Status, lic.IsExpired, username)
			return CloudAuthDetails{
				Allow:        false,
				RejectReason: fmt.Sprintf("اشتراك السحابة (%s) غير مفعّل أو منتهي الصلاحية، يرجى تفعيله من لوحة إدارة SASMAN", subdomain),
				Err:          fmt.Errorf("tenant unlicensed"),
			}
		}
	}

	// 1. Strip realm if provided
	lookupUser := username
	if strings.Contains(username, "@") {
		lookupUser = strings.Split(username, "@")[0]
	}

	// 2. Query radcheck for password
	var dbPass string
	var groupName string
	err = tenantDB.QueryRow("SELECT value FROM radcheck WHERE (username = ? OR username = ?) AND attribute = 'Cleartext-Password'", username, lookupUser).Scan(&dbPass)
	if err != nil {
		// Check voucher
		var isUsed int
		var profileName string
		var valDays int
		vErr := tenantDB.QueryRow("SELECT COALESCE(is_used, 0), COALESCE(profile_name, '10M'), COALESCE(validity_days, 30) FROM radius_vouchers WHERE code = ? OR code = ?", username, lookupUser).Scan(&isUsed, &profileName, &valDays)
		if vErr == nil {
			if password != "" && password != username && password != lookupUser {
				return CloudAuthDetails{Allow: false, RejectReason: "كلمة المرور غير صحيحة"}
			}
			if isUsed == 0 {
				now := time.Now().Format("2006-01-02 15:04:05")
				_, _ = tenantDB.Exec("UPDATE radius_vouchers SET is_used = 1, used_by = ?, used_at = ? WHERE code = ? OR code = ?", lookupUser, now, username, lookupUser)
				if valDays <= 0 {
					valDays = 30
				}
				expTime := time.Now().Unix() + int64(valDays*86400)
				_, _ = tenantDB.Exec(`
					INSERT INTO radius_user_meta (username, full_name, enabled, expiration_unix, created_at, updated_at)
					VALUES (?, ?, 1, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
					ON CONFLICT(username) DO UPDATE SET expiration_unix=excluded.expiration_unix, enabled=1, updated_at=CURRENT_TIMESTAMP
				`, lookupUser, "كارت "+profileName, expTime)
			}
			groupName = profileName
			rateLimit := "10M/10M"
			var mtGroup, pool string
			_ = tenantDB.QueryRow("SELECT value FROM radgroupreply WHERE groupname = ? AND attribute = 'Mikrotik-Rate-Limit'", profileName).Scan(&rateLimit)
			_ = tenantDB.QueryRow("SELECT value FROM radgroupreply WHERE groupname = ? AND attribute = 'Mikrotik-Group'", profileName).Scan(&mtGroup)
			_ = tenantDB.QueryRow("SELECT value FROM radgroupreply WHERE groupname = ? AND attribute = 'Framed-Pool'", profileName).Scan(&pool)
			return CloudAuthDetails{
				Allow:         true,
				RateLimit:     rateLimit,
				MikrotikGroup: mtGroup,
				FramedPool:    pool,
				Password:      username,
				RejectReason:  "OK",
			}
		}
		return CloudAuthDetails{Allow: false, RejectReason: "المستخدم غير مسجل لدى هذا الوكيل", Err: fmt.Errorf("user not found")}
	}

	dbPass = strings.TrimRight(strings.TrimSpace(dbPass), "\x00")
	if password != "" && password != dbPass {
		return CloudAuthDetails{Allow: false, RejectReason: "كلمة المرور غير صحيحة"}
	}

	// 3. Check expiration, active status, and quota in radius_user_meta
	var enabled int
	var expUnix sql.NullInt64
	var quotaLimitMB, usedIn, usedOut int64
	var quotaStatus string
	metaErr := tenantDB.QueryRow("SELECT enabled, expiration_unix, COALESCE(quota_limit_mb, 0), COALESCE(used_octets_in, 0), COALESCE(used_octets_out, 0), COALESCE(quota_status, 'active') FROM radius_user_meta WHERE username = ? OR username = ?", username, lookupUser).Scan(&enabled, &expUnix, &quotaLimitMB, &usedIn, &usedOut, &quotaStatus)
	if metaErr == nil {
		if enabled == 0 {
			return CloudAuthDetails{Allow: false, RejectReason: "الحساب معطل"}
		}
		if expUnix.Valid && expUnix.Int64 > 0 && expUnix.Int64 < time.Now().Unix() {
			return CloudAuthDetails{Allow: false, RejectReason: "انتهى اشتراك المستخدم"}
		}
	}

	// 4. Query Group, Rate Limit, Mikrotik-Group, Framed-Pool
	_ = tenantDB.QueryRow("SELECT groupname FROM radusergroup WHERE username = ? OR username = ? ORDER BY priority ASC LIMIT 1", username, lookupUser).Scan(&groupName)
	if groupName == "" {
		groupName = "10M"
	}

	if quotaLimitMB == 0 {
		_ = tenantDB.QueryRow("SELECT COALESCE(quota_limit_mb, 0) FROM radius_profile_meta WHERE groupname = ?", groupName).Scan(&quotaLimitMB)
	}

	var totalLimitLow, totalLimitGiga uint32
	if quotaLimitMB > 0 {
		totalQuotaBytes := quotaLimitMB * 1024 * 1024
		usedTotal := usedIn + usedOut
		remainingBytes := totalQuotaBytes - usedTotal
		if remainingBytes <= 0 || quotaStatus == "depleted" {
			return CloudAuthDetails{Allow: false, RejectReason: "تم استهلاك باقة البيانات بالكامل (Quota Depleted)"}
		}
		totalLimitLow = uint32(remainingBytes % (1 << 32))
		totalLimitGiga = uint32(remainingBytes >> 32)
	}

	rateLimit := "10M/10M"
	var mtGroup, pool string
	_ = tenantDB.QueryRow("SELECT value FROM radgroupreply WHERE groupname = ? AND attribute = 'Mikrotik-Rate-Limit'", groupName).Scan(&rateLimit)
	_ = tenantDB.QueryRow("SELECT value FROM radgroupreply WHERE groupname = ? AND attribute = 'Mikrotik-Group'", groupName).Scan(&mtGroup)
	_ = tenantDB.QueryRow("SELECT value FROM radgroupreply WHERE groupname = ? AND attribute = 'Framed-Pool'", groupName).Scan(&pool)

	return CloudAuthDetails{
		Allow:               true,
		RateLimit:           rateLimit,
		MikrotikGroup:       mtGroup,
		FramedPool:          pool,
		Password:            dbPass,
		TotalLimit:          totalLimitLow,
		TotalLimitGigawords: totalLimitGiga,
		RejectReason:        "OK",
	}
}

func (m *Manager) VerifyCloudUser(subdomain, username, password string) (bool, string, string, string, error) {
	d := m.VerifyCloudUserDetails(subdomain, username, password)
	return d.Allow, d.RateLimit, d.Password, d.RejectReason, d.Err
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
		res, err := tenantDB.Exec(`
			UPDATE radacct SET 
				acctstoptime = ?,
				acctsessiontime = ?,
				acctinputoctets = ?,
				acctoutputoctets = ?,
				acctterminatecause = ?
			WHERE acctsessionid = ? OR (username = ? AND acctstoptime IS NULL)
		`, now, p.SessionTimeSec, p.BytesIn, p.BytesOut, p.TerminateCause, p.SessionID, p.Username)
		if err == nil {
			if rows, _ := res.RowsAffected(); rows == 0 {
				_, _ = tenantDB.Exec(`
					INSERT INTO radacct (
						acctsessionid, username, nasipaddress, acctstarttime, acctupdatetime, acctstoptime,
						framedipaddress, callingstationid, acctinputoctets, acctoutputoctets, acctsessiontime, acctterminatecause
					) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				`, p.SessionID, p.Username, p.NasIP, now, now, now, p.UserIP, p.UserMAC, p.BytesIn, p.BytesOut, p.SessionTimeSec, p.TerminateCause)
			}
		}
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

	if p.StatusType == "Stop" || p.StatusType == "Interim-Update" {
		_, _ = tenantDB.Exec(`
			UPDATE radius_user_meta 
			SET used_octets_in = (SELECT COALESCE(SUM(acctinputoctets), 0) FROM radacct WHERE username = radius_user_meta.username),
			    used_octets_out = (SELECT COALESCE(SUM(acctoutputoctets), 0) FROM radacct WHERE username = radius_user_meta.username),
			    updated_at = CURRENT_TIMESTAMP
			WHERE username = ?
		`, p.Username)

		var qLimitMB, uIn, uOut int64
		_ = tenantDB.QueryRow("SELECT COALESCE(quota_limit_mb, 0), COALESCE(used_octets_in, 0), COALESCE(used_octets_out, 0) FROM radius_user_meta WHERE username = ?", p.Username).Scan(&qLimitMB, &uIn, &uOut)
		if qLimitMB == 0 {
			var pName string
			_ = tenantDB.QueryRow("SELECT groupname FROM radusergroup WHERE username = ? ORDER BY priority LIMIT 1", p.Username).Scan(&pName)
			if pName != "" {
				_ = tenantDB.QueryRow("SELECT COALESCE(quota_limit_mb, 0) FROM radius_profile_meta WHERE groupname = ?", pName).Scan(&qLimitMB)
			}
		}
		if qLimitMB > 0 {
			totalQuota := qLimitMB * 1024 * 1024
			if (uIn + uOut) >= totalQuota {
				_, _ = tenantDB.Exec("UPDATE radius_user_meta SET quota_status = 'depleted' WHERE username = ?", p.Username)
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


