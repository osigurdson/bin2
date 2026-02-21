package server

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type localRegistryStorage struct {
	dataDir string
}

func newLocalRegistryStorage(dataDir string) registryStorage {
	return &localRegistryStorage{dataDir: dataDir}
}

func (l *localRegistryStorage) Init() error {
	dirs := []string{
		filepath.Join(l.dataDir, "blobs", "sha256"),
		filepath.Join(l.dataDir, "uploads"),
		filepath.Join(l.dataDir, "repositories"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (l *localRegistryStorage) CreateUpload(_ context.Context, uuid string) error {
	path := l.uploadPath(uuid)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}

func (l *localRegistryStorage) AppendUpload(_ context.Context, uuid string, body io.Reader) (int64, error) {
	path := l.uploadPath(uuid)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if errors.Is(err, os.ErrNotExist) {
		return 0, ErrUploadNotFound
	}
	if err != nil {
		return 0, err
	}
	defer f.Close()

	if _, err := io.Copy(f, body); err != nil {
		return 0, err
	}
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (l *localRegistryStorage) UploadSHA256(_ context.Context, uuid string) (string, error) {
	sum, err := fileSHA256(l.uploadPath(uuid))
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrUploadNotFound
	}
	return sum, err
}

func (l *localRegistryStorage) DeleteUpload(_ context.Context, uuid string) error {
	err := os.Remove(l.uploadPath(uuid))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (l *localRegistryStorage) BlobExists(_ context.Context, digestHex string) (bool, error) {
	_, err := os.Stat(l.blobPath(digestHex))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (l *localRegistryStorage) BlobSize(_ context.Context, digestHex string) (int64, error) {
	info, err := os.Stat(l.blobPath(digestHex))
	if errors.Is(err, os.ErrNotExist) {
		return 0, ErrBlobNotFound
	}
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (l *localRegistryStorage) GetBlob(_ context.Context, digestHex string) (io.ReadCloser, int64, error) {
	f, err := os.Open(l.blobPath(digestHex))
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, ErrBlobNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, err
	}
	return f, info.Size(), nil
}

func (l *localRegistryStorage) StoreBlobFromUpload(_ context.Context, uuid, digestHex string) error {
	src := l.uploadPath(uuid)
	if _, err := os.Stat(src); errors.Is(err, os.ErrNotExist) {
		return ErrUploadNotFound
	}

	dst := l.blobPath(digestHex)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(dst); err == nil {
		return l.DeleteUpload(context.Background(), uuid)
	}
	return moveFile(src, dst)
}

func (l *localRegistryStorage) StoreManifest(_ context.Context, repo, reference string, manifest []byte, contentType string) error {
	path := l.manifestPath(repo, reference)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := writeFileAtomically(path, manifest, 0o644); err != nil {
		return err
	}

	contentTypePath := l.manifestContentTypePath(repo, reference)
	return writeFileAtomically(contentTypePath, []byte(manifestContentType(contentType)+"\n"), 0o644)
}

func (l *localRegistryStorage) GetManifest(_ context.Context, repo, reference string) ([]byte, string, error) {
	path := l.manifestPath(repo, reference)
	manifest, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", ErrManifestNotFound
	}
	if err != nil {
		return nil, "", err
	}

	contentType := defaultManifestContentType
	contentTypePath := l.manifestContentTypePath(repo, reference)
	if data, err := os.ReadFile(contentTypePath); err == nil {
		contentType = manifestContentType(strings.TrimSpace(string(data)))
	}

	return manifest, contentType, nil
}

func (l *localRegistryStorage) uploadPath(uuid string) string {
	return filepath.Join(l.dataDir, "uploads", uuid)
}

func (l *localRegistryStorage) blobPath(digestHex string) string {
	return filepath.Join(l.dataDir, blobObjectKey(digestHex))
}

func (l *localRegistryStorage) manifestPath(repo, reference string) string {
	return filepath.Join(l.dataDir, filepath.FromSlash(manifestObjectKey(repo, reference)))
}

func (l *localRegistryStorage) manifestContentTypePath(repo, reference string) string {
	return l.manifestPath(repo, reference) + ".content-type"
}

func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(src)
}

func writeFileAtomically(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-manifest-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
