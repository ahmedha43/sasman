package ota

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// Default embedded Master Public Key for SASMAN OTA verification (Hex encoded)
// This can be overridden via SASMAN_OTA_PUBKEY env var
const DefaultMasterPublicKeyHex = "6f8c40758414434220b33a59828e1d2de22bc13d5a4a584061a9c3629c4146a8"

// CalculateSHA256 computes the hex-encoded SHA-256 hash of bytes
func CalculateSHA256(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// CalculateFileSHA256 computes the hex-encoded SHA-256 hash of a file
func CalculateFileSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file for sha256: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("read file for sha256: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// GenerateKeyPair generates a new Ed25519 public/private keypair
func GenerateKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate ed25519 key: %w", err)
	}
	return pub, priv, nil
}

// SignSHA256 signs a SHA256 hex string using an Ed25519 private key
func SignSHA256(privKey ed25519.PrivateKey, sha256Hex string) (string, error) {
	if len(privKey) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("invalid private key size")
	}
	sig := ed25519.Sign(privKey, []byte(sha256Hex))
	return hex.EncodeToString(sig), nil
}

// VerifySignature verifies that the signature matches the sha256Hex using the public key
func VerifySignature(pubKeyHex string, sha256Hex string, sigHex string) (bool, error) {
	pubKeyBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return false, fmt.Errorf("decode public key hex: %w", err)
	}
	if len(pubKeyBytes) != ed25519.PublicKeySize {
		return false, fmt.Errorf("invalid public key size: %d", len(pubKeyBytes))
	}

	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		return false, fmt.Errorf("decode signature hex: %w", err)
	}
	if len(sigBytes) != ed25519.SignatureSize {
		return false, fmt.Errorf("invalid signature size: %d", len(sigBytes))
	}

	valid := ed25519.Verify(pubKeyBytes, []byte(sha256Hex), sigBytes)
	return valid, nil
}

// VerifyBinaryAgainstManifest verifies both the SHA-256 hash and the Ed25519 signature
func VerifyBinaryAgainstManifest(filePath string, manifest ReleaseManifest, pubKeyHex string) error {
	// 1. Verify File Hash
	actualHash, err := CalculateFileSHA256(filePath)
	if err != nil {
		return fmt.Errorf("calculate binary hash: %w", err)
	}

	if actualHash != manifest.Sha256 {
		return fmt.Errorf("hash mismatch: expected %s, got %s", manifest.Sha256, actualHash)
	}

	// 2. Verify Cryptographic Signature
	if manifest.SignatureEd25519 != "" {
		if pubKeyHex == "" {
			pubKeyHex = DefaultMasterPublicKeyHex
		}
		valid, err := VerifySignature(pubKeyHex, manifest.Sha256, manifest.SignatureEd25519)
		if err != nil {
			return fmt.Errorf("verify signature: %w", err)
		}
		if !valid {
			return fmt.Errorf("invalid cryptographic signature: binary rejected")
		}
	}

	return nil
}
