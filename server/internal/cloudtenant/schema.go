package cloudtenant

import (
	"database/sql"
	"fmt"
	"log"

	"golang.org/x/crypto/bcrypt"
)

func EnsureTenantSchema(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("nil database handle")
	}

	queries := []string{
		`CREATE TABLE IF NOT EXISTS radcheck (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL DEFAULT '',
			attribute TEXT NOT NULL DEFAULT '',
			op TEXT NOT NULL DEFAULT '==',
			value TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE INDEX IF NOT EXISTS idx_radcheck_username ON radcheck (username);`,

		`CREATE TABLE IF NOT EXISTS radreply (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL DEFAULT '',
			attribute TEXT NOT NULL DEFAULT '',
			op TEXT NOT NULL DEFAULT '=',
			value TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE INDEX IF NOT EXISTS idx_radreply_username ON radreply (username);`,

		`CREATE TABLE IF NOT EXISTS radgroupcheck (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			groupname TEXT NOT NULL DEFAULT '',
			attribute TEXT NOT NULL DEFAULT '',
			op TEXT NOT NULL DEFAULT '==',
			value TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE INDEX IF NOT EXISTS idx_radgroupcheck_groupname ON radgroupcheck (groupname);`,

		`CREATE TABLE IF NOT EXISTS radgroupreply (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			groupname TEXT NOT NULL DEFAULT '',
			attribute TEXT NOT NULL DEFAULT '',
			op TEXT NOT NULL DEFAULT '=',
			value TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE INDEX IF NOT EXISTS idx_radgroupreply_groupname ON radgroupreply (groupname);`,

		`CREATE TABLE IF NOT EXISTS radusergroup (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL DEFAULT '',
			groupname TEXT NOT NULL DEFAULT '',
			priority INTEGER NOT NULL DEFAULT 1
		);`,
		`CREATE INDEX IF NOT EXISTS idx_radusergroup_username ON radusergroup (username);`,

		`CREATE TABLE IF NOT EXISTS radacct (
			radacctid INTEGER PRIMARY KEY AUTOINCREMENT,
			acctsessionid TEXT NOT NULL DEFAULT '',
			acctuniqueid TEXT NOT NULL DEFAULT '',
			username TEXT NOT NULL DEFAULT '',
			groupname TEXT NOT NULL DEFAULT '',
			realm TEXT DEFAULT '',
			nasipaddress TEXT NOT NULL DEFAULT '',
			nasportid TEXT DEFAULT '',
			nasporttype TEXT DEFAULT '',
			acctstarttime DATETIME NULL DEFAULT NULL,
			acctupdatetime DATETIME NULL DEFAULT NULL,
			acctstoptime DATETIME NULL DEFAULT NULL,
			acctinterval INTEGER DEFAULT NULL,
			acctsessiontime INTEGER DEFAULT NULL,
			acctauthentic TEXT DEFAULT '',
			connectinfo_start TEXT DEFAULT '',
			connectinfo_stop TEXT DEFAULT '',
			acctinputoctets BIGINT DEFAULT NULL,
			acctoutputoctets BIGINT DEFAULT NULL,
			calledstationid TEXT DEFAULT '',
			callingstationid TEXT DEFAULT '',
			acctterminatecause TEXT DEFAULT '',
			servicetype TEXT DEFAULT '',
			framedprotocol TEXT DEFAULT '',
			framedipaddress TEXT DEFAULT ''
		);`,
		`CREATE INDEX IF NOT EXISTS idx_radacct_username ON radacct (username);`,
		`CREATE INDEX IF NOT EXISTS idx_radacct_active ON radacct (acctstoptime, username);`,

		`CREATE TABLE IF NOT EXISTS radpostauth (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL DEFAULT '',
			pass TEXT NOT NULL DEFAULT '',
			reply TEXT NOT NULL DEFAULT '',
			authdate TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS nas (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			nasname TEXT NOT NULL,
			shortname TEXT,
			type TEXT DEFAULT 'other',
			ports INTEGER,
			secret TEXT NOT NULL,
			server TEXT,
			community TEXT,
			description TEXT
		);`,

		`CREATE TABLE IF NOT EXISTS radius_profile_meta (
			groupname TEXT PRIMARY KEY,
			validity_days INTEGER NOT NULL DEFAULT 0,
			price REAL NOT NULL DEFAULT 0,
			admin_id INTEGER DEFAULT NULL,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS radius_user_meta (
			username TEXT PRIMARY KEY,
			expiration_unix INTEGER DEFAULT NULL,
			renewal_enabled INTEGER NOT NULL DEFAULT 0,
			renewal_profile TEXT NOT NULL DEFAULT '',
			full_name TEXT NOT NULL DEFAULT '',
			phone TEXT NOT NULL DEFAULT '',
			balance REAL NOT NULL DEFAULT 0,
			enabled INTEGER NOT NULL DEFAULT 1,
			admin_id INTEGER DEFAULT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_reminder_at TIMESTAMP NULL DEFAULT NULL,
			reminder_sent_for INTEGER DEFAULT 0
		);`,

		`CREATE TABLE IF NOT EXISTS radius_user_transactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL,
			transaction_type TEXT NOT NULL DEFAULT '',
			amount REAL NOT NULL DEFAULT 0,
			notes TEXT NOT NULL DEFAULT '',
			admin_id INTEGER DEFAULT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS radius_whatsapp_config (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			admin_id INTEGER NOT NULL UNIQUE,
			enabled INTEGER NOT NULL DEFAULT 0,
			phone_number TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			reminder_enabled INTEGER NOT NULL DEFAULT 0,
			reminder_hours INTEGER NOT NULL DEFAULT 24
		);`,

		`CREATE TABLE IF NOT EXISTS radius_vouchers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id TEXT NOT NULL DEFAULT '',
			code TEXT NOT NULL UNIQUE,
			profile_name TEXT NOT NULL,
			validity_days INTEGER NOT NULL,
			price REAL NOT NULL DEFAULT 0,
			created_by INTEGER NOT NULL,
			used_by TEXT DEFAULT NULL,
			used_at TIMESTAMP NULL DEFAULT NULL,
			is_used INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_radius_vouchers_batch ON radius_vouchers(batch_id);`,

		`CREATE TABLE IF NOT EXISTS radius_admins (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'agent',
			name TEXT,
			phone TEXT,
			balance REAL NOT NULL DEFAULT 0.0,
			price_per_user REAL NOT NULL DEFAULT 0.0,
			is_active INTEGER NOT NULL DEFAULT 1,
			permissions TEXT DEFAULT '[]',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS radius_audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			admin_id INTEGER,
			action TEXT NOT NULL,
			target TEXT,
			details TEXT,
			ip TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			log.Printf("[cloudtenant] Warning executing schema query: %v", err)
		}
	}

	// Seed default profile if none exists
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM radgroupreply WHERE groupname = '10M'").Scan(&count)
	if count == 0 {
		_, _ = db.Exec("INSERT INTO radgroupreply (groupname, attribute, op, value) VALUES ('10M', 'Mikrotik-Rate-Limit', ':=', '10M/10M')")
		_, _ = db.Exec("INSERT OR IGNORE INTO radius_profile_meta (groupname, validity_days, price) VALUES ('10M', 30, 25000)")
	}

	// Seed default admin if none exists
	var adminCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM radius_admins WHERE username = 'admin'").Scan(&adminCount)
	if adminCount == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		_, _ = db.Exec(`
			INSERT INTO radius_admins (username, password, role, name, is_active)
			VALUES ('admin', ?, 'superadmin', 'مدير النظام', 1)
		`, string(hash))
	}

	return nil
}
