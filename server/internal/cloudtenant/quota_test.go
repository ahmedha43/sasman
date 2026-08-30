package cloudtenant

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestCloudDataQuotaEnforcement(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cloud_quota_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cloudDir := filepath.Join(tempDir, "cloud_tenants")
	_ = os.MkdirAll(cloudDir, 0755)

	pool := NewTenantDBPool(cloudDir)
	mgr := NewManager(nil, pool, "cloud.sasman.net", []byte("test_secret"))

	subdomain := "quota-agent"
	db, err := pool.Get(subdomain)
	if err != nil {
		t.Fatalf("Failed to get cloud tenant db: %v", err)
	}

	// 1. Create a 5 GB (5120 MB) Profile in Cloud Tenant
	quotaMB := int64(5120) // 5 GB
	_, err = db.Exec(`
		INSERT INTO radgroupreply (groupname, attribute, op, value) VALUES ('5GB_Speed', 'Mikrotik-Rate-Limit', ':=', '20M/20M');
	`)
	if err != nil {
		t.Fatalf("Failed to insert cloud group reply: %v", err)
	}
	_, err = db.Exec(`
		INSERT INTO radius_profile_meta (groupname, validity_days, price, quota_limit_mb)
		VALUES ('5GB_Speed', 30, 15000, ?);
	`, quotaMB)
	if err != nil {
		t.Fatalf("Failed to insert cloud profile meta: %v", err)
	}
	t.Logf("✅ Step 1 (Cloud): Created Profile '5GB_Speed' with Quota = 5 GB (%d MB)", quotaMB)

	// 2. Create User 'cloud_user'
	username := "cloud_user"
	password := "pass123"
	_, err = db.Exec("INSERT INTO radcheck (username, attribute, op, value) VALUES (?, 'Cleartext-Password', ':=', ?)", username, password)
	if err != nil {
		t.Fatalf("Failed to insert cloud user radcheck: %v", err)
	}
	_, err = db.Exec("INSERT INTO radusergroup (username, groupname, priority) VALUES (?, '5GB_Speed', 1)", username)
	if err != nil {
		t.Fatalf("Failed to insert cloud user group: %v", err)
	}
	expUnix := time.Now().Add(30 * 24 * time.Hour).Unix()
	_, err = db.Exec(`
		INSERT INTO radius_user_meta (username, full_name, expiration_unix, quota_limit_mb, used_octets_in, used_octets_out, quota_status, enabled)
		VALUES (?, 'Cloud Subscriber', ?, ?, 0, 0, 'active', 1)
	`, username, expUnix, quotaMB)
	if err != nil {
		t.Fatalf("Failed to insert cloud user meta: %v", err)
	}
	t.Logf("✅ Step 2 (Cloud): Created User '%s' in tenant '%s'", username, subdomain)

	// 3. Verify Cloud User Auth Details
	authDetails := mgr.VerifyCloudUserDetails(subdomain, username, password)
	if !authDetails.Allow {
		t.Fatalf("❌ Expected Allow=true in Cloud, got false: %s", authDetails.RejectReason)
	}

	totalQuotaBytes := uint64(quotaMB * 1024 * 1024) // 5368709120 bytes (> 4GB, requires Gigawords)
	expectedLow := uint32(totalQuotaBytes % (1 << 32))
	expectedGiga := uint32(totalQuotaBytes >> 32)

	if authDetails.TotalLimit != expectedLow || authDetails.TotalLimitGigawords != expectedGiga {
		t.Fatalf("❌ Cloud TotalLimit mismatch: expected Low=%d, Giga=%d, got Low=%d, Giga=%d",
			expectedLow, expectedGiga, authDetails.TotalLimit, authDetails.TotalLimitGigawords)
	}
	t.Logf("✅ Step 3 (Cloud): Auth returned TotalLimit=%d and TotalLimitGigawords=%d (Upper 32-bit Gigaword for 5GB)",
		authDetails.TotalLimit, authDetails.TotalLimitGigawords)

	// 4. Simulate Accounting Stop with 5.5 GB (Exhausted)
	err = mgr.RecordCloudAccounting(subdomain, CloudAccountingPayload{
		Username:       username,
		StatusType:     "Stop",
		SessionID:      "cloud-sess-1",
		BytesIn:        4 * 1024 * 1024 * 1024,
		BytesOut:       1500 * 1024 * 1024, // Total 5.5 GB
		SessionTimeSec: 3600,
	})
	if err != nil {
		t.Fatalf("Failed to record cloud accounting: %v", err)
	}

	// Verify status in DB
	var quotaStatus string
	_ = db.QueryRow("SELECT quota_status FROM radius_user_meta WHERE username=?", username).Scan(&quotaStatus)
	if quotaStatus != "depleted" {
		t.Fatalf("❌ Expected cloud quota_status = 'depleted', got: %s", quotaStatus)
	}
	t.Logf("✅ Step 4 (Cloud): Accounting recorded 5.5 GB, tenant DB marked quota_status = '%s'", quotaStatus)

	// Auth should now be rejected!
	authDetails2 := mgr.VerifyCloudUserDetails(subdomain, username, password)
	if authDetails2.Allow {
		t.Fatalf("❌ Expected Cloud Allow=false after quota depletion, got true!")
	}
	t.Logf("✅ Step 5 (Cloud): Auth correctly REJECTED: Reason = '%s'", authDetails2.RejectReason)

	// 5. Reset Quota in Cloud Tenant DB
	_, _ = db.Exec("UPDATE radius_user_meta SET used_octets_in = 0, used_octets_out = 0, quota_status = 'active' WHERE username = ?", username)
	authDetails3 := mgr.VerifyCloudUserDetails(subdomain, username, password)
	if !authDetails3.Allow {
		t.Fatalf("❌ Expected Cloud Allow=true after quota reset, got false: %s", authDetails3.RejectReason)
	}
	t.Logf("✅ Step 6 (Cloud): Re-Auth after reset succeeded with full 5GB quota restored!")
}
