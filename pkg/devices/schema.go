package devices

import (
	"database/sql"
	"fmt"
	"log"
)

// EnsureSchema creates all 11 network device tables and initializes seed data
func EnsureSchema(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database instance is nil")
	}

	queries := []string{
		// 1. Vendors table
		`CREATE TABLE IF NOT EXISTS vendors (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			icon TEXT NOT NULL DEFAULT '',
			website TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_vendors_slug ON vendors(slug);`,

		// 2. Device Types table
		`CREATE TABLE IF NOT EXISTS device_types (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			icon TEXT NOT NULL DEFAULT '',
			description TEXT DEFAULT '',
			default_profile TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_device_types_slug ON device_types(slug);`,

		// 3. Device Models table
		`CREATE TABLE IF NOT EXISTS device_models (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			vendor_id INTEGER NOT NULL REFERENCES vendors(id) ON DELETE CASCADE,
			model_name TEXT NOT NULL,
			default_type_id INTEGER REFERENCES device_types(id) ON DELETE SET NULL,
			capabilities_json TEXT DEFAULT '{}',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(vendor_id, model_name)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_device_models_vendor ON device_models(vendor_id);`,

		// 4. Devices table
		`CREATE TABLE IF NOT EXISTS devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			vendor_id INTEGER NOT NULL REFERENCES vendors(id),
			type_id INTEGER NOT NULL REFERENCES device_types(id),
			model_id INTEGER REFERENCES device_models(id),
			name TEXT NOT NULL,
			ip TEXT NOT NULL,
			port INTEGER NOT NULL DEFAULT 8728,
			status TEXT NOT NULL DEFAULT 'unknown',
			last_seen DATETIME,
			last_error TEXT DEFAULT '',
			poll_interval_sec INTEGER NOT NULL DEFAULT 30,
			is_monitored INTEGER NOT NULL DEFAULT 1,
			down_since DATETIME,
			up_since DATETIME,
			downtime_duration_sec INTEGER DEFAULT 0,
			os_version TEXT DEFAULT '',
			serial_number TEXT DEFAULT '',
			architecture TEXT DEFAULT '',
			board_name TEXT DEFAULT '',
			uptime_seconds INTEGER DEFAULT 0,
			cpu_load INTEGER DEFAULT 0,
			memory_used INTEGER DEFAULT 0,
			memory_total INTEGER DEFAULT 0,
			storage_used INTEGER DEFAULT 0,
			storage_total INTEGER DEFAULT 0,
			temperature REAL DEFAULT 0,
			voltage REAL DEFAULT 0,
			location TEXT DEFAULT '',
			notes TEXT DEFAULT '',
			metadata_json TEXT DEFAULT '{}',
			admin_id INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_devices_ip ON devices(ip);`,
		`CREATE INDEX IF NOT EXISTS idx_devices_status ON devices(status);`,
		`CREATE INDEX IF NOT EXISTS idx_devices_type ON devices(type_id);`,
		`CREATE INDEX IF NOT EXISTS idx_devices_vendor ON devices(vendor_id);`,

		// 5. Device Credentials table
		`CREATE TABLE IF NOT EXISTS device_credentials (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL UNIQUE REFERENCES devices(id) ON DELETE CASCADE,
			username TEXT NOT NULL,
			password_encrypted TEXT NOT NULL,
			auth_type TEXT NOT NULL DEFAULT 'api',
			community TEXT DEFAULT '',
			api_version TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_device_credentials_dev ON device_credentials(device_id);`,

		// 6. Device Interfaces table
		`CREATE TABLE IF NOT EXISTS device_interfaces (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
			if_index INTEGER NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'ether',
			mac_address TEXT DEFAULT '',
			status TEXT NOT NULL DEFAULT 'unknown',
			speed TEXT DEFAULT '',
			duplex TEXT DEFAULT '',
			mtu INTEGER DEFAULT 1500,
			vlan_id INTEGER DEFAULT 0,
			is_poe INTEGER NOT NULL DEFAULT 0,
			is_sfp INTEGER NOT NULL DEFAULT 0,
			poe_status TEXT DEFAULT '',
			poe_power_watt REAL DEFAULT 0,
			sfp_wavelength TEXT DEFAULT '',
			sfp_temp REAL DEFAULT 0,
			sfp_tx_power_dbm REAL DEFAULT 0,
			sfp_rx_power_dbm REAL DEFAULT 0,
			rx_bytes INTEGER DEFAULT 0,
			tx_bytes INTEGER DEFAULT 0,
			rx_packets INTEGER DEFAULT 0,
			tx_packets INTEGER DEFAULT 0,
			rx_errors INTEGER DEFAULT 0,
			tx_errors INTEGER DEFAULT 0,
			rx_drops INTEGER DEFAULT 0,
			tx_drops INTEGER DEFAULT 0,
			sfp_info_json TEXT DEFAULT '{}',
			poe_info_json TEXT DEFAULT '{}',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(device_id, name)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_device_interfaces_dev ON device_interfaces(device_id);`,

		// 7. Device Metrics History table
		`CREATE TABLE IF NOT EXISTS device_metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
			cpu_load INTEGER DEFAULT 0,
			memory_used INTEGER DEFAULT 0,
			memory_total INTEGER DEFAULT 0,
			storage_used INTEGER DEFAULT 0,
			storage_total INTEGER DEFAULT 0,
			temperature REAL DEFAULT 0,
			voltage REAL DEFAULT 0,
			uptime_seconds INTEGER DEFAULT 0,
			rx_bytes INTEGER DEFAULT 0,
			tx_bytes INTEGER DEFAULT 0,
			rx_packets INTEGER DEFAULT 0,
			tx_packets INTEGER DEFAULT 0,
			rx_errors INTEGER DEFAULT 0,
			tx_errors INTEGER DEFAULT 0,
			rx_drops INTEGER DEFAULT 0,
			tx_drops INTEGER DEFAULT 0,
			recorded_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_device_metrics_dev_time ON device_metrics(device_id, recorded_at);`,

		// 8. Device Wireless Radio table (PtP Link & Sector AP)
		`CREATE TABLE IF NOT EXISTS device_wireless (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL UNIQUE REFERENCES devices(id) ON DELETE CASCADE,
			interface_name TEXT NOT NULL,
			mode TEXT NOT NULL DEFAULT 'ap-bridge',
			ssid TEXT DEFAULT '',
			frequency INTEGER DEFAULT 0,
			channel_width TEXT DEFAULT '',
			noise_floor INTEGER DEFAULT 0,
			tx_power INTEGER DEFAULT 0,
			ccq INTEGER DEFAULT 0,
			signal_strength INTEGER DEFAULT 0,
			snr INTEGER DEFAULT 0,
			tx_rate TEXT DEFAULT '',
			rx_rate TEXT DEFAULT '',
			distance_km REAL DEFAULT 0,
			mcs TEXT DEFAULT '',
			airtime_usage REAL DEFAULT 0,
			remote_mac TEXT DEFAULT '',
			remote_device_info TEXT DEFAULT '',
			connected_clients INTEGER DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_device_wireless_dev ON device_wireless(device_id);`,

		// 9. Device Wireless Clients table (Sector AP Station Clients)
		`CREATE TABLE IF NOT EXISTS device_wireless_clients (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
			wireless_id INTEGER REFERENCES device_wireless(id) ON DELETE CASCADE,
			mac_address TEXT NOT NULL,
			ip_address TEXT DEFAULT '',
			hostname TEXT DEFAULT '',
			signal INTEGER DEFAULT 0,
			noise INTEGER DEFAULT 0,
			snr INTEGER DEFAULT 0,
			tx_rate TEXT DEFAULT '',
			rx_rate TEXT DEFAULT '',
			ccq INTEGER DEFAULT 0,
			uptime_seconds INTEGER DEFAULT 0,
			rx_bytes INTEGER DEFAULT 0,
			tx_bytes INTEGER DEFAULT 0,
			status TEXT DEFAULT 'connected',
			last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(device_id, mac_address)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_wireless_clients_dev ON device_wireless_clients(device_id);`,
		`CREATE INDEX IF NOT EXISTS idx_wireless_clients_mac ON device_wireless_clients(mac_address);`,

		// 10. Device Events table
		`CREATE TABLE IF NOT EXISTS device_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
			event_type TEXT NOT NULL,
			severity TEXT NOT NULL DEFAULT 'info',
			message TEXT NOT NULL,
			event_data_json TEXT DEFAULT '{}',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_device_events_dev ON device_events(device_id, created_at);`,

		// 11. Device Alerts table
		`CREATE TABLE IF NOT EXISTS device_alerts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
			alert_type TEXT NOT NULL,
			message TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			triggered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			resolved_at DATETIME
		);`,
		`CREATE INDEX IF NOT EXISTS idx_device_alerts_dev_status ON device_alerts(device_id, status);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("execute schema query: %w\nQuery: %s", err, q)
		}
	}

	// Seed default vendors
	seedVendors := []struct {
		slug, name, icon, website string
	}{
		{"mikrotik", "MikroTik", "fa-solid fa-microchip", "https://mikrotik.com"},
		{"ubiquiti", "Ubiquiti Networks", "fa-solid fa-wifi", "https://ui.com"},
		{"huawei", "Huawei", "fa-solid fa-tower-cell", "https://huawei.com"},
		{"cisco", "Cisco Systems", "fa-solid fa-network-wired", "https://cisco.com"},
		{"tplink", "TP-Link", "fa-solid fa-router", "https://tp-link.com"},
		{"zte", "ZTE Corporation", "fa-solid fa-satellite-dish", "https://zte.com.cn"},
		{"cambium", "Cambium Networks", "fa-solid fa-satellite-dish", "https://cambiumnetworks.com"},
		{"mimosa", "Mimosa Networks", "fa-solid fa-tower-broadcast", "https://mimosa.co"},
	}

	for _, v := range seedVendors {
		_, _ = db.Exec(`
			INSERT INTO vendors (slug, name, icon, website)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(slug) DO UPDATE SET name=excluded.name, icon=excluded.icon, website=excluded.website;
		`, v.slug, v.name, v.icon, v.website)
	}

	// Seed default device types
	seedTypes := []struct {
		slug, name, icon, desc, profile string
	}{
		{"switch", "Switch / محول شبكة", "fa-solid fa-diagram-project", "إدارة ومراقبة منافذ السويتش، السرعات، VLANs، و PoE", "switch"},
		{"link", "PtP Link / ربط نقطة لنقطة", "fa-solid fa-arrows-left-right-to-line", "مراقبة روابط الأبراج اللاسلكية، الإشارة، CCQ، ومعدلات النقل", "link"},
		{"sector", "Sector AP / سكتور بث", "fa-solid fa-tower-broadcast", "مراقبة سكترات البث، التردد، وقائمة المشتركين المتصلين الحية", "sector"},
		{"router", "Router / راوتر توجيه", "fa-solid fa-server", "مراقبة راوترات التوجيه وموارد المعالج والذاكرة والترافيك", "router"},
	}

	for _, t := range seedTypes {
		_, _ = db.Exec(`
			INSERT INTO device_types (slug, name, icon, description, default_profile)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(slug) DO UPDATE SET name=excluded.name, icon=excluded.icon, description=excluded.description, default_profile=excluded.default_profile;
		`, t.slug, t.name, t.icon, t.desc, t.profile)
	}

	log.Printf("[NetworkDevices] Database schema initialized with 11 tables and seed records.")
	return nil
}
