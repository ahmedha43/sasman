package devices_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"mikrotik-manager/pkg/devices"
	_ "mikrotik-manager/pkg/devices/drivers/mikrotik"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dbPath := "test_devices_" + time.Now().Format("20060102150405.000") + ".db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test db: %v", err)
	}

	if err := devices.EnsureSchema(db); err != nil {
		t.Fatalf("Failed to ensure schema: %v", err)
	}

	cleanup := func() {
		_ = db.Close()
		_ = os.Remove(dbPath)
		_ = os.Remove(dbPath + "-shm")
		_ = os.Remove(dbPath + "-wal")
	}

	return db, cleanup
}

func TestCrypto(t *testing.T) {
	plain := "SecretP@ssw0rd123!#"
	enc, err := devices.EncryptPassword(plain)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if enc == plain {
		t.Errorf("Encrypted password equals plaintext!")
	}

	dec, err := devices.DecryptPassword(enc)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if dec != plain {
		t.Errorf("Expected decrypted text '%s', got '%s'", plain, dec)
	}
}

func TestDeviceRegistry(t *testing.T) {
	driver, err := devices.GetDriver("mikrotik")
	if err != nil {
		t.Fatalf("MikroTik driver not found in registry: %v", err)
	}
	if driver == nil {
		t.Fatal("MikroTik driver is nil")
	}

	_, err = devices.GetDriver("non_existent_vendor")
	if err == nil {
		t.Error("Expected error for non existent vendor, got nil")
	}
}

