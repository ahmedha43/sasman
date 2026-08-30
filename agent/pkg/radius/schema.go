
package radius

import (
	"log"
)

func EnsureSchema() {
	if DB == nil {
		return
	}

	tables := []string{
		// FreeRADIUS Standard Tables
		`CREATE TABLE IF NOT EXISTS radcheck (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL DEFAULT '',
			attribute TEXT NOT NULL DEFAULT '',
			op TEXT NOT NULL DEFAULT '==',
			value TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_radcheck_username ON radcheck (username)`,

		`CREATE TABLE IF NOT EXISTS radreply (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL DEFAULT '',
			attribute TEXT NOT NULL DEFAULT '',
			op TEXT NOT NULL DEFAULT '=',
			value TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_radreply_username ON radreply (username)`,

		`CREATE TABLE IF NOT EXISTS radgroupcheck (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			groupname TEXT NOT NULL DEFAULT '',
			attribute TEXT NOT NULL DEFAULT '',
			op TEXT NOT NULL DEFAULT '==',
			value TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_radgroupcheck_groupname ON radgroupcheck (groupname)`,

		`CREATE TABLE IF NOT EXISTS radgroupreply (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			groupname TEXT NOT NULL DEFAULT '',
			attribute TEXT NOT NULL DEFAULT '',
			op TEXT NOT NULL DEFAULT '=',
			value TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_radgroupreply_groupname ON radgroupreply (groupname)`,

		`CREATE TABLE IF NOT EXISTS radusergroup (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL DEFAULT '',
			groupname TEXT NOT NULL DEFAULT '',
			priority INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE INDEX IF NOT EXISTS idx_radusergroup_username ON radusergroup (username)`,

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
			acctstarttime DATETIME DEFAULT NULL,
			acctupdatetime DATETIME DEFAULT NULL,
			acctstoptime DATETIME DEFAULT NULL,
			acctinterval INTEGER DEFAULT NULL,
			acctsessiontime INTEGER DEFAULT NULL,
			acctauthentic TEXT DEFAULT '',
			connectinfo_start TEXT DEFAULT '',
			connectinfo_stop TEXT DEFAULT '',
			acctinputoctets INTEGER DEFAULT NULL,
			acctoutputoctets INTEGER DEFAULT NULL,
			calledstationid TEXT DEFAULT '',
			callingstationid TEXT DEFAULT '',
			acctterminatecause TEXT DEFAULT '',
			servicetype TEXT DEFAULT '',
			framedprotocol TEXT DEFAULT '',
			framedipaddress TEXT DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_radacct_active ON radacct (username) WHERE acctstoptime IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_radacct_sessionid ON radacct (acctsessionid)`,

		`CREATE TABLE IF NOT EXISTS radpostauth (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL DEFAULT '',
			pass TEXT NOT NULL DEFAULT '',
			reply TEXT NOT NULL DEFAULT '',
			authdate DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS nas (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			nasname TEXT NOT NULL,
			shortname TEXT,
			type TEXT DEFAULT 'other',
			ports INTEGER,
			secret TEXT NOT NULL,
			server TEXT,
			community TEXT,
			description TEXT DEFAULT 'RADIUS Client',
			profile_nas_ip TEXT DEFAULT '',
			admin_id INTEGER DEFAULT NULL
		)`,

		// SASMAN Custom Tables
		`CREATE TABLE IF NOT EXISTS radius_profile_meta (
			groupname TEXT PRIMARY KEY,
			validity_days INTEGER NOT NULL DEFAULT 0,
			price REAL NOT NULL DEFAULT 0,
			agent_price REAL NOT NULL DEFAULT 0,
			admin_id INTEGER DEFAULT NULL,
			expired_pool TEXT DEFAULT '',
			expired_profile TEXT DEFAULT '',
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,

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
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_reminder_at DATETIME DEFAULT NULL,
			reminder_sent_for INTEGER DEFAULT 0
		)`,

		`CREATE TABLE IF NOT EXISTS radius_user_transactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL,
			transaction_type TEXT NOT NULL DEFAULT '',
			amount REAL NOT NULL DEFAULT 0,
			notes TEXT NOT NULL DEFAULT '',
			admin_id INTEGER DEFAULT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS radius_whatsapp_config (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			admin_id INTEGER NOT NULL UNIQUE,
			enabled INTEGER NOT NULL DEFAULT 0,
			phone_number TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			reminder_enabled INTEGER NOT NULL DEFAULT 0,
			reminder_hours INTEGER NOT NULL DEFAULT 24,
			device_jid TEXT NOT NULL DEFAULT ''
		)`,

		`CREATE TABLE IF NOT EXISTS radius_vouchers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id TEXT NOT NULL DEFAULT '',
			code TEXT NOT NULL UNIQUE,
			profile_name TEXT NOT NULL,
			validity_days INTEGER NOT NULL,
			price REAL NOT NULL DEFAULT 0,
			created_by INTEGER NOT NULL,
			used_by TEXT DEFAULT NULL,
			used_at DATETIME DEFAULT NULL,
			is_used INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS radius_message_templates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			_template_key TEXT NOT NULL UNIQUE,
			template_text TEXT NOT NULL,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS radius_admins (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			email TEXT NOT NULL DEFAULT '',
			role TEXT NOT NULL DEFAULT 'superadmin',
			parent_id INTEGER DEFAULT NULL,
			balance REAL NOT NULL DEFAULT 0,
			can_manage_profiles INTEGER NOT NULL DEFAULT 0,
			can_manage_nas INTEGER NOT NULL DEFAULT 0,
			can_create_users INTEGER NOT NULL DEFAULT 1,
			can_edit_users INTEGER NOT NULL DEFAULT 1,
			can_delete_users INTEGER NOT NULL DEFAULT 0,
			can_toggle_users INTEGER NOT NULL DEFAULT 1,
			can_disconnect_users INTEGER NOT NULL DEFAULT 1,
			can_renew_users INTEGER NOT NULL DEFAULT 1,
			can_generate_vouchers INTEGER NOT NULL DEFAULT 1,
			can_delete_vouchers INTEGER NOT NULL DEFAULT 0,
			can_print_vouchers INTEGER NOT NULL DEFAULT 1,
			can_manage_devices INTEGER NOT NULL DEFAULT 0,
			can_manage_transactions INTEGER NOT NULL DEFAULT 1,
			can_manage_subagents INTEGER NOT NULL DEFAULT 0,
			can_view_logs INTEGER NOT NULL DEFAULT 1,
			can_clear_logs INTEGER NOT NULL DEFAULT 0,
			can_manage_whatsapp INTEGER NOT NULL DEFAULT 0,
			can_manage_streams INTEGER NOT NULL DEFAULT 0,
			permissions TEXT NOT NULL DEFAULT '',
			plain_secret TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS radius_admin_sessions (
			token TEXT PRIMARY KEY,
			admin_id INTEGER NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL
		)`,

		`CREATE TABLE IF NOT EXISTS radius_system_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT '',
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS radius_streams (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			source TEXT NOT NULL,
			status TEXT DEFAULT 'inactive',
			local_relay INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS radius_stream_viewers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			stream_id TEXT,
			username TEXT,
			ip TEXT,
			connected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (stream_id) REFERENCES radius_streams(id)
		)`,

		`CREATE TABLE IF NOT EXISTS radius_admin_transactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			admin_id INTEGER NOT NULL,
			performed_by INTEGER NOT NULL,
			transaction_type TEXT NOT NULL,
			amount REAL NOT NULL,
			notes TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS radius_audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			admin_id INTEGER DEFAULT NULL,
			admin_username TEXT NOT NULL DEFAULT '',
			action_type TEXT NOT NULL,
			target TEXT NOT NULL DEFAULT '',
			details TEXT NOT NULL DEFAULT '',
			ip_address TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_created_at ON radius_audit_logs (created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_action_type ON radius_audit_logs (action_type)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_admin_id ON radius_audit_logs (admin_id)`,

		// RadSec & PKI Certificates Table
		`CREATE TABLE IF NOT EXISTS nas_certificates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			nas_id INTEGER,
			nas_name TEXT NOT NULL,
			common_name TEXT NOT NULL UNIQUE,
			serial_number TEXT NOT NULL,
			cert_pem TEXT NOT NULL,
			key_pem TEXT NOT NULL,
			ca_pem TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL,
			revoked INTEGER NOT NULL DEFAULT 0,
			revoked_at DATETIME DEFAULT NULL,
			FOREIGN KEY (nas_id) REFERENCES nas(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_nas_certs_common_name ON nas_certificates(common_name)`,
		`CREATE INDEX IF NOT EXISTS idx_nas_certs_nas_id ON nas_certificates(nas_id)`,
	}

	for _, stmt := range tables {
		if _, err := DB.Exec(stmt); err != nil {
			log.Printf("[schema] Error executing stmt: %v\nStmt: %s", err, stmt)
		}
	}

	// Insert Default Admin if no admins exist
	var count int
	_ = DB.QueryRow("SELECT COUNT(*) FROM radius_admins").Scan(&count)
	if count == 0 {
		// Default admin: admin / 12345678 (hash should be provided, but here I'll use a placeholder or let main.go handle it)
		// For now, let's just log it.
		log.Println("[schema] No admins found. System is ready for first-run initialization.")
	}

	log.Println("[schema] SQLite Schema verification completed.")
	
	// Migration for existing databases
	DB.Exec("ALTER TABLE radius_profile_meta ADD COLUMN agent_price REAL NOT NULL DEFAULT 0")
	DB.Exec("ALTER TABLE radius_admins ADD COLUMN can_manage_profiles INTEGER NOT NULL DEFAULT 0")
	DB.Exec("ALTER TABLE radius_admins ADD COLUMN can_manage_nas INTEGER NOT NULL DEFAULT 0")
	DB.Exec("ALTER TABLE radius_admins ADD COLUMN can_create_users INTEGER NOT NULL DEFAULT 1")
	DB.Exec("ALTER TABLE radius_admins ADD COLUMN can_edit_users INTEGER NOT NULL DEFAULT 1")
	DB.Exec("ALTER TABLE radius_admins ADD COLUMN can_delete_users INTEGER NOT NULL DEFAULT 0")
	DB.Exec("ALTER TABLE radius_admins ADD COLUMN can_toggle_users INTEGER NOT NULL DEFAULT 1")
	DB.Exec("ALTER TABLE radius_admins ADD COLUMN can_disconnect_users INTEGER NOT NULL DEFAULT 1")
	DB.Exec("ALTER TABLE radius_admins ADD COLUMN can_renew_users INTEGER NOT NULL DEFAULT 1")
	DB.Exec("ALTER TABLE radius_admins ADD COLUMN can_generate_vouchers INTEGER NOT NULL DEFAULT 1")
	DB.Exec("ALTER TABLE radius_admins ADD COLUMN can_delete_vouchers INTEGER NOT NULL DEFAULT 0")
	DB.Exec("ALTER TABLE radius_admins ADD COLUMN can_print_vouchers INTEGER NOT NULL DEFAULT 1")
	DB.Exec("ALTER TABLE radius_admins ADD COLUMN can_manage_devices INTEGER NOT NULL DEFAULT 0")
	DB.Exec("ALTER TABLE radius_admins ADD COLUMN can_manage_transactions INTEGER NOT NULL DEFAULT 1")
	DB.Exec("ALTER TABLE radius_admins ADD COLUMN can_manage_subagents INTEGER NOT NULL DEFAULT 0")
	DB.Exec("ALTER TABLE radius_admins ADD COLUMN can_view_logs INTEGER NOT NULL DEFAULT 1")
	DB.Exec("ALTER TABLE radius_admins ADD COLUMN can_clear_logs INTEGER NOT NULL DEFAULT 0")
	DB.Exec("ALTER TABLE radius_admins ADD COLUMN can_manage_whatsapp INTEGER NOT NULL DEFAULT 0")
	DB.Exec("ALTER TABLE radius_admins ADD COLUMN can_manage_streams INTEGER NOT NULL DEFAULT 0")
	DB.Exec("ALTER TABLE radius_admins ADD COLUMN permissions TEXT NOT NULL DEFAULT ''")
	DB.Exec("ALTER TABLE radius_profile_meta ADD COLUMN expired_pool TEXT DEFAULT ''")
	DB.Exec("ALTER TABLE radius_profile_meta ADD COLUMN expired_profile TEXT DEFAULT ''")
	
	// Whatsapp config migrations for older database dumps
	DB.Exec("ALTER TABLE radius_whatsapp_config ADD COLUMN admin_id INTEGER DEFAULT 1")
	DB.Exec("ALTER TABLE radius_whatsapp_config ADD COLUMN reminder_enabled INTEGER NOT NULL DEFAULT 0")
	DB.Exec("ALTER TABLE radius_whatsapp_config ADD COLUMN reminder_hours INTEGER NOT NULL DEFAULT 24")
	DB.Exec("ALTER TABLE radius_whatsapp_config ADD COLUMN device_jid TEXT NOT NULL DEFAULT ''")

	// Ensure admin_id uniqueness for ON CONFLICT to work
	DB.Exec("DELETE FROM radius_whatsapp_config WHERE id NOT IN (SELECT MAX(id) FROM radius_whatsapp_config GROUP BY admin_id)")
	DB.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_whatsapp_admin ON radius_whatsapp_config(admin_id)")
	DB.Exec("ALTER TABLE radius_streams ADD COLUMN local_relay INTEGER NOT NULL DEFAULT 0")
	DB.Exec("ALTER TABLE radius_admins ADD COLUMN plain_secret TEXT NOT NULL DEFAULT ''")
}
