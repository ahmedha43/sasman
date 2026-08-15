package supervisor

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mikrotik-manager/pkg/ota"
)

func TestSupervisorAtomicUpgradeAndRollback(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "app")
	dataDir := filepath.Join(appDir, "data")
	releasesDir := filepath.Join(appDir, "releases")
	_ = os.MkdirAll(dataDir, 0755)
	_ = os.MkdirAll(releasesDir, 0755)

	agentBin := filepath.Join(appDir, "agent")
	// Write dummy v5.0 binary
	_ = os.WriteFile(agentBin, []byte("BINARY_V5.0"), 0755)

	// Create test keys
	pub, priv, err := ota.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	// Prepare new v5.1 binary content
	v51Content := []byte("BINARY_V5.1_NEW_FEATURE")
	shaHex := ota.CalculateSHA256(v51Content)
	sigHex, _ := ota.SignSHA256(priv, shaHex)

	// Mock HTTP Server for binary download
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(v51Content)
	}))
	defer server.Close()

	manifest := ota.ReleaseManifest{
		Version:          "5.1.0",
		Sha256:           shaHex,
		SignatureEd25519: sigHex,
		BinaryURL:        server.URL + "/sasman-agent",
	}

	cfg := Config{
		AppDir:          appDir,
		AgentBinary:     agentBin,
		DataDir:         dataDir,
		ReleasesDir:     releasesDir,
		WatchdogTimeout: 2 * time.Second,
		PublicKeyHex:    hex.EncodeToString(pub),
	}

	sup := NewSupervisor(cfg)

	// Test ExecuteUpgrade
	if err := sup.ExecuteUpgrade(manifest); err != nil {
		t.Fatalf("ExecuteUpgrade failed: %v", err)
	}

	// Verify agent binary was updated to v5.1 content
	updatedContent, err := os.ReadFile(agentBin)
	if err != nil {
		t.Fatalf("ReadFile updated agent: %v", err)
	}
	if string(updatedContent) != string(v51Content) {
		t.Errorf("Expected %s, got %s", string(v51Content), string(updatedContent))
	}

	// Verify backup binary was created with v5.0 content
	backupBin := filepath.Join(releasesDir, "agent.prev")
	backupContent, err := os.ReadFile(backupBin)
	if err != nil {
		t.Fatalf("ReadFile backup: %v", err)
	}
	if string(backupContent) != "BINARY_V5.0" {
		t.Errorf("Expected BINARY_V5.0, got %s", string(backupContent))
	}

	// Test Rollback
	sup.performRollback()
	restoredContent, err := os.ReadFile(agentBin)
	if err != nil {
		t.Fatalf("ReadFile restored agent: %v", err)
	}
	if string(restoredContent) != "BINARY_V5.0" {
		t.Errorf("Expected BINARY_V5.0 after rollback, got %s", string(restoredContent))
	}

	// Verify status file
	statusFile := filepath.Join(dataDir, "ota_status.json")
	statusData, err := os.ReadFile(statusFile)
	if err != nil {
		t.Fatalf("ReadFile status file: %v", err)
	}
	var st ota.AgentOTAStatus
	if err := json.Unmarshal(statusData, &st); err != nil {
		t.Fatalf("Unmarshal status: %v", err)
	}
	if st.Status != "rollback" {
		t.Errorf("Expected status 'rollback', got %q", st.Status)
	}
}

func pubHex(pub []byte) string {
	return hex.EncodeToString(pub)
}
