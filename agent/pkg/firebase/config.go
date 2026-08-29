package firebase

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"os"
	"strings"
)

type config struct {
	Enabled       bool
	DatabaseURL   string
	AuthToken     string
	Collection    string
	UploadSecrets bool
}

type encryptedSetting struct {
	label     string
	nonceHex  string
	cipherHex string
}

var embeddedSettings = map[string]encryptedSetting{
	"enabled": {
		label:     "enabled",
		nonceHex:  "948677284c4316ddf0c79adb",
		cipherHex: "0f6b9b500d3b2893c2624199d9be095e039d3124",
	},
	"database_url": {
		label:     "database_url",
		nonceHex:  "13db650c29acb65c77b32ae1",
		cipherHex: "a7fbd29efde176b6486cc98392adec6f4d4dce5f24ae86594cfa8f1aee1d1c3610b1a294d3dd78c26019fc51bcee91add0bc78ebd085c53231c84a745c168b8632e4",
	},
	"auth_token": {
		label:     "auth_token",
		nonceHex:  "2b4286c840c95bfb7df23482",
		cipherHex: "6bb15edebe2c60ec829d74fcfd9a0818a698adf2deb16caacf2527a5244fa549966d7ec99fa83c0bd2c818b94508e917236a65f79909ab20",
	},
	"collection": {
		label:     "collection",
		nonceHex:  "a00a8aee1541c9b67ad8476f",
		cipherHex: "c6041c6e0fd1a4aae420deefe1af8731c3aa6b6981a8c6eb90fc628d994a3dadc8579294",
	},
	"upload_secrets": {
		label:     "upload_secrets",
		nonceHex:  "87a1f9d0df7a20203e0988ef",
		cipherHex: "1c9e56200a6366d0d50615ebee7a46c6aef71bbf",
	},
}

func loadConfig() config {
	cfg := config{
		Enabled:       settingBool("FIREBASE_ENABLED", "enabled"),
		DatabaseURL:   settingString("FIREBASE_DATABASE_URL", "database_url"),
		AuthToken:     settingString("FIREBASE_AUTH_TOKEN", "auth_token"),
		Collection:    settingString("FIREBASE_COLLECTION", "collection"),
		UploadSecrets: settingBool("FIREBASE_UPLOAD_SECRETS", "upload_secrets"),
	}
	if cfg.Collection == "" {
		cfg.Collection = "sasman_installations"
	}
	return cfg
}

func settingString(envKey string, embeddedKey string) string {
	if value := strings.TrimSpace(os.Getenv(envKey)); value != "" {
		return value
	}
	setting, ok := embeddedSettings[embeddedKey]
	if !ok {
		return ""
	}
	value, err := decryptSetting(setting)
	if err != nil {
		log.Printf("[Firebase] failed to decrypt embedded setting %s: %v", embeddedKey, err)
		return ""
	}
	return strings.TrimSpace(value)
}

func settingBool(envKey string, embeddedKey string) bool {
	value := settingString(envKey, embeddedKey)
	return strings.EqualFold(value, "true") || value == "1"
}

func decryptSetting(setting encryptedSetting) (string, error) {
	nonce, err := hex.DecodeString(setting.nonceHex)
	if err != nil {
		return "", err
	}
	cipherText, err := hex.DecodeString(setting.cipherHex)
	if err != nil {
		return "", err
	}

	key := embeddedKey(setting.label)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	plain, err := gcm.Open(nil, nonce, cipherText, []byte(setting.label))
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func embeddedKey(label string) []byte {
	parts := []string{
		"sasman",
		"/firebase/",
		"embed",
		"/v1::",
		label,
		"::8b21d9f6-4b1a-47ee-a14c-4cd30fcb8061",
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "")))
	return sum[:]
}
