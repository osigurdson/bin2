package registrystorage

import "context"

// BlobDeleter is the narrow storage interface required by GC.
// R2 DeleteObject is idempotent: deleting a key that does not exist returns
// success, so callers do not need to handle a "not found" error.
type BlobDeleter interface {
	DeleteBlob(ctx context.Context, digestHex string) error
}
