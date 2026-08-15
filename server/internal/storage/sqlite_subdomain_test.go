package storage

import (
	"os"
	"testing"
)

func TestSubdomainAvailability(t *testing.T) {
	tempDB := "test_subdomain_" + t.Name() + ".db"
	defer os.Remove(tempDB)

	repo, err := NewSQLiteRepository(tempDB)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}
	if err := repo.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// 1. Initial check: 'agent1' should be available
	avail, err := repo.IsSubdomainAvailable("agent1")
	if err != nil {
		t.Fatalf("failed check: %v", err)
	}
	if !avail {
		t.Errorf("expected 'agent1' to be available")
	}

	// 2. Create subdomain 'agent1'
	_, err = repo.CreateOrGetSubdomain("cust-1", "lic-1", "agent1")
	if err != nil {
		t.Fatalf("failed to create subdomain: %v", err)
	}

	// 3. Check again: 'agent1' (case insensitive) should NOT be available
	avail, err = repo.IsSubdomainAvailable("AGENT1")
	if err != nil {
		t.Fatalf("failed check: %v", err)
	}
	if avail {
		t.Errorf("expected 'agent1' to NOT be available")
	}

	// 4. Different subdomain 'agent2' should be available
	avail, err = repo.IsSubdomainAvailable("agent2")
	if err != nil {
		t.Fatalf("failed check: %v", err)
	}
	if !avail {
		t.Errorf("expected 'agent2' to be available")
	}
}
