package radius

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
	"github.com/wxccs/radius/v2/packet"
	"github.com/wxccs/radius/v2/types"
)

func encryptUserPassword(password string, auth [16]byte, secret []byte) []byte {
	b := []byte(password)
	pad := 16 - (len(b) % 16)
	if pad != 16 && len(b) > 0 {
		b = append(b, make([]byte, pad)...)
	} else if len(b) == 0 {
		b = make([]byte, 16)
	}
	out := make([]byte, len(b))
	last := auth[:]
	for i := 0; i < len(b); i += 16 {
		h := md5.New()
		h.Write(secret)
		h.Write(last)
		digest := h.Sum(nil)
		for j := 0; j < 16; j++ {
			out[i+j] = b[i+j] ^ digest[j]
		}
		last = out[i : i+16]
	}
	return out
}

func TestLocalDataQuotaEnforcement(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "local_quota_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test_radius.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open sqlite db: %v", err)
	}
	defer db.Close()

	DB = db
	EnsureSchema()

	// 1. Create a 500 MB Profile
	quotaMB := int64(500)
	_, err = db.Exec(`
		INSERT INTO radgroupreply (groupname, attribute, op, value) VALUES ('500MB_Pack', 'Mikrotik-Rate-Limit', ':=', '10M/10M');
	`)
	if err != nil {
		t.Fatalf("Failed to insert radgroupreply: %v", err)
	}
	_, err = db.Exec(`
		INSERT INTO radius_profile_meta (groupname, validity_days, price, quota_limit_mb)
		VALUES ('500MB_Pack', 30, 5000, ?);
	`, quotaMB)
	if err != nil {
		t.Fatalf("Failed to insert profile meta: %v", err)
	}
	t.Logf("✅ Step 1: Created Profile '500MB_Pack' with Quota = %d MB", quotaMB)

	// 2. Create User 'tester_quota'
	username := "tester_quota"
	password := "123456"
	_, err = db.Exec("INSERT INTO radcheck (username, attribute, op, value) VALUES (?, 'Cleartext-Password', ':=', ?)", username, password)
	if err != nil {
		t.Fatalf("Failed to insert user radcheck: %v", err)
	}
	_, err = db.Exec("INSERT INTO radusergroup (username, groupname, priority) VALUES (?, '500MB_Pack', 1)", username)
	if err != nil {
		t.Fatalf("Failed to insert user group: %v", err)
	}
	expUnix := time.Now().Add(30 * 24 * time.Hour).Unix()
	_, err = db.Exec(`
		INSERT INTO radius_user_meta (username, full_name, expiration_unix, quota_limit_mb, used_octets_in, used_octets_out, quota_status, enabled)
		VALUES (?, 'Quota Tester', ?, ?, 0, 0, 'active', 1)
	`, username, expUnix, quotaMB)
	if err != nil {
		t.Fatalf("Failed to insert user meta: %v", err)
	}
	t.Logf("✅ Step 2: Created User '%s' with Profile '500MB_Pack'", username)

	// 3. Authenticate initial Access-Request
	secret := []byte("testing123")
	auth := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	reqPkt := &packet.Packet{
		Code:          types.AccessRequest,
		Identifier:    1,
		Authenticator: auth,
	}
	reqPkt.Attributes = append(reqPkt.Attributes, packet.NewString(types.AttrUserName, username))
	encPass := encryptUserPassword(password, auth, secret)
	reqPkt.Attributes = append(reqPkt.Attributes, packet.NewOctets(types.AttrUserPassword, encPass))

	req := NewServerRequestForTest(reqPkt, secret, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 1812})

	authResp, err := handleAuthRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Auth failed with error: %v", err)
	}
	if authResp.Code != types.AccessAccept {
		t.Fatalf("Expected Access-Accept, got: %v", authResp.Code)
	}

	// Verify Mikrotik-Total-Limit attribute
	expectedBytes := uint32(quotaMB * 1024 * 1024)
	foundTotalLimit := false
	for _, attr := range authResp.Attributes {
		if attr.Type == types.AttrVendorSpecific && len(attr.Value) >= 10 {
			vendorID := binary.BigEndian.Uint32(attr.Value[0:4])
			subType := attr.Value[4]
			if vendorID == 14988 && subType == 17 { // Mikrotik-Total-Limit
				limitVal := binary.BigEndian.Uint32(attr.Value[6:10])
				if limitVal != expectedBytes {
					t.Fatalf("❌ Expected Total-Limit = %d bytes, got %d", expectedBytes, limitVal)
				}
				foundTotalLimit = true
				t.Logf("✅ Step 3: RADIUS Access-Accept contains Mikrotik-Total-Limit = %d bytes (%d MB)", limitVal, limitVal/(1024*1024))
			}
		}
	}
	if !foundTotalLimit {
		t.Fatalf("❌ Mikrotik-Total-Limit attribute (VSA 17) NOT found in Access-Accept!")
	}

	// 4. Simulate Accounting: User consumes 300 MB (Interim-Update)
	consumedIn := uint64(200 * 1024 * 1024)
	consumedOut := uint64(100 * 1024 * 1024)
	recordSQLiteAccounting(username, 3, "sess-001", "10.0.0.50", "00:11:22:33:44:55", "127.0.0.1", consumedIn, consumedOut, 600, 0)
	time.Sleep(50 * time.Millisecond)

	var usedIn, usedOut int64
	var qStat string
	_ = db.QueryRow("SELECT used_octets_in, used_octets_out, quota_status FROM radius_user_meta WHERE username=?", username).Scan(&usedIn, &usedOut, &qStat)
	t.Logf("✅ Step 4: Accounting Interim-Update recorded: Download=%d MB, Upload=%d MB (Total Used = %d MB, Status=%s)",
		usedIn/(1024*1024), usedOut/(1024*1024), (usedIn+usedOut)/(1024*1024), qStat)

	// Re-authenticate: remaining bytes should now be 200 MB
	authResp2, _ := handleAuthRequest(context.Background(), req)
	expectedRem := uint32(200 * 1024 * 1024)
	foundRemLimit := false
	for _, attr := range authResp2.Attributes {
		if attr.Type == types.AttrVendorSpecific && len(attr.Value) >= 10 {
			vendorID := binary.BigEndian.Uint32(attr.Value[0:4])
			subType := attr.Value[4]
			if vendorID == 14988 && subType == 17 {
				limitVal := binary.BigEndian.Uint32(attr.Value[6:10])
				if limitVal != expectedRem {
					t.Fatalf("❌ Expected Remaining Total-Limit = %d bytes, got %d", expectedRem, limitVal)
				}
				foundRemLimit = true
				t.Logf("✅ Step 5: Second Auth correctly returned Remaining Mikrotik-Total-Limit = %d bytes (%d MB)", limitVal, limitVal/(1024*1024))
			}
		}
	}
	if !foundRemLimit {
		t.Fatalf("❌ Remaining Total-Limit attribute NOT found!")
	}

	// 5. Simulate Total Exhaustion: User consumes 250 MB more (Total 550 MB > 500 MB)
	consumedIn2 := uint64(350 * 1024 * 1024)
	consumedOut2 := uint64(200 * 1024 * 1024)
	recordSQLiteAccounting(username, 3, "sess-001", "10.0.0.50", "00:11:22:33:44:55", "127.0.0.1", consumedIn2, consumedOut2, 1200, 0)
	time.Sleep(50 * time.Millisecond)

	_ = db.QueryRow("SELECT used_octets_in, used_octets_out, quota_status FROM radius_user_meta WHERE username=?", username).Scan(&usedIn, &usedOut, &qStat)
	if qStat != "depleted" {
		t.Fatalf("❌ Expected quota_status = 'depleted', got: %s", qStat)
	}
	t.Logf("✅ Step 6: Quota exceeded! Database marked quota_status = '%s' (Used = %d MB / %d MB)", qStat, (usedIn+usedOut)/(1024*1024), quotaMB)

	// Auth should now be REJECTED!
	authResp3, _ := handleAuthRequest(context.Background(), req)
	if authResp3.Code != types.AccessReject {
		t.Fatalf("❌ Expected Access-Reject due to depleted quota, got: %v", authResp3.Code)
	}
	t.Logf("✅ Step 7: Auth correctly REJECTED after quota depletion!")

	// 6. Reset Quota
	_, err = db.Exec("UPDATE radius_user_meta SET used_octets_in = 0, used_octets_out = 0, quota_status = 'active' WHERE username=?", username)
	if err != nil {
		t.Fatalf("Failed to reset quota: %v", err)
	}
	t.Logf("✅ Step 8: Executed Reset Quota on user '%s'", username)

	// Re-authenticate after reset: should be accepted with full 500 MB
	authResp4, _ := handleAuthRequest(context.Background(), req)
	if authResp4.Code != types.AccessAccept {
		t.Fatalf("❌ Expected Access-Accept after reset, got: %v", authResp4.Code)
	}
	t.Logf("✅ Step 9: Re-Auth after Quota Reset succeeded with full %d MB restored!", quotaMB)
}
