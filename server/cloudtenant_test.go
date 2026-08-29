package main

import (
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mikrotik-manager/server/internal/cloudtenant"

	"github.com/gofiber/fiber/v2"
)

func TestCloudTenantHTTPAPIs(t *testing.T) {
	app := fiber.New()
	baseDir := filepath.Join(os.TempDir(), fmt.Sprintf("test_tenant_http_%d", time.Now().UnixNano()))
	defer os.RemoveAll(baseDir)

	pool := cloudtenant.NewTenantDBPool(baseDir)
	defer pool.CloseAll()

	mgr := cloudtenant.NewManager(nil, pool, "sas-man.net", []byte("TEST_SECRET"))
	apiH := cloudtenant.NewAPIHandler(mgr)
	apiH.RegisterRoutes(app)

	// Register tenant
	_, err := mgr.RegisterTenant(cloudtenant.RegisterRequest{
		Subdomain: "sasradius",
		Email:     "admin@sasradius.net",
		Password:  "admin123",
		OwnerName: "SAS Radius",
		Phone:     "07700000000",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	// Login to get token
	_, token, err := mgr.AuthenticateTenant("sasradius", "admin123")
	if err != nil {
		t.Fatalf("auth: %v", err)
	}

	// Test GET /radius/api/users with Host: sasradius.sas-man.net and Bearer token
	req := httptest.NewRequest("GET", "/radius/api/users", nil)
	req.Host = "sasradius.sas-man.net"
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, 5000)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("GET /radius/api/users status: %d, err: %v", resp.StatusCode, err)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "[]" {
		t.Fatalf("Expected empty array [], got: %s", string(body))
	}

	// Test GET /radius/api/profiles
	req = httptest.NewRequest("GET", "/radius/api/profiles", nil)
	req.Host = "sasradius.sas-man.net"
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = app.Test(req, 5000)
	body, _ = io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("GET /radius/api/profiles status: %d, body: %s", resp.StatusCode, string(body))
	}
	if !strings.Contains(string(body), "10M") {
		t.Fatalf("Expected default profile 10M, got: %s", string(body))
	}

	// Test POST /radius/api/users (Create User)
	createUserJSON := `{"user":"th","pass":"1234","profile":"10M","days":30,"full_name":"Ahmed Test"}`
	req = httptest.NewRequest("POST", "/radius/api/users", strings.NewReader(createUserJSON))
	req.Host = "sasradius.sas-man.net"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = app.Test(req, 5000)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("POST /radius/api/users status: %d", resp.StatusCode)
	}

	// Verify User in GET /radius/api/users
	req = httptest.NewRequest("GET", "/radius/api/users", nil)
	req.Host = "sasradius.sas-man.net"
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = app.Test(req, 5000)
	body, _ = io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"user":"th"`) {
		t.Fatalf("Expected user 'th' in list, got: %s", string(body))
	}

	// Test GET /radius/api/nas
	req = httptest.NewRequest("GET", "/radius/api/nas", nil)
	req.Host = "sasradius.sas-man.net"
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = app.Test(req, 5000)
	if resp.StatusCode != 200 {
		t.Fatalf("GET /radius/api/nas status: %d", resp.StatusCode)
	}

	// Test GET /radius/api/auth/admins
	req = httptest.NewRequest("GET", "/radius/api/auth/admins", nil)
	req.Host = "sasradius.sas-man.net"
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = app.Test(req, 5000)
	if resp.StatusCode != 200 {
		t.Fatalf("GET /radius/api/auth/admins status: %d", resp.StatusCode)
	}

	// Test GET /radius/api/broadcasts/active
	req = httptest.NewRequest("GET", "/radius/api/broadcasts/active", nil)
	req.Host = "sasradius.sas-man.net"
	resp, err = app.Test(req, 5000)
	if resp.StatusCode != 200 {
		t.Fatalf("GET /radius/api/broadcasts/active status: %d", resp.StatusCode)
	}

	// Test POST /radius/api/users/th/renew
	renewJSON := `{"profile":"10M"}`
	req = httptest.NewRequest("POST", "/radius/api/users/th/renew", strings.NewReader(renewJSON))
	req.Host = "sasradius.sas-man.net"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = app.Test(req, 5000)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("POST /radius/api/users/th/renew status: %d", resp.StatusCode)
	}

	// Test GET /radius/api/audit-logs
	req = httptest.NewRequest("GET", "/radius/api/audit-logs", nil)
	req.Host = "sasradius.sas-man.net"
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = app.Test(req, 5000)
	if resp.StatusCode != 200 {
		t.Fatalf("GET /radius/api/audit-logs status: %d", resp.StatusCode)
	}

	// Test GET /radius/api/logs
	req = httptest.NewRequest("GET", "/radius/api/logs", nil)
	req.Host = "sasradius.sas-man.net"
	resp, err = app.Test(req, 5000)
	if resp.StatusCode != 200 {
		t.Fatalf("GET /radius/api/logs status: %d", resp.StatusCode)
	}

	t.Logf("✅ All Cloud Tenant HTTP APIs tested successfully and returned proper Arrays/JSON!")
}

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
