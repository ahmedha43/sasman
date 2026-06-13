package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
)

func main() {
	enabled := flag.String("enabled", "true", "Firebase enabled flag")
	databaseURL := flag.String("database-url", "", "Firebase Realtime Database URL")
	authToken := flag.String("auth-token", "", "Firebase auth token")
	collection := flag.String("collection", "sasman_installations", "Firebase collection")
	uploadSecrets := flag.String("upload-secrets", "false", "Upload raw license/password")
	flag.Parse()

	values := map[string]string{
		"enabled":        *enabled,
		"database_url":   *databaseURL,
		"auth_token":     *authToken,
		"collection":     *collection,
		"upload_secrets": *uploadSecrets,
	}

	for _, label := range []string{"enabled", "database_url", "auth_token", "collection", "upload_secrets"} {
		nonce, cipherText, err := encrypt(label, values[label])
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s:\n  nonceHex:  %s\n  cipherHex: %s\n\n", label, nonce, cipherText)
	}
}

func encrypt(label, value string) (string, string, error) {
	key := embeddedKey(label)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}

	nonceSeed := sha256.Sum256([]byte("nonce::" + label + "::" + value))
	nonce := nonceSeed[:gcm.NonceSize()]
	cipherText := gcm.Seal(nil, nonce, []byte(value), []byte(label))
	return hex.EncodeToString(nonce), hex.EncodeToString(cipherText), nil
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
	sum := sha256.Sum256([]byte(join(parts)))
	return sum[:]
}

func join(parts []string) string {
	out := ""
	for _, part := range parts {
		out += part
	}
	return out
}
