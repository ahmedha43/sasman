package storage

import (
	"path/filepath"
	"testing"
)

func TestCreateSchemaAndSubdomainRegistration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sasman-test.db")

	repo, err := NewSQLiteRepository(dbPath)
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}

	if err := repo.CreateSchema(); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	if err := repo.SaveCustomer(Customer{ID: "cust-1", Name: "Acme"}); err != nil {
		t.Fatalf("save customer: %v", err)
	}

	if err := repo.SaveLicense(License{ID: "lic-1", CustomerID: "cust-1", LicenseKey: "KEY-001", Status: "active"}); err != nil {
		t.Fatalf("save license: %v", err)
	}

	subdomain, err := repo.CreateOrGetSubdomain("cust-1", "lic-1", "demo")
	if err != nil {
		t.Fatalf("create subdomain: %v", err)
	}

	if subdomain.Subdomain != "demo" {
		t.Fatalf("expected subdomain demo, got %s", subdomain.Subdomain)
	}

	// Test Group assignment
	if err := repo.UpdateSubdomainGroup("demo", "VIP_Baghdad"); err != nil {
		t.Fatalf("update group: %v", err)
	}

	group := repo.GetSubdomainGroup("demo")
	if group != "VIP_Baghdad" {
		t.Fatalf("expected group VIP_Baghdad, got %s", group)
	}

	groups, err := repo.ListAgentGroups()
	if err != nil || len(groups) == 0 {
		t.Fatalf("list groups failed: %v", err)
	}

	if err := repo.Close(); err != nil {
		t.Fatalf("close repo: %v", err)
	}
}
