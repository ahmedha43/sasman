package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSubdomainTakeoverWorkflow(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sasman-takeover-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	repo, err := NewSQLiteRepository(dbPath)
	if err != nil {
		t.Fatalf("Failed to create repo: %v", err)
	}
	defer repo.Close()

	if err := repo.CreateSchema(); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// 1. Create initial customer and subdomain
	initialCust := Customer{
		ID:          "cust-original",
		Name:        "المالك القديم",
		Phone:       "07701112233",
		CompanyName: "testnode",
		Status:      "active",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := repo.SaveCustomer(initialCust); err != nil {
		t.Fatalf("Failed to save initial customer: %v", err)
	}

	initialSub := Subdomain{
		ID:         "sub-1",
		CustomerID: "cust-original",
		LicenseID:  "lic-1",
		Subdomain:  "testnode",
		ZoneName:   "sas-man.net",
		Status:     "active",
		Token:      "old-token-123",
		WinboxPort: 18291,
		GroupName:  "default",
		AssignedAt: time.Now().UTC(),
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := repo.SaveSubdomain(initialSub); err != nil {
		t.Fatalf("Failed to save initial subdomain: %v", err)
	}

	// Verify it's not available
	avail, err := repo.IsSubdomainAvailable("testnode")
	if err != nil || avail {
		t.Fatalf("Expected testnode to be taken, got avail=%v, err=%v", avail, err)
	}

	// Verify owner info
	ownerName, ownerPhone, err := repo.GetSubdomainOwnerInfo("testnode")
	if err != nil || ownerName != "المالك القديم" || ownerPhone != "07701112233" {
		t.Fatalf("Expected owner info, got name=%s, phone=%s, err=%v", ownerName, ownerPhone, err)
	}

	// 2. Submit Takeover Request
	takeoverReq := SubdomainTakeoverRequest{
		ID:                "req-1001",
		Subdomain:         "testnode",
		RequesterName:     "المشتري الجديد",
		RequesterPhone:    "07709998877",
		RequesterSerial:   "R12345678",
		RequesterNotes:    "تم شراء الراوتر من المشترك السابق",
		RequesterAgentID:  "192.168.1.50",
		CurrentOwnerName:  ownerName,
		CurrentOwnerPhone: ownerPhone,
		Status:            "pending",
		RequestedAt:       time.Now().UTC(),
	}
	if err := repo.CreateTakeoverRequest(takeoverReq); err != nil {
		t.Fatalf("Failed to create takeover request: %v", err)
	}

	// Verify pending count
	count, err := repo.GetPendingTakeoverCount()
	if err != nil || count != 1 {
		t.Fatalf("Expected pending count 1, got %d, err=%v", count, err)
	}

	// Verify list
	pendingList, err := repo.GetTakeoverRequests("pending")
	if err != nil || len(pendingList) != 1 {
		t.Fatalf("Expected 1 pending request in list, got %d", len(pendingList))
	}
	if pendingList[0].RequesterName != "المشتري الجديد" {
		t.Fatalf("Expected requester name 'المشتري الجديد', got '%s'", pendingList[0].RequesterName)
	}

	// Verify CheckTakeoverStatus
	statusCheck, err := repo.CheckTakeoverStatus("testnode", "07709998877")
	if err != nil || statusCheck == nil || statusCheck.Status != "pending" {
		t.Fatalf("Expected pending status, got %v, err=%v", statusCheck, err)
	}

	// 3. Approve Takeover Request
	approvedSub, err := repo.ApproveTakeoverRequest("req-1001", "تم التحقق والموافقة")
	if err != nil {
		t.Fatalf("Failed to approve takeover request: %v", err)
	}
	if approvedSub.Token == "old-token-123" {
		t.Fatalf("Expected new rotated token upon approval, but got old token")
	}

	// Verify pending count is now 0
	count, _ = repo.GetPendingTakeoverCount()
	if count != 0 {
		t.Fatalf("Expected pending count 0 after approval, got %d", count)
	}

	// Verify CheckTakeoverStatus returns approved
	statusCheck, err = repo.CheckTakeoverStatus("testnode", "07709998877")
	if err != nil || statusCheck == nil || statusCheck.Status != "approved" {
		t.Fatalf("Expected approved status, got %v, err=%v", statusCheck, err)
	}

	// Verify new owner info
	newOwnerName, newOwnerPhone, err := repo.GetSubdomainOwnerInfo("testnode")
	if err != nil || newOwnerName != "المشتري الجديد" || newOwnerPhone != "07709998877" {
		t.Fatalf("Expected updated owner info, got name=%s, phone=%s", newOwnerName, newOwnerPhone)
	}
}
