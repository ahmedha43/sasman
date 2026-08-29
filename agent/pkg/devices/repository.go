package devices

import (
	"database/sql"
	"fmt"
	"time"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// ─── Vendors & Types ──────────────────────────────────────────────────────────

func (r *Repository) ListVendors() ([]Vendor, error) {
	rows, err := r.db.Query(`SELECT id, slug, name, icon, COALESCE(website, ''), created_at, updated_at FROM vendors ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Vendor
	for rows.Next() {
		var v Vendor
		if err := rows.Scan(&v.ID, &v.Slug, &v.Name, &v.Icon, &v.Website, &v.CreatedAt, &v.UpdatedAt); err == nil {
			list = append(list, v)
		}
	}
	return list, nil
}

func (r *Repository) ListDeviceTypes() ([]DeviceType, error) {
	rows, err := r.db.Query(`SELECT id, slug, name, icon, COALESCE(description, ''), COALESCE(default_profile, ''), created_at FROM device_types ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []DeviceType
	for rows.Next() {
		var t DeviceType
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.Icon, &t.Description, &t.DefaultProfile, &t.CreatedAt); err == nil {
			list = append(list, t)
		}
	}
	return list, nil
}

func (r *Repository) GetVendorBySlug(slug string) (*Vendor, error) {
	var v Vendor
	err := r.db.QueryRow(`SELECT id, slug, name, icon, COALESCE(website, ''), created_at, updated_at FROM vendors WHERE slug = ?`, slug).
		Scan(&v.ID, &v.Slug, &v.Name, &v.Icon, &v.Website, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *Repository) GetDeviceTypeBySlug(slug string) (*DeviceType, error) {
	var t DeviceType
	err := r.db.QueryRow(`SELECT id, slug, name, icon, COALESCE(description, ''), COALESCE(default_profile, ''), created_at FROM device_types WHERE slug = ?`, slug).
		Scan(&t.ID, &t.Slug, &t.Name, &t.Icon, &t.Description, &t.DefaultProfile, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ─── Devices CRUD ───────────────────────────────────────────────────────────

func (r *Repository) CreateDevice(d *Device, cred *DeviceCredential) (int64, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	pollInterval := d.PollIntervalSec
	if pollInterval <= 0 {
		pollInterval = 30
	}

	isMonitored := 1
	if !d.IsMonitored {
		isMonitored = 0
	}

	res, err := tx.Exec(`
		INSERT INTO devices (
			vendor_id, type_id, model_id, name, ip, port, status, poll_interval_sec, is_monitored,
			os_version, serial_number, architecture, board_name, location, notes, metadata_json, admin_id
		) VALUES (?, ?, ?, ?, ?, ?, 'unknown', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, d.VendorID, d.TypeID, d.ModelID, d.Name, d.IP, d.Port, pollInterval, isMonitored,
		d.OSVersion, d.SerialNumber, d.Architecture, d.BoardName, d.Location, d.Notes, d.MetadataJSON, d.AdminID)
	if err != nil {
		return 0, fmt.Errorf("insert device: %w", err)
	}

	deviceID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	if cred != nil {
		encryptedPass, err := EncryptPassword(cred.PasswordEncrypted)
		if err != nil {
			return 0, fmt.Errorf("encrypt password: %w", err)
		}

		authType := cred.AuthType
		if authType == "" {
			authType = "api"
		}

		_, err = tx.Exec(`
			INSERT INTO device_credentials (device_id, username, password_encrypted, auth_type, community, api_version)
			VALUES (?, ?, ?, ?, ?, ?)
		`, deviceID, cred.Username, encryptedPass, authType, cred.Community, cred.APIVersion)
		if err != nil {
			return 0, fmt.Errorf("insert credentials: %w", err)
		}
	}

	// Create initial event
	_, _ = tx.Exec(`
		INSERT INTO device_events (device_id, event_type, severity, message)
		VALUES (?, 'config_change', 'info', 'Device registered in SASMAN Network Devices module')
	`, deviceID)

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	d.ID = deviceID
	return deviceID, nil
}

func (r *Repository) UpdateDevice(d *Device) error {
	pollInterval := d.PollIntervalSec
	if pollInterval <= 0 {
		pollInterval = 30
	}

	isMonitored := 1
	if !d.IsMonitored {
		isMonitored = 0
	}

	_, err := r.db.Exec(`
		UPDATE devices SET
			vendor_id = ?, type_id = ?, model_id = ?, name = ?, ip = ?, port = ?,
			poll_interval_sec = ?, is_monitored = ?, location = ?, notes = ?, metadata_json = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, d.VendorID, d.TypeID, d.ModelID, d.Name, d.IP, d.Port,
		pollInterval, isMonitored, d.Location, d.Notes, d.MetadataJSON, d.ID)
	return err
}

func (r *Repository) UpdateDeviceCredentials(deviceID int64, username, password, authType string) error {
	encryptedPass, err := EncryptPassword(password)
	if err != nil {
		return fmt.Errorf("encrypt password: %w", err)
	}

	if authType == "" {
		authType = "api"
	}

	_, err = r.db.Exec(`
		INSERT INTO device_credentials (device_id, username, password_encrypted, auth_type, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(device_id) DO UPDATE SET
			username = excluded.username,
			password_encrypted = excluded.password_encrypted,
			auth_type = excluded.auth_type,
			updated_at = CURRENT_TIMESTAMP
	`, deviceID, username, encryptedPass, authType)
	return err
}

func (r *Repository) GetDevice(id int64) (*Device, error) {
	var d Device
	var lastSeen, downSince, upSince sql.NullTime
	var modelID sql.NullInt64
	var modelName sql.NullString

	err := r.db.QueryRow(`
		SELECT 
			d.id, d.vendor_id, v.slug, v.name,
			d.type_id, t.slug, t.name,
			d.model_id, dm.model_name,
			d.name, d.ip, d.port, d.status, d.last_seen, COALESCE(d.last_error, ''),
			d.poll_interval_sec, d.is_monitored, d.down_since, d.up_since, COALESCE(d.downtime_duration_sec, 0),
			COALESCE(d.os_version, ''), COALESCE(d.serial_number, ''), COALESCE(d.architecture, ''), COALESCE(d.board_name, ''),
			COALESCE(d.uptime_seconds, 0), COALESCE(d.cpu_load, 0), COALESCE(d.memory_used, 0), COALESCE(d.memory_total, 0),
			COALESCE(d.storage_used, 0), COALESCE(d.storage_total, 0), COALESCE(d.temperature, 0), COALESCE(d.voltage, 0),
			COALESCE(d.location, ''), COALESCE(d.notes, ''), COALESCE(d.metadata_json, '{}'),
			d.admin_id, d.created_at, d.updated_at,
			(SELECT COUNT(*) FROM device_interfaces WHERE device_id = d.id),
			(SELECT COUNT(*) FROM device_wireless_clients WHERE device_id = d.id)
		FROM devices d
		JOIN vendors v ON d.vendor_id = v.id
		JOIN device_types t ON d.type_id = t.id
		LEFT JOIN device_models dm ON d.model_id = dm.id
		WHERE d.id = ?
	`, id).Scan(
		&d.ID, &d.VendorID, &d.VendorSlug, &d.VendorName,
		&d.TypeID, &d.TypeSlug, &d.TypeName,
		&modelID, &modelName,
		&d.Name, &d.IP, &d.Port, &d.Status, &lastSeen, &d.LastError,
		&d.PollIntervalSec, &d.IsMonitored, &downSince, &upSince, &d.DowntimeDurationSec,
		&d.OSVersion, &d.SerialNumber, &d.Architecture, &d.BoardName,
		&d.UptimeSeconds, &d.CPULoad, &d.MemoryUsed, &d.MemoryTotal,
		&d.StorageUsed, &d.StorageTotal, &d.Temperature, &d.Voltage,
		&d.Location, &d.Notes, &d.MetadataJSON,
		&d.AdminID, &d.CreatedAt, &d.UpdatedAt,
		&d.InterfacesCount, &d.ClientsCount,
	)

	if err != nil {
		return nil, err
	}

	if lastSeen.Valid {
		d.LastSeen = &lastSeen.Time
	}
	if downSince.Valid {
		d.DownSince = &downSince.Time
	}
	if upSince.Valid {
		d.UpSince = &upSince.Time
	}
	if modelID.Valid {
		d.ModelID = modelID.Int64
	}
	if modelName.Valid {
		d.ModelName = modelName.String
	}

	// Fetch wireless info if available
	wInfo, _ := r.GetDeviceWireless(d.ID)
	d.WirelessInfo = wInfo

	return &d, nil
}

func (r *Repository) ListDevices(typeFilter, vendorFilter, statusFilter string) ([]Device, error) {
	query := `
		SELECT 
			d.id, d.vendor_id, v.slug, v.name,
			d.type_id, t.slug, t.name,
			d.model_id, dm.model_name,
			d.name, d.ip, d.port, d.status, d.last_seen, COALESCE(d.last_error, ''),
			d.poll_interval_sec, d.is_monitored, d.down_since, d.up_since, COALESCE(d.downtime_duration_sec, 0),
			COALESCE(d.os_version, ''), COALESCE(d.serial_number, ''), COALESCE(d.architecture, ''), COALESCE(d.board_name, ''),
			COALESCE(d.uptime_seconds, 0), COALESCE(d.cpu_load, 0), COALESCE(d.memory_used, 0), COALESCE(d.memory_total, 0),
			COALESCE(d.storage_used, 0), COALESCE(d.storage_total, 0), COALESCE(d.temperature, 0), COALESCE(d.voltage, 0),
			COALESCE(d.location, ''), COALESCE(d.notes, ''), COALESCE(d.metadata_json, '{}'),
			d.admin_id, d.created_at, d.updated_at,
			(SELECT COUNT(*) FROM device_interfaces WHERE device_id = d.id),
			(SELECT COUNT(*) FROM device_wireless_clients WHERE device_id = d.id)
		FROM devices d
		JOIN vendors v ON d.vendor_id = v.id
		JOIN device_types t ON d.type_id = t.id
		LEFT JOIN device_models dm ON d.model_id = dm.id
		WHERE 1=1
	`
	var args []any
	if typeFilter != "" && typeFilter != "all" {
		query += " AND t.slug = ?"
		args = append(args, typeFilter)
	}
	if vendorFilter != "" && vendorFilter != "all" {
		query += " AND v.slug = ?"
		args = append(args, vendorFilter)
	}
	if statusFilter != "" && statusFilter != "all" {
		query += " AND d.status = ?"
		args = append(args, statusFilter)
	}

	query += " ORDER BY d.id DESC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Device
	for rows.Next() {
		var d Device
		var lastSeen, downSince, upSince sql.NullTime
		var modelID sql.NullInt64
		var modelName sql.NullString

		err := rows.Scan(
			&d.ID, &d.VendorID, &d.VendorSlug, &d.VendorName,
			&d.TypeID, &d.TypeSlug, &d.TypeName,
			&modelID, &modelName,
			&d.Name, &d.IP, &d.Port, &d.Status, &lastSeen, &d.LastError,
			&d.PollIntervalSec, &d.IsMonitored, &downSince, &upSince, &d.DowntimeDurationSec,
			&d.OSVersion, &d.SerialNumber, &d.Architecture, &d.BoardName,
			&d.UptimeSeconds, &d.CPULoad, &d.MemoryUsed, &d.MemoryTotal,
			&d.StorageUsed, &d.StorageTotal, &d.Temperature, &d.Voltage,
			&d.Location, &d.Notes, &d.MetadataJSON,
			&d.AdminID, &d.CreatedAt, &d.UpdatedAt,
			&d.InterfacesCount, &d.ClientsCount,
		)
		if err == nil {
			if lastSeen.Valid {
				d.LastSeen = &lastSeen.Time
			}
			if downSince.Valid {
				d.DownSince = &downSince.Time
			}
			if upSince.Valid {
				d.UpSince = &upSince.Time
			}
			if modelID.Valid {
				d.ModelID = modelID.Int64
			}
			if modelName.Valid {
				d.ModelName = modelName.String
			}
			list = append(list, d)
		}
	}
	return list, nil
}

func (r *Repository) DeleteDevice(id int64) error {
	_, err := r.db.Exec(`DELETE FROM devices WHERE id = ?`, id)
	return err
}

func (r *Repository) GetDeviceCredentials(deviceID int64) (*TargetConfig, error) {
	var target TargetConfig
	var passEnc string
	err := r.db.QueryRow(`
		SELECT d.ip, d.port, c.username, c.password_encrypted, c.auth_type
		FROM devices d
		JOIN device_credentials c ON d.id = c.device_id
		WHERE d.id = ?
	`, deviceID).Scan(&target.IP, &target.Port, &target.Username, &passEnc, &target.AuthType)
	if err != nil {
		return nil, err
	}

	plainPass, err := DecryptPassword(passEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt device password: %w", err)
	}
	target.Password = plainPass
	target.Timeout = 10 * time.Second
	return &target, nil
}

// ─── Status & State Machine Updates ──────────────────────────────────────────

func (r *Repository) UpdateDeviceOnlineStatus(deviceID int64, isOnline bool, isWarning bool, errMsg string, now time.Time) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var currentStatus string
	var downSince sql.NullTime
	var downtimeDur int64

	_ = tx.QueryRow(`SELECT status, down_since, downtime_duration_sec FROM devices WHERE id = ?`, deviceID).
		Scan(&currentStatus, &downSince, &downtimeDur)

	if isOnline {
		newStatus := "online"
		if isWarning {
			newStatus = "warning"
		}

		if currentStatus == "offline" && downSince.Valid {
			// Recovered from offline
			outageDuration := int64(now.Sub(downSince.Time).Seconds())
			downtimeDur += outageDuration

			_, _ = tx.Exec(`
				UPDATE devices SET
					status = ?, last_seen = ?, last_error = ?,
					up_since = ?, down_since = NULL, downtime_duration_sec = ?,
					updated_at = ?
				WHERE id = ?
			`, newStatus, now, errMsg, now, downtimeDur, now, deviceID)

			_, _ = tx.Exec(`
				INSERT INTO device_events (device_id, event_type, severity, message, created_at)
				VALUES (?, 'online', 'info', ?, ?)
			`, deviceID, fmt.Sprintf("Device recovered and is back online (Outage duration: %ds)", outageDuration), now)

			// Resolve any active offline alerts
			_, _ = tx.Exec(`
				UPDATE device_alerts SET status = 'resolved', resolved_at = ?
				WHERE device_id = ? AND alert_type = 'offline' AND status = 'active'
			`, now, deviceID)

		} else {
			_, _ = tx.Exec(`
				UPDATE devices SET
					status = ?, last_seen = ?, last_error = ?,
					updated_at = ?
				WHERE id = ?
			`, newStatus, now, errMsg, now, deviceID)
		}

	} else {
		// Device is Offline
		if currentStatus != "offline" {
			_, _ = tx.Exec(`
				UPDATE devices SET
					status = 'offline', last_error = ?,
					down_since = ?, up_since = NULL,
					updated_at = ?
				WHERE id = ?
			`, errMsg, now, now, deviceID)

			_, _ = tx.Exec(`
				INSERT INTO device_events (device_id, event_type, severity, message, created_at)
				VALUES (?, 'offline', 'critical', ?, ?)
			`, deviceID, fmt.Sprintf("Device connection lost: %s", errMsg), now)

			_, _ = tx.Exec(`
				INSERT INTO device_alerts (device_id, alert_type, message, status, triggered_at)
				VALUES (?, 'offline', ?, 'active', ?)
			`, deviceID, fmt.Sprintf("Device is unreachable at %s (%s)", errMsg, now.Format("15:04:05")), now)
		} else {
			_, _ = tx.Exec(`
				UPDATE devices SET last_error = ?, updated_at = ? WHERE id = ?
			`, errMsg, now, deviceID)
		}
	}

	return tx.Commit()
}

func (r *Repository) UpdateDeviceSystemInfo(deviceID int64, d *Device) error {
	_, err := r.db.Exec(`
		UPDATE devices SET
			os_version = ?, serial_number = ?, architecture = ?, board_name = ?,
			uptime_seconds = ?, cpu_load = ?, memory_used = ?, memory_total = ?,
			storage_used = ?, storage_total = ?, temperature = ?, voltage = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, d.OSVersion, d.SerialNumber, d.Architecture, d.BoardName,
		d.UptimeSeconds, d.CPULoad, d.MemoryUsed, d.MemoryTotal,
		d.StorageUsed, d.StorageTotal, d.Temperature, d.Voltage, deviceID)
	return err
}

// ─── Interfaces & Metrics ───────────────────────────────────────────────────

func (r *Repository) SaveInterfaces(deviceID int64, ifaces []DeviceInterface) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now()
	for _, iface := range ifaces {
		isPoE := 0
		if iface.IsPoE {
			isPoE = 1
		}
		isSFP := 0
		if iface.IsSFP {
			isSFP = 1
		}

		_, err := tx.Exec(`
			INSERT INTO device_interfaces (
				device_id, if_index, name, type, mac_address, status, speed, duplex, mtu, vlan_id,
				is_poe, is_sfp, poe_status, poe_power_watt, sfp_wavelength, sfp_temp, sfp_tx_power_dbm, sfp_rx_power_dbm,
				rx_bytes, tx_bytes, rx_packets, tx_packets, rx_errors, tx_errors, rx_drops, tx_drops,
				sfp_info_json, poe_info_json, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(device_id, name) DO UPDATE SET
				if_index = excluded.if_index,
				type = excluded.type,
				mac_address = excluded.mac_address,
				status = excluded.status,
				speed = excluded.speed,
				duplex = excluded.duplex,
				mtu = excluded.mtu,
				vlan_id = excluded.vlan_id,
				is_poe = excluded.is_poe,
				is_sfp = excluded.is_sfp,
				poe_status = excluded.poe_status,
				poe_power_watt = excluded.poe_power_watt,
				sfp_wavelength = excluded.sfp_wavelength,
				sfp_temp = excluded.sfp_temp,
				sfp_tx_power_dbm = excluded.sfp_tx_power_dbm,
				sfp_rx_power_dbm = excluded.sfp_rx_power_dbm,
				rx_bytes = excluded.rx_bytes,
				tx_bytes = excluded.tx_bytes,
				rx_packets = excluded.rx_packets,
				tx_packets = excluded.tx_packets,
				rx_errors = excluded.rx_errors,
				tx_errors = excluded.tx_errors,
				rx_drops = excluded.rx_drops,
				tx_drops = excluded.tx_drops,
				sfp_info_json = excluded.sfp_info_json,
				poe_info_json = excluded.poe_info_json,
				updated_at = excluded.updated_at
		`, deviceID, iface.IfIndex, iface.Name, iface.Type, iface.MACAddress, iface.Status, iface.Speed, iface.Duplex, iface.MTU, iface.VLANID,
			isPoE, isSFP, iface.PoEStatus, iface.PoEPowerWatt, iface.SFPWavelength, iface.SFPTemp, iface.SFPTXPowerDBm, iface.SFPRXPowerDBm,
			iface.RXBytes, iface.TXBytes, iface.RXPackets, iface.TXPackets, iface.RXErrors, iface.TXErrors, iface.RXDrops, iface.TXDrops,
			iface.SFPInfoJSON, iface.PoEInfoJSON, now)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) ListDeviceInterfaces(deviceID int64) ([]DeviceInterface, error) {
	rows, err := r.db.Query(`
		SELECT id, device_id, if_index, name, type, mac_address, status, speed, duplex, mtu, vlan_id,
		       is_poe, is_sfp, poe_status, poe_power_watt, sfp_wavelength, sfp_temp, sfp_tx_power_dbm, sfp_rx_power_dbm,
		       rx_bytes, tx_bytes, rx_packets, tx_packets, rx_errors, tx_errors, rx_drops, tx_drops,
		       sfp_info_json, poe_info_json, updated_at
		FROM device_interfaces
		WHERE device_id = ?
		ORDER BY if_index ASC, name ASC
	`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []DeviceInterface
	for rows.Next() {
		var i DeviceInterface
		var isPoE, isSFP int
		err := rows.Scan(
			&i.ID, &i.DeviceID, &i.IfIndex, &i.Name, &i.Type, &i.MACAddress, &i.Status, &i.Speed, &i.Duplex, &i.MTU, &i.VLANID,
			&isPoE, &isSFP, &i.PoEStatus, &i.PoEPowerWatt, &i.SFPWavelength, &i.SFPTemp, &i.SFPTXPowerDBm, &i.SFPRXPowerDBm,
			&i.RXBytes, &i.TXBytes, &i.RXPackets, &i.TXPackets, &i.RXErrors, &i.TXErrors, &i.RXDrops, &i.TXDrops,
			&i.SFPInfoJSON, &i.PoEInfoJSON, &i.UpdatedAt,
		)
		if err == nil {
			i.IsPoE = isPoE == 1
			i.IsSFP = isSFP == 1
			list = append(list, i)
		}
	}
	return list, nil
}

func (r *Repository) RecordMetric(m *DeviceMetric) error {
	_, err := r.db.Exec(`
		INSERT INTO device_metrics (
			device_id, cpu_load, memory_used, memory_total, storage_used, storage_total,
			temperature, voltage, uptime_seconds, rx_bytes, tx_bytes, rx_packets, tx_packets,
			rx_errors, tx_errors, rx_drops, tx_drops, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, m.DeviceID, m.CPULoad, m.MemoryUsed, m.MemoryTotal, m.StorageUsed, m.StorageTotal,
		m.Temperature, m.Voltage, m.UptimeSeconds, m.RXBytes, m.TXBytes, m.RXPackets, m.TXPackets,
		m.RXErrors, m.TXErrors, m.RXDrops, m.TXDrops, m.RecordedAt)
	return err
}

func (r *Repository) GetMetricsHistory(deviceID int64, limit int) ([]DeviceMetric, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(`
		SELECT id, device_id, cpu_load, memory_used, memory_total, storage_used, storage_total,
		       temperature, voltage, uptime_seconds, rx_bytes, tx_bytes, rx_packets, tx_packets,
		       rx_errors, tx_errors, rx_drops, tx_drops, recorded_at
		FROM device_metrics
		WHERE device_id = ?
		ORDER BY id DESC
		LIMIT ?
	`, deviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []DeviceMetric
	for rows.Next() {
		var m DeviceMetric
		if err := rows.Scan(
			&m.ID, &m.DeviceID, &m.CPULoad, &m.MemoryUsed, &m.MemoryTotal, &m.StorageUsed, &m.StorageTotal,
			&m.Temperature, &m.Voltage, &m.UptimeSeconds, &m.RXBytes, &m.TXBytes, &m.RXPackets, &m.TXPackets,
			&m.RXErrors, &m.TXErrors, &m.RXDrops, &m.TXDrops, &m.RecordedAt,
		); err == nil {
			list = append(list, m)
		}
	}
	return list, nil
}

// ─── Wireless & Clients ─────────────────────────────────────────────────────

func (r *Repository) SaveWirelessInfo(w *DeviceWireless) (int64, error) {
	res, err := r.db.Exec(`
		INSERT INTO device_wireless (
			device_id, interface_name, mode, ssid, frequency, channel_width, noise_floor,
			tx_power, ccq, signal_strength, snr, tx_rate, rx_rate, distance_km, mcs,
			airtime_usage, remote_mac, remote_device_info, connected_clients, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(device_id) DO UPDATE SET
			interface_name = excluded.interface_name,
			mode = excluded.mode,
			ssid = excluded.ssid,
			frequency = excluded.frequency,
			channel_width = excluded.channel_width,
			noise_floor = excluded.noise_floor,
			tx_power = excluded.tx_power,
			ccq = excluded.ccq,
			signal_strength = excluded.signal_strength,
			snr = excluded.snr,
			tx_rate = excluded.tx_rate,
			rx_rate = excluded.rx_rate,
			distance_km = excluded.distance_km,
			mcs = excluded.mcs,
			airtime_usage = excluded.airtime_usage,
			remote_mac = excluded.remote_mac,
			remote_device_info = excluded.remote_device_info,
			connected_clients = excluded.connected_clients,
			updated_at = excluded.updated_at
	`, w.DeviceID, w.InterfaceName, w.Mode, w.SSID, w.Frequency, w.ChannelWidth, w.NoiseFloor,
		w.TXPower, w.CCQ, w.SignalStrength, w.SNR, w.TXRate, w.RXRate, w.DistanceKm, w.MCS,
		w.AirtimeUsage, w.RemoteMAC, w.RemoteDeviceInfo, w.ConnectedClients, time.Now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Repository) GetDeviceWireless(deviceID int64) (*DeviceWireless, error) {
	var w DeviceWireless
	err := r.db.QueryRow(`
		SELECT id, device_id, interface_name, mode, ssid, frequency, channel_width, noise_floor,
		       tx_power, ccq, signal_strength, snr, tx_rate, rx_rate, distance_km, mcs,
		       airtime_usage, remote_mac, remote_device_info, connected_clients, updated_at
		FROM device_wireless
		WHERE device_id = ?
	`, deviceID).Scan(
		&w.ID, &w.DeviceID, &w.InterfaceName, &w.Mode, &w.SSID, &w.Frequency, &w.ChannelWidth, &w.NoiseFloor,
		&w.TXPower, &w.CCQ, &w.SignalStrength, &w.SNR, &w.TXRate, &w.RXRate, &w.DistanceKm, &w.MCS,
		&w.AirtimeUsage, &w.RemoteMAC, &w.RemoteDeviceInfo, &w.ConnectedClients, &w.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *Repository) SaveWirelessClients(deviceID int64, wirelessID int64, clients []DeviceWirelessClient) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now()
	for _, c := range clients {
		_, err := tx.Exec(`
			INSERT INTO device_wireless_clients (
				device_id, wireless_id, mac_address, ip_address, hostname, signal, noise, snr,
				tx_rate, rx_rate, ccq, uptime_seconds, rx_bytes, tx_bytes, status, last_seen
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(device_id, mac_address) DO UPDATE SET
				wireless_id = excluded.wireless_id,
				ip_address = excluded.ip_address,
				hostname = excluded.hostname,
				signal = excluded.signal,
				noise = excluded.noise,
				snr = excluded.snr,
				tx_rate = excluded.tx_rate,
				rx_rate = excluded.rx_rate,
				ccq = excluded.ccq,
				uptime_seconds = excluded.uptime_seconds,
				rx_bytes = excluded.rx_bytes,
				tx_bytes = excluded.tx_bytes,
				status = excluded.status,
				last_seen = excluded.last_seen
		`, deviceID, wirelessID, c.MACAddress, c.IPAddress, c.Hostname, c.Signal, c.Noise, c.SNR,
			c.TXRate, c.RXRate, c.CCQ, c.UptimeSeconds, c.RXBytes, c.TXBytes, c.Status, now)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) ListWirelessClients(deviceID int64) ([]DeviceWirelessClient, error) {
	rows, err := r.db.Query(`
		SELECT id, device_id, COALESCE(wireless_id, 0), mac_address, COALESCE(ip_address, ''), COALESCE(hostname, ''),
		       signal, noise, snr, tx_rate, rx_rate, ccq, uptime_seconds, rx_bytes, tx_bytes, status, last_seen
		FROM device_wireless_clients
		WHERE device_id = ?
		ORDER BY signal DESC, last_seen DESC
	`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []DeviceWirelessClient
	for rows.Next() {
		var c DeviceWirelessClient
		if err := rows.Scan(
			&c.ID, &c.DeviceID, &c.WirelessID, &c.MACAddress, &c.IPAddress, &c.Hostname,
			&c.Signal, &c.Noise, &c.SNR, &c.TXRate, &c.RXRate, &c.CCQ, &c.UptimeSeconds,
			&c.RXBytes, &c.TXBytes, &c.Status, &c.LastSeen,
		); err == nil {
			list = append(list, c)
		}
	}
	return list, nil
}

// ─── Events & Alerts ────────────────────────────────────────────────────────

func (r *Repository) LogEvent(deviceID int64, eventType, severity, message, eventDataJSON string) error {
	_, err := r.db.Exec(`
		INSERT INTO device_events (device_id, event_type, severity, message, event_data_json)
		VALUES (?, ?, ?, ?, ?)
	`, deviceID, eventType, severity, message, eventDataJSON)
	return err
}

func (r *Repository) ListEvents(deviceID int64, limit int) ([]DeviceEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(`
		SELECT id, device_id, event_type, severity, message, COALESCE(event_data_json, '{}'), created_at
		FROM device_events
		WHERE device_id = ?
		ORDER BY id DESC
		LIMIT ?
	`, deviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []DeviceEvent
	for rows.Next() {
		var e DeviceEvent
		if err := rows.Scan(&e.ID, &e.DeviceID, &e.EventType, &e.Severity, &e.Message, &e.EventDataJSON, &e.CreatedAt); err == nil {
			list = append(list, e)
		}
	}
	return list, nil
}

func (r *Repository) ListAlerts(activeOnly bool) ([]DeviceAlert, error) {
	query := `
		SELECT a.id, a.device_id, d.name, a.alert_type, a.message, a.status, a.triggered_at, a.resolved_at
		FROM device_alerts a
		JOIN devices d ON a.device_id = d.id
	`
	if activeOnly {
		query += " WHERE a.status = 'active'"
	}
	query += " ORDER BY a.id DESC LIMIT 100"

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []DeviceAlert
	for rows.Next() {
		var a DeviceAlert
		var resAt sql.NullTime
		if err := rows.Scan(&a.ID, &a.DeviceID, &a.DeviceName, &a.AlertType, &a.Message, &a.Status, &a.TriggeredAt, &resAt); err == nil {
			if resAt.Valid {
				a.ResolvedAt = &resAt.Time
			}
			list = append(list, a)
		}
	}
	return list, nil
}
