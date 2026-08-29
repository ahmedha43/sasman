package main

import (
	"os"
	"path/filepath"
	"testing"

	"mikrotik-manager/server/internal/cloudtenant"
)

func TestCloudTenantLifecycle(t *testing.T) {
	testBaseDir := filepath.Join(os.TempDir(), "sasman_test_tenants")
	_ = os.RemoveAll(testBaseDir)
	defer os.RemoveAll(testBaseDir)

	pool := cloudtenant.NewTenantDBPool(testBaseDir)
	mgr := cloudtenant.NewManager(nil, pool, "sas-man.net", []byte("TEST_SECRET"))

	// 1. Check Subdomain Availability
	avail, reason := mgr.IsSubdomainAvailable("testcloud")
	if !avail {
		t.Fatalf("Subdomain testcloud should be available: %s", reason)
	}

	// 2. Register Tenant
	tenant, err := mgr.RegisterTenant(cloudtenant.RegisterRequest{
		Subdomain: "testcloud",
		Email:     "admin@testcloud.net",
		Password:  "CloudPass123!",
		OwnerName: "Al-Qamar Network",
		Phone:     "07700000000",
	})
	if err != nil {
		t.Fatalf("Failed to register tenant: %v", err)
	}
	if tenant.Subdomain != "testcloud" {
		t.Fatalf("Expected subdomain testcloud, got: %s", tenant.Subdomain)
	}

	// 3. Login
	authTenant, token, err := mgr.AuthenticateTenant("testcloud", "CloudPass123!")
	if err != nil || token == "" {
		t.Fatalf("Failed to authenticate tenant: %v", err)
	}
	if authTenant.Subdomain != "testcloud" {
		t.Fatalf("Expected auth subdomain testcloud, got: %s", authTenant.Subdomain)
	}

	// 4. Add User to Tenant's Database
	db, err := pool.Get("testcloud")
	if err != nil {
		t.Fatalf("Failed to get tenant DB: %v", err)
	}
	_, err = db.Exec("INSERT INTO radcheck (username, attribute, op, value) VALUES ('user1', 'Cleartext-Password', ':=', '1234')")
	if err != nil {
		t.Fatalf("Failed to insert user: %v", err)
	}
	_, _ = db.Exec("INSERT INTO radusergroup (username, groupname) VALUES ('user1', '10M')")

	// 5. Verify RadSec Access-Accept
	allow, rateLimit, _, reason, err := mgr.VerifyCloudUser("testcloud", "user1", "1234")
	if err != nil || !allow {
		t.Fatalf("RadSec verify failed: %v (reason: %s)", err, reason)
	}
	if rateLimit == "" {
		t.Fatalf("Expected non-empty rate limit, got: %s", rateLimit)
	}

	// 6. Verify Wrong Password Rejection
	allowWrong, _, _, _, _ := mgr.VerifyCloudUser("testcloud", "user1", "wrong_pass")
	if allowWrong {
		t.Fatalf("RadSec should have rejected wrong password")
	}

	// 7. Test Accounting
	err = mgr.RecordCloudAccounting("testcloud", cloudtenant.CloudAccountingPayload{
		Username:       "user1",
		StatusType:     "Start",
		SessionID:      "sess_9999",
		UserIP:         "10.0.0.50",
		UserMAC:        "11:22:33:44:55:66",
		NasIP:          "10.0.0.1",
		BytesIn:        2048,
		BytesOut:       4096,
		SessionTimeSec: 10,
	})
	if err != nil {
		t.Fatalf("Failed to record accounting: %v", err)
	}

	var sessCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM radacct WHERE username = 'user1' AND acctsessionid = 'sess_9999'").Scan(&sessCount)
	if sessCount != 1 {
		t.Fatalf("Expected 1 accounting session in DB, got: %d", sessCount)
	}

	t.Logf("✅ All SASMAN Cloud Tenant lifecycle tests passed successfully!")
}
