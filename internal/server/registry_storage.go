package server

import (
	"context"
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

type registryStorage interface {
	Init() error
	CreateUpload(ctx context.Context, uuid string) error
	AppendUpload(ctx context.Context, uuid string, body io.Reader) (int64, error)
	UploadSHA256(ctx context.Context, uuid string) (string, error)
	DeleteUpload(ctx context.Context, uuid string) error
	BlobExists(ctx context.Context, digestHex string) (bool, error)
	BlobSize(ctx context.Context, digestHex string) (int64, error)
	GetBlob(ctx context.Context, digestHex string) (io.ReadCloser, int64, error)
	StoreBlobFromUpload(ctx context.Context, uuid, digestHex string) error
	StoreManifest(ctx context.Context, repo, reference string, manifest []byte, contentType string) error
	GetManifest(ctx context.Context, repo, reference string) ([]byte, string, error)
}

func newRegistryStorageFromEnv() (registryStorage, error) {
	backend := strings.ToLower(strings.TrimSpace(getenvDefault("REGISTRY_STORAGE_BACKEND", "local")))
	dataDir := getenvDefault("REGISTRY_DATA_DIR", "registry-data")

	switch backend {
	case "r2":
		return newR2RegistryStorageFromEnv(dataDir)
	case "local":
		return newLocalRegistryStorage(dataDir), nil
	default:
		return nil, fmt.Errorf(
			"unsupported REGISTRY_STORAGE_BACKEND=%q (expected: r2|local)",
			backend,
		)
	}
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
