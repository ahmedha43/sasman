package ota

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestCryptoSigningAndVerification(t *testing.T) {
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	pubHex := hex.EncodeToString(pub)

	// Create a dummy binary file
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "agent_bin")
	content := []byte("SIMULATED_SASMAN_AGENT_BINARY_V5.1.0")
	if err := os.WriteFile(binaryPath, content, 0755); err != nil {
		t.Fatalf("Failed to write dummy binary: %v", err)
	}

	shaHex, err := CalculateFileSHA256(binaryPath)
	if err != nil {
		t.Fatalf("Failed to calculate SHA256: %v", err)
	}

	sigHex, err := SignSHA256(priv, shaHex)
	if err != nil {
		t.Fatalf("Failed to sign SHA256: %v", err)
	}

	manifest := ReleaseManifest{
		Version:          "5.1.0",
		Sha256:           shaHex,
		SignatureEd25519: sigHex,
	}

	// 1. Valid binary should pass verification
	if err := VerifyBinaryAgainstManifest(binaryPath, manifest, pubHex); err != nil {
		t.Errorf("Expected valid verification, got error: %v", err)
	}

	// 2. Tampered binary should fail hash check
	tamperedPath := filepath.Join(tmpDir, "tampered_bin")
	_ = os.WriteFile(tamperedPath, []byte("MALICIOUS_TAMPERED_CODE"), 0755)
	if err := VerifyBinaryAgainstManifest(tamperedPath, manifest, pubHex); err == nil {
		t.Errorf("Expected error on tampered binary, but verification passed!")
	}

	// 3. Forged signature should fail signature verification
	forgedManifest := manifest
	forgedManifest.SignatureEd25519 = hex.EncodeToString(make([]byte, 64))
	if err := VerifyBinaryAgainstManifest(binaryPath, forgedManifest, pubHex); err == nil {
		t.Errorf("Expected error on forged signature, but verification passed!")
	}
}
