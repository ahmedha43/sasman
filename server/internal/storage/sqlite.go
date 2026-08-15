package storage

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"mikrotik-manager/pkg/ota"

	_ "modernc.org/sqlite"
)

type Customer struct {
	ID          string
	Name        string
	Phone       string
	Email       string
	CompanyName string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type License struct {
	ID         string
	CustomerID string
	LicenseKey string
	HWUUID     string
	PlanName   string
	Status     string
	IssuedAt   time.Time
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
	Metadata   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Subdomain struct {
	ID         string
	CustomerID string
	LicenseID  string
	Subdomain  string
	ZoneName   string
	Status     string
	Token      string
	WinboxPort int
	GroupName  string
	AssignedAt time.Time
	RevokedAt  *time.Time
	LastSeenAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type AgentLicenseInfo struct {
	Subdomain     string     `json:"subdomain"`
	LicenseID     string     `json:"license_id"`
	Status        string     `json:"status"` // 'active', 'suspended', 'expired', 'unlicensed'
	ExpiresAt     *time.Time `json:"expires_at"`
	ExpiresAtStr  string     `json:"expires_at_str"`
	DaysRemaining int        `json:"days_remaining"`
	IsExpired     bool       `json:"is_expired"`
	DurationDays  int        `json:"duration_days"`
	LastRenewedAt *time.Time `json:"last_renewed_at"`
}

type Broadcast struct {
	ID                string     `json:"id"`
	Title             string     `json:"title"`
	Message           string     `json:"message"`
	ImageURL          string     `json:"image_url"`
	ActionURL         string     `json:"action_url"`
	ActionText        string     `json:"action_text"`
	DisplayType       string     `json:"display_type"`       // 'banner' | 'modal' | 'splash'
	TargetType        string     `json:"target_type"`        // 'agents' | 'users' | 'both'
	TargetAgents      string     `json:"target_agents"`      // 'ALL' or comma/JSON array
	TargetProfiles    string     `json:"target_profiles"`    // 'ALL' or comma/JSON array
	Frequency         string     `json:"frequency"`          // 'once' | 'daily' | 'always'
	SplashDurationSec int        `json:"splash_duration_sec"`// seconds for broadband countdown
	StartAt           *time.Time `json:"start_at"`
	EndAt             *time.Time `json:"end_at"`
	Status            string     `json:"status"`             // 'active' | 'paused' | 'expired'
	CreatedBy         string     `json:"created_by"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type BroadcastLog struct {
	BroadcastID    string    `json:"broadcast_id"`
	AgentID        string    `json:"agent_id"`
	UserIdentifier string    `json:"user_identifier"` // IP, PPPoE username, or 'panel'
	ViewedAt       time.Time `json:"viewed_at"`
	Clicked        int       `json:"clicked"`
}

type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(path string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Performance Pragmas: WAL mode, Normal sync, Memory temp store, and 5s busy timeout
	_, _ = db.Exec("PRAGMA journal_mode = WAL;")
	_, _ = db.Exec("PRAGMA synchronous = NORMAL;")
	_, _ = db.Exec("PRAGMA temp_store = MEMORY;")
	_, _ = db.Exec("PRAGMA busy_timeout = 5000;")

	return &SQLiteRepository{db: db}, nil
}

func (r *SQLiteRepository) CreateSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS customers (
            id TEXT PRIMARY KEY,
            name TEXT NOT NULL,
            email TEXT,
            company_name TEXT,
            status TEXT NOT NULL DEFAULT 'active',
            created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
        );`,
		`CREATE TABLE IF NOT EXISTS licenses (
            id TEXT PRIMARY KEY,
            customer_id TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
            license_key TEXT NOT NULL UNIQUE,
            hw_uuid TEXT UNIQUE,
            plan_name TEXT NOT NULL DEFAULT 'basic',
            status TEXT NOT NULL DEFAULT 'active',
            issued_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
            expires_at TEXT,
            revoked_at TEXT,
            metadata TEXT NOT NULL DEFAULT '{}',
            created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
        );`,
		`CREATE TABLE IF NOT EXISTS subdomains (
            id TEXT PRIMARY KEY,
            customer_id TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
            license_id TEXT NOT NULL REFERENCES licenses(id) ON DELETE CASCADE,
            subdomain TEXT NOT NULL UNIQUE,
            zone_name TEXT NOT NULL DEFAULT 'sas-man.net',
            status TEXT NOT NULL DEFAULT 'active',
            token TEXT NOT NULL,
            winbox_port INTEGER NOT NULL,
            group_name TEXT NOT NULL DEFAULT 'default',
            assigned_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
            revoked_at TEXT,
            last_seen_at TEXT,
            created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
        );`,
		`CREATE INDEX IF NOT EXISTS idx_licenses_customer_id ON licenses(customer_id);`,
		`CREATE INDEX IF NOT EXISTS idx_subdomains_customer_id ON subdomains(customer_id);`,
		`CREATE INDEX IF NOT EXISTS idx_subdomains_subdomain ON subdomains(subdomain);`,
		`CREATE TABLE IF NOT EXISTS ota_releases (
            version TEXT NOT NULL,
            channel TEXT NOT NULL DEFAULT 'stable',
            target_arch TEXT NOT NULL,
            binary_data BLOB NOT NULL,
            sha256 TEXT NOT NULL,
            signature_ed25519 TEXT NOT NULL,
            min_agent_version TEXT NOT NULL DEFAULT '5.0.0',
            release_notes TEXT NOT NULL DEFAULT '',
            created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY (version, target_arch)
        );`,
		`CREATE TABLE IF NOT EXISTS agent_ota_status (
            subdomain TEXT PRIMARY KEY,
            current_version TEXT NOT NULL DEFAULT 'v5.0.0',
            target_version TEXT NOT NULL DEFAULT '',
            arch TEXT NOT NULL DEFAULT '',
            status TEXT NOT NULL DEFAULT 'idle',
            last_error TEXT NOT NULL DEFAULT '',
            last_attempt_at TEXT,
            updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
        );`,
		`CREATE TABLE IF NOT EXISTS broadcasts (
            id TEXT PRIMARY KEY,
            title TEXT NOT NULL,
            message TEXT NOT NULL,
            image_url TEXT NOT NULL DEFAULT '',
            action_url TEXT NOT NULL DEFAULT '',
            action_text TEXT NOT NULL DEFAULT '',
            display_type TEXT NOT NULL DEFAULT 'banner',
            target_type TEXT NOT NULL DEFAULT 'agents',
            target_agents TEXT NOT NULL DEFAULT 'ALL',
            target_profiles TEXT NOT NULL DEFAULT 'ALL',
            frequency TEXT NOT NULL DEFAULT 'once',
            splash_duration_sec INTEGER NOT NULL DEFAULT 10,
            start_at TEXT,
            end_at TEXT,
            status TEXT NOT NULL DEFAULT 'active',
            created_by TEXT NOT NULL DEFAULT 'admin',
            created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
        );`,
		`CREATE INDEX IF NOT EXISTS idx_broadcasts_status ON broadcasts(status);`,
		`CREATE TABLE IF NOT EXISTS broadcast_logs (
            broadcast_id TEXT NOT NULL,
            agent_id TEXT NOT NULL,
            user_identifier TEXT NOT NULL,
            viewed_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
            clicked INTEGER NOT NULL DEFAULT 0,
            PRIMARY KEY (broadcast_id, agent_id, user_identifier)
        );`,
		`CREATE INDEX IF NOT EXISTS idx_broadcast_logs_bid ON broadcast_logs(broadcast_id);`,
	}

	for _, q := range queries {
		if _, err := r.db.Exec(q); err != nil {
			return fmt.Errorf("exec %s: %w", q, err)
		}
	}

	// Column migration for existing db
	_, _ = r.db.Exec("ALTER TABLE customers ADD COLUMN phone TEXT NOT NULL DEFAULT ''")
	_, _ = r.db.Exec("ALTER TABLE subdomains ADD COLUMN token TEXT NOT NULL DEFAULT ''")
	_, _ = r.db.Exec("ALTER TABLE subdomains ADD COLUMN winbox_port INTEGER NOT NULL DEFAULT 0")
	_, _ = r.db.Exec("ALTER TABLE subdomains ADD COLUMN group_name TEXT NOT NULL DEFAULT 'default'")
	_, _ = r.db.Exec("ALTER TABLE subdomains ADD COLUMN agent_version TEXT NOT NULL DEFAULT 'v5.0.0'")
	_, _ = r.db.Exec("ALTER TABLE subdomains ADD COLUMN agent_arch TEXT NOT NULL DEFAULT 'linux_arm64'")

	// Create group_name index after migration
	_, _ = r.db.Exec("CREATE INDEX IF NOT EXISTS idx_subdomains_group_name ON subdomains(group_name);")

	return nil
}

func (r *SQLiteRepository) IsSubdomainAvailable(subdomain string) (bool, error) {
	subdomain = strings.ToLower(strings.TrimSpace(subdomain))
	if subdomain == "" {
		return false, fmt.Errorf("empty subdomain")
	}
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM subdomains WHERE LOWER(subdomain) = ?", subdomain).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func (r *SQLiteRepository) SaveCustomer(c Customer) error {
	_, err := r.db.Exec(`
        INSERT INTO customers (id, name, phone, email, company_name, status, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            name=excluded.name,
            phone=excluded.phone,
            email=excluded.email,
            company_name=excluded.company_name,
            status=excluded.status,
            updated_at=excluded.updated_at
    `, c.ID, c.Name, c.Phone, c.Email, c.CompanyName, c.Status, c.CreatedAt.UTC().Format(time.RFC3339), c.UpdatedAt.UTC().Format(time.RFC3339))
	return err
}

func (r *SQLiteRepository) SaveLicense(l License) error {
	var expiresAt, revokedAt interface{}
	if l.ExpiresAt != nil {
		expiresAt = l.ExpiresAt.UTC().Format(time.RFC3339)
	}
	if l.RevokedAt != nil {
		revokedAt = l.RevokedAt.UTC().Format(time.RFC3339)
	}

	_, err := r.db.Exec(`
        INSERT INTO licenses (id, customer_id, license_key, hw_uuid, plan_name, status, issued_at, expires_at, revoked_at, metadata, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            customer_id=excluded.customer_id,
            license_key=excluded.license_key,
            hw_uuid=excluded.hw_uuid,
            plan_name=excluded.plan_name,
            status=excluded.status,
            expires_at=excluded.expires_at,
            revoked_at=excluded.revoked_at,
            metadata=excluded.metadata,
            updated_at=excluded.updated_at
    `, l.ID, l.CustomerID, l.LicenseKey, l.HWUUID, l.PlanName, l.Status, l.IssuedAt.UTC().Format(time.RFC3339), expiresAt, revokedAt, l.Metadata, l.CreatedAt.UTC().Format(time.RFC3339), l.UpdatedAt.UTC().Format(time.RFC3339))
	return err
}

func (r *SQLiteRepository) CreateOrGetSubdomain(customerID, licenseID, subdomain string) (*Subdomain, error) {
	row := r.db.QueryRow(`SELECT id, customer_id, license_id, subdomain, zone_name, status, token, winbox_port, group_name, assigned_at, revoked_at, last_seen_at, created_at, updated_at FROM subdomains WHERE subdomain = ?`, subdomain)
	var s Subdomain
	var assignedAt, revokedAt, lastSeenAt, createdAt, updatedAt string
	err := row.Scan(&s.ID, &s.CustomerID, &s.LicenseID, &s.Subdomain, &s.ZoneName, &s.Status, &s.Token, &s.WinboxPort, &s.GroupName, &assignedAt, &revokedAt, &lastSeenAt, &createdAt, &updatedAt)
	if err == nil {
		return &s, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	id := fmt.Sprintf("sub-%d", time.Now().UnixNano())
	token := "default-token"
	port := 0
	groupName := "default"
	_, err = r.db.Exec(`
        INSERT INTO subdomains (id, customer_id, license_id, subdomain, zone_name, status, token, winbox_port, group_name, assigned_at, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `, id, customerID, licenseID, subdomain, "sas-man.net", "active", token, port, groupName, time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}

	return &Subdomain{ID: id, CustomerID: customerID, LicenseID: licenseID, Subdomain: subdomain, ZoneName: "sas-man.net", Status: "active", Token: token, WinboxPort: port, GroupName: groupName}, nil
}

func (r *SQLiteRepository) SaveSubdomain(s Subdomain) error {
	assignedAtStr := s.AssignedAt.UTC().Format(time.RFC3339)
	if s.AssignedAt.IsZero() {
		assignedAtStr = time.Now().UTC().Format(time.RFC3339)
	}
	createdAtStr := s.CreatedAt.UTC().Format(time.RFC3339)
	if s.CreatedAt.IsZero() {
		createdAtStr = time.Now().UTC().Format(time.RFC3339)
	}
	updatedAtStr := time.Now().UTC().Format(time.RFC3339)

	_, err := r.db.Exec(`
        INSERT INTO subdomains (id, customer_id, license_id, subdomain, zone_name, status, token, winbox_port, group_name, assigned_at, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            customer_id=excluded.customer_id,
            license_id=excluded.license_id,
            subdomain=excluded.subdomain,
            zone_name=excluded.zone_name,
            status=excluded.status,
            token=excluded.token,
            winbox_port=excluded.winbox_port,
            group_name=excluded.group_name,
            updated_at=excluded.updated_at
    `, s.ID, s.CustomerID, s.LicenseID, s.Subdomain, s.ZoneName, s.Status, s.Token, s.WinboxPort, s.GroupName, assignedAtStr, createdAtStr, updatedAtStr)
	return err
}

func (r *SQLiteRepository) ResetSubdomain(subdomain string) error {
	_, err := r.db.Exec(`
        UPDATE subdomains
        SET status = 'active', revoked_at = NULL, updated_at = ?, last_seen_at = ?
        WHERE subdomain = ?
    `, time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339), subdomain)
	return err
}

func (r *SQLiteRepository) DeleteSubdomain(subdomain string) error {
	_, err := r.db.Exec(`DELETE FROM subdomains WHERE subdomain = ?`, subdomain)
	return err
}

func (r *SQLiteRepository) UpdateSubdomainStatus(subdomain, token string, port int) error {
	_, err := r.db.Exec(`
        UPDATE subdomains
        SET token = ?, winbox_port = ?, updated_at = ?
        WHERE subdomain = ?
    `, token, port, time.Now().UTC().Format(time.RFC3339), subdomain)
	return err
}

func (r *SQLiteRepository) UpdateSubdomainGroup(subdomain, groupName string) error {
	if groupName == "" {
		groupName = "default"
	}
	_, err := r.db.Exec(`
        UPDATE subdomains
        SET group_name = ?, updated_at = ?
        WHERE subdomain = ?
    `, groupName, time.Now().UTC().Format(time.RFC3339), subdomain)
	return err
}

func (r *SQLiteRepository) GetSubdomainOwners() (map[string]Customer, error) {
	rows, err := r.db.Query(`
        SELECT s.subdomain, c.id, c.name, c.phone, c.email, c.company_name
        FROM subdomains s
        LEFT JOIN customers c ON s.customer_id = c.id
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	owners := make(map[string]Customer)
	for rows.Next() {
		var sub string
		var c Customer
		var id, name, phone, email, comp sql.NullString
		if err := rows.Scan(&sub, &id, &name, &phone, &email, &comp); err == nil {
			c.ID = id.String
			c.Name = name.String
			c.Phone = phone.String
			c.Email = email.String
			c.CompanyName = comp.String
			owners[strings.ToLower(strings.TrimSpace(sub))] = c
		}
	}
	return owners, nil
}

func (r *SQLiteRepository) GetSubdomainGroup(subdomain string) string {
	var group string
	err := r.db.QueryRow("SELECT group_name FROM subdomains WHERE subdomain = ?", subdomain).Scan(&group)
	if err != nil || group == "" {
		return "default"
	}
	return group
}

func (r *SQLiteRepository) ListAgentGroups() ([]string, error) {
	rows, err := r.db.Query("SELECT DISTINCT group_name FROM subdomains WHERE group_name != '' ORDER BY group_name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err == nil {
			groups = append(groups, g)
		}
	}
	return groups, nil
}

func (r *SQLiteRepository) ListAllSubdomains() ([]string, error) {
	rows, err := r.db.Query("SELECT subdomain FROM subdomains WHERE status = 'active'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err == nil {
			list = append(list, s)
		}
	}
	return list, nil
}

func (r *SQLiteRepository) GetSubdomainArch(subdomain string) (string, error) {
	var arch string
	err := r.db.QueryRow("SELECT COALESCE(agent_arch, 'linux_arm64') FROM subdomains WHERE subdomain = ?", subdomain).Scan(&arch)
	if err != nil {
		return "linux_arm64", err
	}
	return arch, nil
}

func (r *SQLiteRepository) Close() error {
	return r.db.Close()
}

func (r *SQLiteRepository) GetDB() *sql.DB {
	return r.db
}

func (r *SQLiteRepository) BackupTo(destPath string) error {
	_ = os.Remove(destPath)
	_, err := r.db.Exec(fmt.Sprintf("VACUUM INTO '%s'", strings.ReplaceAll(destPath, "'", "''")))
	return err
}

func (r *SQLiteRepository) Reopen(path string) error {
	_ = r.db.Close()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		return err
	}

	_, _ = db.Exec("PRAGMA journal_mode = WAL;")
	_, _ = db.Exec("PRAGMA synchronous = NORMAL;")
	_, _ = db.Exec("PRAGMA temp_store = MEMORY;")
	_, _ = db.Exec("PRAGMA busy_timeout = 5000;")

	r.db = db
	return r.CreateSchema()
}

// ─── OTA Releases & Status Storage ──────────────────────────────────────────

func (r *SQLiteRepository) SaveRelease(m ota.ReleaseManifest, binaryData []byte) error {
	_, err := r.db.Exec(`
        INSERT INTO ota_releases (version, channel, target_arch, binary_data, sha256, signature_ed25519, min_agent_version, release_notes, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(version, target_arch) DO UPDATE SET
            channel=excluded.channel,
            binary_data=excluded.binary_data,
            sha256=excluded.sha256,
            signature_ed25519=excluded.signature_ed25519,
            min_agent_version=excluded.min_agent_version,
            release_notes=excluded.release_notes,
            created_at=excluded.created_at
    `, m.Version, m.Channel, m.TargetArch, binaryData, m.Sha256, m.SignatureEd25519, m.MinAgentVersion, m.ReleaseNotes, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (r *SQLiteRepository) GetRelease(version string, targetArch string) (*ota.ReleaseManifest, []byte, error) {
	row := r.db.QueryRow(`
        SELECT version, channel, target_arch, binary_data, sha256, signature_ed25519, min_agent_version, release_notes, created_at
        FROM ota_releases WHERE version = ? AND target_arch = ?
    `, version, targetArch)

	var m ota.ReleaseManifest
	var binaryData []byte
	var createdAtStr string
	err := row.Scan(&m.Version, &m.Channel, &m.TargetArch, &binaryData, &m.Sha256, &m.SignatureEd25519, &m.MinAgentVersion, &m.ReleaseNotes, &createdAtStr)
	if err != nil {
		return nil, nil, err
	}
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	m.BinaryURL = fmt.Sprintf("/api/ota/bin/%s/%s", m.TargetArch, m.Version)
	return &m, binaryData, nil
}

func (r *SQLiteRepository) ListReleases() ([]ota.ReleaseManifest, error) {
	rows, err := r.db.Query(`
        SELECT version, channel, target_arch, sha256, signature_ed25519, min_agent_version, release_notes, created_at
        FROM ota_releases ORDER BY created_at DESC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ota.ReleaseManifest
	for rows.Next() {
		var m ota.ReleaseManifest
		var createdAtStr string
		if err := rows.Scan(&m.Version, &m.Channel, &m.TargetArch, &m.Sha256, &m.SignatureEd25519, &m.MinAgentVersion, &m.ReleaseNotes, &createdAtStr); err == nil {
			m.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
			m.BinaryURL = fmt.Sprintf("/api/ota/bin/%s/%s", m.TargetArch, m.Version)
			list = append(list, m)
		}
	}
	return list, nil
}

func (r *SQLiteRepository) DeleteRelease(version string, targetArch string) error {
	var err error
	if targetArch == "" || targetArch == "all" {
		_, err = r.db.Exec(`DELETE FROM ota_releases WHERE version = ?`, version)
	} else {
		_, err = r.db.Exec(`DELETE FROM ota_releases WHERE version = ? AND target_arch = ?`, version, targetArch)
	}
	return err
}

func (r *SQLiteRepository) UpdateAgentOTAStatus(st ota.AgentOTAStatus) error {
	nowStr := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(`
        INSERT INTO agent_ota_status (subdomain, current_version, target_version, arch, status, last_error, last_attempt_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(subdomain) DO UPDATE SET
            current_version=CASE WHEN excluded.current_version != '' THEN excluded.current_version ELSE agent_ota_status.current_version END,
            target_version=CASE WHEN excluded.target_version != '' THEN excluded.target_version ELSE agent_ota_status.target_version END,
            arch=CASE WHEN excluded.arch != '' THEN excluded.arch ELSE agent_ota_status.arch END,
            status=excluded.status,
            last_error=excluded.last_error,
            last_attempt_at=excluded.last_attempt_at,
            updated_at=excluded.updated_at
    `, st.Subdomain, st.CurrentVer, st.TargetVer, st.Arch, st.Status, st.LastError, nowStr, nowStr)
	return err
}

func (r *SQLiteRepository) GetAgentOTAStatuses() (map[string]ota.AgentOTAStatus, error) {
	rows, err := r.db.Query(`
        SELECT s.subdomain, COALESCE(o.current_version, s.agent_version), COALESCE(o.target_version, ''), COALESCE(o.arch, s.agent_arch), COALESCE(o.status, 'idle'), COALESCE(o.last_error, ''), COALESCE(o.updated_at, s.updated_at)
        FROM subdomains s
        LEFT JOIN agent_ota_status o ON s.subdomain = o.subdomain
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]ota.AgentOTAStatus)
	for rows.Next() {
		var st ota.AgentOTAStatus
		var updatedStr string
		if err := rows.Scan(&st.Subdomain, &st.CurrentVer, &st.TargetVer, &st.Arch, &st.Status, &st.LastError, &updatedStr); err == nil {
			st.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
			res[st.Subdomain] = st
		}
	}
	return res, nil
}

func (r *SQLiteRepository) UpdateAgentVersionAndArch(subdomain, version, arch string) error {
	_, err := r.db.Exec(`
        UPDATE subdomains 
        SET agent_version = ?, agent_arch = ?, updated_at = CURRENT_TIMESTAMP
        WHERE subdomain = ?
    `, version, arch, subdomain)
	if err == nil {
		_ = r.UpdateAgentOTAStatus(ota.AgentOTAStatus{
			Subdomain:  subdomain,
			CurrentVer: version,
			Arch:       arch,
			Status:     "updated",
		})
	}
	return err
}

// ─── Broadcasts & Ads Storage ────────────────────────────────────────────────

func (r *SQLiteRepository) SaveBroadcast(b Broadcast) error {
	var startAt, endAt interface{}
	if b.StartAt != nil {
		startAt = b.StartAt.UTC().Format(time.RFC3339)
	}
	if b.EndAt != nil {
		endAt = b.EndAt.UTC().Format(time.RFC3339)
	}

	if b.Status == "" {
		b.Status = "active"
	}
	if b.DisplayType == "" {
		b.DisplayType = "banner"
	}
	if b.TargetType == "" {
		b.TargetType = "agents"
	}
	if b.TargetAgents == "" {
		b.TargetAgents = "ALL"
	}
	if b.TargetProfiles == "" {
		b.TargetProfiles = "ALL"
	}
	if b.Frequency == "" {
		b.Frequency = "once"
	}
	if b.SplashDurationSec <= 0 {
		b.SplashDurationSec = 10
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)

	_, err := r.db.Exec(`
        INSERT INTO broadcasts (
            id, title, message, image_url, action_url, action_text,
            display_type, target_type, target_agents, target_profiles,
            frequency, splash_duration_sec, start_at, end_at, status,
            created_by, created_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            title=excluded.title,
            message=excluded.message,
            image_url=excluded.image_url,
            action_url=excluded.action_url,
            action_text=excluded.action_text,
            display_type=excluded.display_type,
            target_type=excluded.target_type,
            target_agents=excluded.target_agents,
            target_profiles=excluded.target_profiles,
            frequency=excluded.frequency,
            splash_duration_sec=excluded.splash_duration_sec,
            start_at=excluded.start_at,
            end_at=excluded.end_at,
            status=excluded.status,
            updated_at=excluded.updated_at
    `, b.ID, b.Title, b.Message, b.ImageURL, b.ActionURL, b.ActionText,
		b.DisplayType, b.TargetType, b.TargetAgents, b.TargetProfiles,
		b.Frequency, b.SplashDurationSec, startAt, endAt, b.Status,
		b.CreatedBy, nowStr, nowStr)
	return err
}

func (r *SQLiteRepository) GetBroadcast(id string) (*Broadcast, error) {
	row := r.db.QueryRow(`
        SELECT id, title, message, image_url, action_url, action_text,
               display_type, target_type, target_agents, target_profiles,
               frequency, splash_duration_sec, start_at, end_at, status,
               created_by, created_at, updated_at
        FROM broadcasts WHERE id = ?
    `, id)

	var b Broadcast
	var startStr, endStr sql.NullString
	var createdStr, updatedStr string
	err := row.Scan(&b.ID, &b.Title, &b.Message, &b.ImageURL, &b.ActionURL, &b.ActionText,
		&b.DisplayType, &b.TargetType, &b.TargetAgents, &b.TargetProfiles,
		&b.Frequency, &b.SplashDurationSec, &startStr, &endStr, &b.Status,
		&b.CreatedBy, &createdStr, &updatedStr)
	if err != nil {
		return nil, err
	}

	b.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	b.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
	if startStr.Valid && startStr.String != "" {
		t, _ := time.Parse(time.RFC3339, startStr.String)
		b.StartAt = &t
	}
	if endStr.Valid && endStr.String != "" {
		t, _ := time.Parse(time.RFC3339, endStr.String)
		b.EndAt = &t
	}
	return &b, nil
}

func (r *SQLiteRepository) ListBroadcasts() ([]Broadcast, error) {
	rows, err := r.db.Query(`
        SELECT id, title, message, image_url, action_url, action_text,
               display_type, target_type, target_agents, target_profiles,
               frequency, splash_duration_sec, start_at, end_at, status,
               created_by, created_at, updated_at
        FROM broadcasts ORDER BY created_at DESC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Broadcast
	for rows.Next() {
		var b Broadcast
		var startStr, endStr sql.NullString
		var createdStr, updatedStr string
		if err := rows.Scan(&b.ID, &b.Title, &b.Message, &b.ImageURL, &b.ActionURL, &b.ActionText,
			&b.DisplayType, &b.TargetType, &b.TargetAgents, &b.TargetProfiles,
			&b.Frequency, &b.SplashDurationSec, &startStr, &endStr, &b.Status,
			&b.CreatedBy, &createdStr, &updatedStr); err == nil {
			b.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
			b.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
			if startStr.Valid && startStr.String != "" {
				t, _ := time.Parse(time.RFC3339, startStr.String)
				b.StartAt = &t
			}
			if endStr.Valid && endStr.String != "" {
				t, _ := time.Parse(time.RFC3339, endStr.String)
				b.EndAt = &t
			}
			list = append(list, b)
		}
	}
	return list, nil
}

func (r *SQLiteRepository) DeleteBroadcast(id string) error {
	_, err := r.db.Exec(`DELETE FROM broadcasts WHERE id = ?`, id)
	if err == nil {
		_, _ = r.db.Exec(`DELETE FROM broadcast_logs WHERE broadcast_id = ?`, id)
	}
	return err
}

func (r *SQLiteRepository) UpdateBroadcastStatus(id, status string) error {
	_, err := r.db.Exec(`
        UPDATE broadcasts SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
    `, status, id)
	return err
}

func (r *SQLiteRepository) LogBroadcastView(log BroadcastLog) error {
	nowStr := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(`
        INSERT INTO broadcast_logs (broadcast_id, agent_id, user_identifier, viewed_at, clicked)
        VALUES (?, ?, ?, ?, ?)
        ON CONFLICT(broadcast_id, agent_id, user_identifier) DO UPDATE SET
            viewed_at=excluded.viewed_at,
            clicked=CASE WHEN excluded.clicked > 0 THEN excluded.clicked ELSE broadcast_logs.clicked END
    `, log.BroadcastID, log.AgentID, log.UserIdentifier, nowStr, log.Clicked)
	return err
}

func (r *SQLiteRepository) GetBroadcastStats(broadcastID string) (impressions int, clicks int, err error) {
	err = r.db.QueryRow(`
        SELECT COUNT(*), COALESCE(SUM(clicked), 0)
        FROM broadcast_logs WHERE broadcast_id = ?
    `, broadcastID).Scan(&impressions, &clicks)
	return
}

func (r *SQLiteRepository) GetActiveBroadcastsForAgent(subdomain string) ([]Broadcast, error) {
	all, err := r.ListBroadcasts()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var matched []Broadcast
	for _, b := range all {
		if b.Status != "active" {
			continue
		}
		if b.StartAt != nil && now.Before(*b.StartAt) {
			continue
		}
		if b.EndAt != nil && now.After(*b.EndAt) {
			continue
		}
		if b.TargetAgents == "ALL" || b.TargetAgents == "" {
			matched = append(matched, b)
			continue
		}
		// Check if subdomain is in comma-separated list
		parts := strings.Split(b.TargetAgents, ",")
		for _, p := range parts {
			if strings.TrimSpace(p) == subdomain {
				matched = append(matched, b)
				break
			}
		}
	}
	return matched, nil
}

func (r *SQLiteRepository) GetAgentLicenseInfo(subdomain string) (*AgentLicenseInfo, error) {
	row := r.db.QueryRow(`
		SELECT s.subdomain, COALESCE(l.id, ''), COALESCE(l.status, 'unlicensed'), l.expires_at, l.updated_at
		FROM subdomains s
		LEFT JOIN licenses l ON s.license_id = l.id
		WHERE LOWER(s.subdomain) = LOWER(?)
	`, subdomain)

	var info AgentLicenseInfo
	var expiresAtStr, updatedAtStr sql.NullString
	err := row.Scan(&info.Subdomain, &info.LicenseID, &info.Status, &expiresAtStr, &updatedAtStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return &AgentLicenseInfo{
				Subdomain:     subdomain,
				Status:        "unlicensed",
				IsExpired:     true,
				DaysRemaining: 0,
			}, nil
		}
		return nil, err
	}

	now := time.Now().UTC()
	if expiresAtStr.Valid && expiresAtStr.String != "" {
		if exp, err := time.Parse(time.RFC3339, expiresAtStr.String); err == nil {
			info.ExpiresAt = &exp
			info.ExpiresAtStr = exp.Format("2006-01-02 15:04:05")
			if now.After(exp) {
				info.IsExpired = true
				info.DaysRemaining = 0
				if info.Status == "active" {
					info.Status = "expired"
				}
			} else {
				info.IsExpired = false
				diff := exp.Sub(now)
				info.DaysRemaining = int(diff.Hours() / 24)
				if info.DaysRemaining == 0 && diff.Seconds() > 0 {
					info.DaysRemaining = 1
				}
			}
		}
	} else if info.Status == "active" {
		// If active without expiry, default 30 days
		info.IsExpired = false
		info.DaysRemaining = 30
	} else {
		info.IsExpired = true
		info.DaysRemaining = 0
	}

	return &info, nil
}

func (r *SQLiteRepository) ActivateAgentLicense(subdomain string, days int) (*AgentLicenseInfo, error) {
	if days <= 0 {
		days = 30
	}

	info, err := r.GetAgentLicenseInfo(subdomain)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var newExp time.Time

	// If currently active and expires in the future, add days to current expires_at
	if info.ExpiresAt != nil && info.ExpiresAt.After(now) && info.Status == "active" {
		newExp = info.ExpiresAt.Add(time.Duration(days) * 24 * time.Hour)
	} else {
		// If expired, suspended, or no exp, start from now + days
		newExp = now.Add(time.Duration(days) * 24 * time.Hour)
	}

	newExpStr := newExp.Format(time.RFC3339)
	nowStr := now.Format(time.RFC3339)

	// If no license exists for this subdomain, create one
	if info.LicenseID == "" || info.Status == "unlicensed" {
		row := r.db.QueryRow(`SELECT customer_id FROM subdomains WHERE LOWER(subdomain) = LOWER(?)`, subdomain)
		var customerID string
		if err := row.Scan(&customerID); err != nil || customerID == "" {
			customerID = fmt.Sprintf("cust-%d", now.UnixNano())
			_, _ = r.db.Exec(`INSERT INTO customers (id, name, status, created_at, updated_at) VALUES (?, ?, 'active', ?, ?)`, customerID, subdomain, nowStr, nowStr)
		}

		licID := fmt.Sprintf("lic-%d", now.UnixNano())
		licKey := fmt.Sprintf("KEY-%s-%d", strings.ToUpper(subdomain), now.Unix())
		_, err = r.db.Exec(`
			INSERT INTO licenses (id, customer_id, license_key, status, issued_at, expires_at, created_at, updated_at)
			VALUES (?, ?, ?, 'active', ?, ?, ?, ?)
		`, licID, customerID, licKey, nowStr, newExpStr, nowStr, nowStr)
		if err != nil {
			return nil, err
		}

		_, err = r.db.Exec(`UPDATE subdomains SET license_id = ?, status = 'active', updated_at = ? WHERE LOWER(subdomain) = LOWER(?)`, licID, nowStr, subdomain)
		if err != nil {
			return nil, err
		}
	} else {
		_, err = r.db.Exec(`
			UPDATE licenses
			SET status = 'active', expires_at = ?, updated_at = ?
			WHERE id = ?
		`, newExpStr, nowStr, info.LicenseID)
		if err != nil {
			return nil, err
		}

		_, _ = r.db.Exec(`UPDATE subdomains SET status = 'active', updated_at = ? WHERE LOWER(subdomain) = LOWER(?)`, nowStr, subdomain)
	}

	return r.GetAgentLicenseInfo(subdomain)
}

func (r *SQLiteRepository) SuspendAgentLicense(subdomain string) error {
	nowStr := time.Now().UTC().Format(time.RFC3339)
	info, err := r.GetAgentLicenseInfo(subdomain)
	if err != nil {
		return err
	}
	if info.LicenseID != "" {
		_, err = r.db.Exec(`UPDATE licenses SET status = 'suspended', updated_at = ? WHERE id = ?`, nowStr, info.LicenseID)
		if err != nil {
			return err
		}
	}
	_, err = r.db.Exec(`UPDATE subdomains SET status = 'suspended', updated_at = ? WHERE LOWER(subdomain) = LOWER(?)`, nowStr, subdomain)
	return err
}

func (r *SQLiteRepository) ResumeAgentLicense(subdomain string) error {
	info, err := r.GetAgentLicenseInfo(subdomain)
	if err != nil {
		return err
	}
	if info.IsExpired || info.ExpiresAt == nil {
		_, err = r.ActivateAgentLicense(subdomain, 30)
		return err
	}
	nowStr := time.Now().UTC().Format(time.RFC3339)
	if info.LicenseID != "" {
		_, err = r.db.Exec(`UPDATE licenses SET status = 'active', updated_at = ? WHERE id = ?`, nowStr, info.LicenseID)
		if err != nil {
			return err
		}
	}
	_, err = r.db.Exec(`UPDATE subdomains SET status = 'active', updated_at = ? WHERE LOWER(subdomain) = LOWER(?)`, nowStr, subdomain)
	return err
}

func (r *SQLiteRepository) GetSubdomainLicensesMap() (map[string]AgentLicenseInfo, error) {
	rows, err := r.db.Query(`
		SELECT s.subdomain, COALESCE(l.id, ''), COALESCE(l.status, 'unlicensed'), l.expires_at, l.updated_at
		FROM subdomains s
		LEFT JOIN licenses l ON s.license_id = l.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now().UTC()
	result := make(map[string]AgentLicenseInfo)
	for rows.Next() {
		var sub string
		var licID, status, expiresAtStr, updatedAtStr sql.NullString
		if err := rows.Scan(&sub, &licID, &status, &expiresAtStr, &updatedAtStr); err == nil {
			info := AgentLicenseInfo{
				Subdomain: sub,
				LicenseID: licID.String,
				Status:    status.String,
			}
			if info.Status == "" {
				info.Status = "unlicensed"
			}
			if expiresAtStr.Valid && expiresAtStr.String != "" {
				if exp, err := time.Parse(time.RFC3339, expiresAtStr.String); err == nil {
					info.ExpiresAt = &exp
					info.ExpiresAtStr = exp.Format("2006-01-02 15:04:05")
					if now.After(exp) {
						info.IsExpired = true
						info.DaysRemaining = 0
						if info.Status == "active" {
							info.Status = "expired"
						}
					} else {
						info.IsExpired = false
						diff := exp.Sub(now)
						info.DaysRemaining = int(diff.Hours() / 24)
						if info.DaysRemaining == 0 && diff.Seconds() > 0 {
							info.DaysRemaining = 1
						}
					}
				}
			} else if info.Status == "active" {
				info.IsExpired = false
				info.DaysRemaining = 30
			} else {
				info.IsExpired = true
				info.DaysRemaining = 0
			}
			result[strings.ToLower(sub)] = info
		}
	}
	return result, nil
}

