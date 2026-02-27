package server

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
)

var apiKeyRe = regexp.MustCompile(`^sk_([0-9a-f]{16})_([A-Z2-7]+)$`)

// generateAPIKey returns a new random API key string and its prefix.
// Format: sk_{16-hex-prefix}_{52-char-base32-secret}
func generateAPIKey() (fullKey string, prefix string, err error) {
	prefixBytes := make([]byte, 8)
	if _, err = rand.Read(prefixBytes); err != nil {
		return "", "", err
	}
	secretBytes := make([]byte, 32)
	if _, err = rand.Read(secretBytes); err != nil {
		return "", "", err
	}
	prefix = hex.EncodeToString(prefixBytes)
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secretBytes)
	fullKey = fmt.Sprintf("sk_%s_%s", prefix, secret)
	return fullKey, prefix, nil
}

// encryptAPIKey encrypts a full API key string with AES-256-GCM.
// The output is base64(nonce || ciphertext).
func encryptAPIKey(fullKey string, encKey [32]byte) (string, error) {
	block, err := aes.NewCipher(encKey[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(fullKey), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptAPIKey decrypts a value produced by encryptAPIKey.
func decryptAPIKey(encrypted string, encKey [32]byte) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(encKey[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// matchAPIKey does a constant-time comparison of the provided key against the
// decrypted stored key, preventing timing attacks.
func matchAPIKey(provided, decrypted string) bool {
	return subtle.ConstantTimeCompare([]byte(provided), []byte(decrypted)) == 1
}

// parseAPIKeyPrefix extracts the prefix segment from an API key string.
func parseAPIKeyPrefix(key string) (string, error) {
	matches := apiKeyRe.FindStringSubmatch(key)
	if len(matches) < 2 {
		return "", fmt.Errorf("invalid API key format")
	}
	return matches[1], nil
}
