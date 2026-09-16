package cloudtenant

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestCloudExpiredProfileRedirection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cloud_expired_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cloudDir := filepath.Join(tempDir, "cloud_tenants")
	_ = os.MkdirAll(cloudDir, 0755)

	pool := NewTenantDBPool(cloudDir)
	mgr := NewManager(nil, pool, "cloud.sasman.net", []byte("test_secret"))

	subdomain := "redirect-test"
	db, err := pool.Get(subdomain)
	if err != nil {
		t.Fatalf("Failed to get cloud tenant db: %v", err)
	}

	// 1. Create Profiles:
	// Profile A: Has expired_profile = 'expired-profile'
	_, _ = db.Exec(`INSERT INTO radgroupreply (groupname, attribute, op, value) VALUES ('ActivePlan', 'Mikrotik-Rate-Limit', ':=', '20M/20M');`)
	_, _ = db.Exec(`INSERT INTO radgroupreply (groupname, attribute, op, value) VALUES ('ActivePlan', 'Mikrotik-Group', ':=', 'active-profile');`)
	_, _ = db.Exec(`INSERT INTO radius_profile_meta (groupname, validity_days, price, expired_profile, expired_pool) VALUES ('ActivePlan', 30, 25000, 'expired-profile', 'expired-pool');`)

	// Profile B: No expired_profile or expired_pool configured
	_, _ = db.Exec(`INSERT INTO radgroupreply (groupname, attribute, op, value) VALUES ('StrictPlan', 'Mikrotik-Rate-Limit', ':=', '10M/10M');`)
	_, _ = db.Exec(`INSERT INTO radius_profile_meta (groupname, validity_days, price, expired_profile, expired_pool) VALUES ('StrictPlan', 30, 15000, '', '');`)

	// 2. Create User 1: Expired subscriber on ActivePlan
	_, _ = db.Exec("INSERT INTO radcheck (username, attribute, op, value) VALUES ('user_expired_with_profile', 'Cleartext-Password', ':=', 'pass1')")
	_, _ = db.Exec("INSERT INTO radusergroup (username, groupname, priority) VALUES ('user_expired_with_profile', 'ActivePlan', 1)")
	pastExp := time.Now().Add(-2 * time.Hour).Unix()
	_, _ = db.Exec("INSERT INTO radius_user_meta (username, full_name, expiration_unix, enabled) VALUES ('user_expired_with_profile', 'Expired User', ?, 1)", pastExp)

	// Verify User 1 gets redirected to expired profile
	auth1 := mgr.VerifyCloudUserDetails(subdomain, "user_expired_with_profile", "pass1")
	if !auth1.Allow {
		t.Fatalf("Expected Allow=true for expired user with expired_profile, got false: %s", auth1.RejectReason)
	}
	if auth1.MikrotikGroup != "expired-profile" {
		t.Fatalf("Expected MikrotikGroup='expired-profile', got '%s'", auth1.MikrotikGroup)
	}
	if auth1.FramedPool != "expired-pool" {
		t.Fatalf("Expected FramedPool='expired-pool', got '%s'", auth1.FramedPool)
	}
	t.Logf("✅ User 1: Successfully allowed onto expired profile '%s' with pool '%s'", auth1.MikrotikGroup, auth1.FramedPool)

	// 3. Create User 2: Expired subscriber on StrictPlan (no expired profile)
	_, _ = db.Exec("INSERT INTO radcheck (username, attribute, op, value) VALUES ('user_expired_strict', 'Cleartext-Password', ':=', 'pass2')")
	_, _ = db.Exec("INSERT INTO radusergroup (username, groupname, priority) VALUES ('user_expired_strict', 'StrictPlan', 1)")
	_, _ = db.Exec("INSERT INTO radius_user_meta (username, full_name, expiration_unix, enabled) VALUES ('user_expired_strict', 'Strict Expired', ?, 1)", pastExp)

	// Verify User 2 gets rejected
	auth2 := mgr.VerifyCloudUserDetails(subdomain, "user_expired_strict", "pass2")
	if auth2.Allow {
		t.Fatalf("Expected Allow=false for expired user with no expired_profile, got true")
	}
	if auth2.RejectReason != "انتهى اشتراك المستخدم" {
		t.Fatalf("Expected RejectReason='انتهى اشتراك المستخدم', got '%s'", auth2.RejectReason)
	}
	t.Logf("✅ User 2: Correctly rejected with reason: '%s'", auth2.RejectReason)

	// 4. Create User 3: Active subscriber on ActivePlan
	futureExp := time.Now().Add(10 * 24 * time.Hour).Unix()
	_, _ = db.Exec("INSERT INTO radcheck (username, attribute, op, value) VALUES ('user_active', 'Cleartext-Password', ':=', 'pass3')")
	_, _ = db.Exec("INSERT INTO radusergroup (username, groupname, priority) VALUES ('user_active', 'ActivePlan', 1)")
	_, _ = db.Exec("INSERT INTO radius_user_meta (username, full_name, expiration_unix, enabled) VALUES ('user_active', 'Active User', ?, 1)", futureExp)

	// Verify User 3 gets normal speed and active profile
	auth3 := mgr.VerifyCloudUserDetails(subdomain, "user_active", "pass3")
	if !auth3.Allow {
		t.Fatalf("Expected Allow=true for active user, got false: %s", auth3.RejectReason)
	}
	if auth3.MikrotikGroup != "active-profile" {
		t.Fatalf("Expected MikrotikGroup='active-profile', got '%s'", auth3.MikrotikGroup)
	}
	if auth3.RateLimit != "20M/20M" {
		t.Fatalf("Expected RateLimit='20M/20M', got '%s'", auth3.RateLimit)
	}
	t.Logf("✅ User 3: Successfully authenticated with regular speed '%s' and group '%s'", auth3.RateLimit, auth3.MikrotikGroup)
}
