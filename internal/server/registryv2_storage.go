package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

var (
	ErrUploadNotFound   = errors.New("upload not found")
	ErrBlobNotFound     = errors.New("blob not found")
	ErrManifestNotFound = errors.New("manifest not found")
)

func newRegistryStorageFromEnv() (*r2RegistryStorage, error) {
	dataDir := getenvDefault("REGISTRY_DATA_DIR", "registry-data")
	return newR2RegistryStorageFromEnv(dataDir)
}

func newUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4],
		b[4:6],
		b[6:8],
		b[8:10],
		b[10:16],
	), nil
}

func fileSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func blobObjectKey(digestHex string) string {
	return path.Join("blobs", "sha256", digestHex[:2], digestHex)
}

func manifestObjectKey(repo, reference string) string {
	return path.Join("repositories", repo, "manifests", reference+".json")
}

func getenvDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
