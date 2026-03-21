package server

import (
	"context"

	"bin2.io/internal/db"
	"github.com/google/uuid"
)

// registryDatabase is the subset of db.DB used by the OCI registry v2 handlers.
// It is defined as an interface so tests can provide an in-memory implementation
// without a real Postgres connection.
type registryDatabase interface {
	GetRegistryByName(ctx context.Context, name string) (db.Registry, error)
	GetRegistryTenantID(ctx context.Context, registryID uuid.UUID) (uuid.UUID, error)
	TenantHasBlob(ctx context.Context, tenantID uuid.UUID, digest string) (bool, error)
	GetRepositoryObjectSize(ctx context.Context, registryID uuid.UUID, repository, digest string) (int64, error)
	UpsertObjectBlob(ctx context.Context, digest string, sizeBytes int64) error
	GetObjectSize(ctx context.Context, digest string) (int64, error)
	NoteObjectExistenceCheck(ctx context.Context, digest string) error
	InsertUsageEvents(ctx context.Context, events []db.UsageEvent) error
	UpsertManifest(ctx context.Context, args db.UpsertManifestArgs) error
	HasManifestDigestInRepository(ctx context.Context, registryID uuid.UUID, repository, manifestDigest string) (bool, error)
	GetManifestByReference(ctx context.Context, registryID uuid.UUID, repository, reference string) ([]byte, string, string, error)
	DeleteManifestByDigestInRepository(ctx context.Context, registryID uuid.UUID, tenantID uuid.UUID, repository, manifestDigest string) (bool, []db.DeletedBlobInfo, error)
	DeleteManifestReference(ctx context.Context, registryID uuid.UUID, repository, reference string) (bool, error)
	ListRepositoryManifestRecords(ctx context.Context, registryID uuid.UUID, repository string) ([]db.RepositoryManifestRecord, error)
	ListRepositoryTags(ctx context.Context, registryID uuid.UUID, repository string, limit int, last string) ([]string, error)
}

// Compile-time check: *db.DB satisfies registryDatabase.
var _ registryDatabase = (*db.DB)(nil)