func TestRepositoryDeviceCRUD(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := devices.NewRepository(db)

	vendors, err := repo.ListVendors()
	if err != nil || len(vendors) == 0 {
		t.Fatalf("Failed to list seed vendors: %v", err)
	}

	types, err := repo.ListDeviceTypes()
	if err != nil || len(types) == 0 {
		t.Fatalf("Failed to list seed device types: %v", err)
	}

	// 1. Create a Switch Device
	switchDev := &devices.Device{
		VendorID:        vendors[0].ID,
		TypeID:          types[0].ID,
		Name:            "Tower-Main-CRS326",
		IP:              "192.168.10.2",
		Port:            8728,
		PollIntervalSec: 15,
		IsMonitored:     true,
		Location:        "Tower Alpha - Cabinet A",
		Notes:           "Main Distribution Switch",
	}

	cred := &devices.DeviceCredential{
		Username:          "admin",
		PasswordEncrypted: "mypassword",
		AuthType:          "api",
	}

	devID, err := repo.CreateDevice(switchDev, cred)
	if err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}

	if devID <= 0 {
		t.Errorf("Expected positive device ID, got %d", devID)
	}

	// 2. Fetch Device
	fetched, err := repo.GetDevice(devID)
	if err != nil {
		t.Fatalf("Failed to fetch device %d: %v", devID, err)
	}
	if fetched.Name != "Tower-Main-CRS326" {
		t.Errorf("Expected name 'Tower-Main-CRS326', got '%s'", fetched.Name)
	}

	// 3. Verify Decrypted Credentials
	target, err := repo.GetDeviceCredentials(devID)
	if err != nil {
		t.Fatalf("Failed to get credentials: %v", err)
	}
	if target.Password != "mypassword" {
		t.Errorf("Expected decrypted password 'mypassword', got '%s'", target.Password)
	}

	// 4. Save Interfaces
	ifaces := []devices.DeviceInterface{
		{
			IfIndex:    1,
			Name:       "ether1",
			Type:       "ether",
			MACAddress: "48:8F:5A:11:22:33",
			Status:     "up",
			Speed:      "1Gbps",
			Duplex:     "full",
			MTU:        1500,
			IsPoE:      true,
			PoEStatus:  "powered",
		},
		{
			IfIndex:    2,
			Name:       "sfp-sfpplus1",
			Type:       "sfp",
			MACAddress: "48:8F:5A:11:22:34",
			Status:     "up",
			Speed:      "10Gbps",
			IsSFP:      true,
		},
	}
	err = repo.SaveInterfaces(devID, ifaces)
	if err != nil {
		t.Fatalf("Failed to save interfaces: %v", err)
	}

	fetchedIfaces, err := repo.ListDeviceInterfaces(devID)
	if err != nil || len(fetchedIfaces) != 2 {
		t.Fatalf("Expected 2 interfaces, got %d (err: %v)", len(fetchedIfaces), err)
	}

	// 5. Test State Machine Transitions
	now := time.Now()
	// Set online
	err = repo.UpdateDeviceOnlineStatus(devID, true, false, "", now)
	if err != nil {
		t.Fatalf("Failed to set online: %v", err)
	}

	dOnline, _ := repo.GetDevice(devID)
	if dOnline.Status != "online" {
		t.Errorf("Expected status 'online', got '%s'", dOnline.Status)
	}

	// Transition to offline
	offTime := now.Add(1 * time.Minute)
	err = repo.UpdateDeviceOnlineStatus(devID, false, false, "Connection refused", offTime)
	if err != nil {
		t.Fatalf("Failed to set offline: %v", err)
	}

	dOffline, _ := repo.GetDevice(devID)
	if dOffline.Status != "offline" {
		t.Errorf("Expected status 'offline', got '%s'", dOffline.Status)
	}

	// Recover back online after 30 seconds
	recoverTime := offTime.Add(30 * time.Second)
	err = repo.UpdateDeviceOnlineStatus(devID, true, false, "", recoverTime)
	if err != nil {
		t.Fatalf("Failed to recover: %v", err)
	}

	dRecovered, _ := repo.GetDevice(devID)
	if dRecovered.Status != "online" {
		t.Errorf("Expected status 'online', got '%s'", dRecovered.Status)
	}
	if dRecovered.DowntimeDurationSec < 30 {
		t.Errorf("Expected recorded downtime >= 30s, got %d", dRecovered.DowntimeDurationSec)
	}

	// 6. Test Metrics History
	m := &devices.DeviceMetric{
		DeviceID:      devID,
		CPULoad:       15,
		MemoryUsed:    256 * 1024 * 1024,
		MemoryTotal:   512 * 1024 * 1024,
		StorageUsed:   16 * 1024 * 1024,
		StorageTotal:  64 * 1024 * 1024,
		Temperature:   42.5,
		Voltage:       24.1,
		UptimeSeconds: 3600,
		RXBytes:       1000000,
		TXBytes:       500000,
		RecordedAt:    time.Now(),
	}
	if err := repo.RecordMetric(m); err != nil {
		t.Fatalf("Failed to record metric: %v", err)
	}

	history, err := repo.GetMetricsHistory(devID, 10)
	if err != nil || len(history) == 0 {
		t.Fatalf("Expected metric history, got %d (err: %v)", len(history), err)
	}

	// 7. Test Wireless & Clients for Sector AP
	sectorDev := &devices.Device{
		VendorID:        vendors[0].ID,
		TypeID:          types[2].ID, // Sector
		Name:            "Tower-Sector-North-5GHz",
		IP:              "192.168.30.2",
		PollIntervalSec: 20,
		IsMonitored:     true,
	}
	sectorID, err := repo.CreateDevice(sectorDev, nil)
	if err != nil {
		t.Fatalf("Failed to create sector: %v", err)
	}

	wInfo := &devices.DeviceWireless{
		DeviceID:         sectorID,
		InterfaceName:    "wlan1",
		Mode:             "ap-bridge",
		SSID:             "Tower-North-5805",
		Frequency:        5805,
		ChannelWidth:     "40MHz",
		NoiseFloor:       -96,
		TXPower:          20,
		ConnectedClients: 2,
	}
	wID, err := repo.SaveWirelessInfo(wInfo)
	if err != nil {
		t.Fatalf("Failed to save wireless info: %v", err)
	}

	clients := []devices.DeviceWirelessClient{
		{
			DeviceID:      sectorID,
			WirelessID:    wID,
			MACAddress:    "D4:01:C3:12:34:56",
			IPAddress:     "10.10.10.45",
			Hostname:      "Client-Ahmad",
			Signal:        -58,
			Noise:         -96,
			SNR:           38,
			TXRate:        "144 Mbps",
			RXRate:        "130 Mbps",
			CCQ:           95,
			UptimeSeconds: 7200,
			Status:        "connected",
		},
		{
			DeviceID:      sectorID,
			WirelessID:    wID,
			MACAddress:    "D4:01:C3:12:34:57",
			IPAddress:     "10.10.10.46",
			Hostname:      "Client-Mustafa",
			Signal:        -64,
			Noise:         -96,
			SNR:           32,
			TXRate:        "86 Mbps",
			RXRate:        "86 Mbps",
			CCQ:           88,
			UptimeSeconds: 3600,
			Status:        "connected",
		},
	}
	err = repo.SaveWirelessClients(sectorID, wID, clients)
	if err != nil {
		t.Fatalf("Failed to save wireless clients: %v", err)
	}

	fetchedClients, err := repo.ListWirelessClients(sectorID)
	if err != nil || len(fetchedClients) != 2 {
		t.Fatalf("Expected 2 clients, got %d", len(fetchedClients))
	}
}

func TestServiceSummary(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	svc, err := devices.Init(db)
	if err != nil {
		t.Fatalf("Failed to init service: %v", err)
	}
	defer svc.GetMonitor().Stop()

	// Add test device via Service
	_, err = svc.AddDevice(context.Background(), devices.AddDeviceRequest{
		VendorSlug:      "mikrotik",
		TypeSlug:        "switch",
		Name:            "Switch-Lab-01",
		IP:              "192.168.88.2",
		Port:            8728,
		Username:        "admin",
		Password:        "pass",
		PollIntervalSec: 30,
		IsMonitored:     true,
	})
	if err != nil {
		t.Fatalf("Failed to add device via service: %v", err)
	}

	stats, err := svc.GetSummaryStats()
	if err != nil {
		t.Fatalf("Failed to get summary stats: %v", err)
	}

	if stats.TotalDevices != 1 {
		t.Errorf("Expected 1 total device, got %d", stats.TotalDevices)
	}
	if stats.SwitchesCount != 1 {
		t.Errorf("Expected 1 switch, got %d", stats.SwitchesCount)
	}
}
