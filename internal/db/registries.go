package db

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Registry struct {
	ID                  uuid.UUID
	OrgID               uuid.UUID
	Name                string
	CachedSizeBytes     int64
	CachedSizeUpdatedAt *time.Time
}

type AddRegistryArgs struct {
	OrgID uuid.UUID
	Name  string
}

func (d *DB) AddRegistry(ctx context.Context, args AddRegistryArgs) (Registry, error) {
	registry := Registry{
		ID:    uuid.New(),
		OrgID: args.OrgID,
		Name:  args.Name,
	}

	const cmd = `INSERT INTO registries (id, org_id, name)
		VALUES ($1, $2, $3)`
	if _, err := d.conn.Exec(ctx, cmd, registry.ID, registry.OrgID, registry.Name); err != nil {
		if isUniqueViolation(err) {
			return Registry{}, ErrConflict
		}
		return Registry{}, err
	}

	return registry, nil
}

type AddRegistryWithKeyArgs struct {
	OrgID           uuid.UUID
	Name            string
	UserID          uuid.UUID
	KeyName         string
	SecretEncrypted string
	Prefix          string
}

type AddRegistryWithKeyResult struct {
	Registry Registry
	APIKey   APIKey
}

// AddRegistryWithKey creates a registry and a default admin-scoped API key
// atomically. The key is granted registry-wide admin permission.
func (d *DB) AddRegistryWithKey(ctx context.Context, args AddRegistryWithKeyArgs) (AddRegistryWithKeyResult, error) {
	tx, err := d.conn.Begin(ctx)
	if err != nil {
		return AddRegistryWithKeyResult{}, err
	}
	defer tx.Rollback(ctx)

	registry := Registry{
		ID:    uuid.New(),
		OrgID: args.OrgID,
		Name:  args.Name,
	}
	const insertRegistryCmd = `INSERT INTO registries (id, org_id, name) VALUES ($1, $2, $3)`
	if _, err := tx.Exec(ctx, insertRegistryCmd, registry.ID, registry.OrgID, registry.Name); err != nil {
		if isUniqueViolation(err) {
			return AddRegistryWithKeyResult{}, ErrConflict
		}
		return AddRegistryWithKeyResult{}, err
	}

	apiKey := APIKey{
		ID:              uuid.New(),
		UserID:          args.UserID,
		KeyName:         args.KeyName,
		Prefix:          args.Prefix,
		SecretEncrypted: args.SecretEncrypted,
		Scopes:          make([]APIKeyScope, 0, 1),
	}
	const insertKeyCmd = `INSERT INTO api_keys (id, user_id, name, secret_encrypted, prefix)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at`
	if err := tx.QueryRow(ctx, insertKeyCmd,
		apiKey.ID, apiKey.UserID, apiKey.KeyName, apiKey.SecretEncrypted, apiKey.Prefix,
	).Scan(&apiKey.CreatedAt); err != nil {
		return AddRegistryWithKeyResult{}, err
	}

	scope := APIKeyScope{
		ID:         uuid.New(),
		APIKeyID:   apiKey.ID,
		RegistryID: registry.ID,
		Permission: APIKeyPermissionAdmin,
	}
	const insertScopeCmd = `INSERT INTO api_key_scopes (id, api_key_id, registry_id, repository_id, permission)
		VALUES ($1, $2, $3, NULL, $4)
		RETURNING created_at`
	if err := tx.QueryRow(ctx, insertScopeCmd,
		scope.ID, scope.APIKeyID, scope.RegistryID, scope.Permission,
	).Scan(&scope.CreatedAt); err != nil {
		return AddRegistryWithKeyResult{}, err
	}
	apiKey.Scopes = append(apiKey.Scopes, scope)

	if err := tx.Commit(ctx); err != nil {
		return AddRegistryWithKeyResult{}, err
	}
	return AddRegistryWithKeyResult{Registry: registry, APIKey: apiKey}, nil
}

func (d *DB) ListRegistriesByOrg(ctx context.Context, orgID uuid.UUID) ([]Registry, error) {
	const cmd = `SELECT id, org_id, name, cached_size_bytes, cached_size_updated_at
		FROM registries
		WHERE org_id = $1
		ORDER BY name ASC`
	rows, err := d.conn.Query(ctx, cmd, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	registries := make([]Registry, 0)
	for rows.Next() {
		var registry Registry
		if err := rows.Scan(
			&registry.ID,
			&registry.OrgID,
			&registry.Name,
			&registry.CachedSizeBytes,
			&registry.CachedSizeUpdatedAt,
		); err != nil {
			return nil, err
		}
		registries = append(registries, registry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return registries, nil
}

func (d *DB) GetRegistryByID(ctx context.Context, id uuid.UUID) (Registry, error) {
	const cmd = `SELECT id, org_id, name, cached_size_bytes, cached_size_updated_at
		FROM registries
		WHERE id = $1`
	var registry Registry
	err := d.conn.QueryRow(ctx, cmd, id).Scan(
		&registry.ID,
		&registry.OrgID,
		&registry.Name,
		&registry.CachedSizeBytes,
		&registry.CachedSizeUpdatedAt,
	)
	if err != nil {
		if isNoRows(err) {
			return Registry{}, ErrNotFound
		}
		return Registry{}, err
	}
	return registry, nil
}

func (d *DB) GetRegistryByName(ctx context.Context, name string) (Registry, error) {
	const cmd = `SELECT id, org_id, name, cached_size_bytes, cached_size_updated_at
		FROM registries
		WHERE name = $1`
	var registry Registry
	err := d.conn.QueryRow(ctx, cmd, name).Scan(
		&registry.ID,
		&registry.OrgID,
		&registry.Name,
		&registry.CachedSizeBytes,
		&registry.CachedSizeUpdatedAt,
	)
	if err != nil {
		if isNoRows(err) {
			return Registry{}, ErrNotFound
		}
		return Registry{}, err
	}
	return registry, nil
}

func (d *DB) GetRegistryReferencedBlobBytesCached(ctx context.Context, registryID uuid.UUID, maxAge time.Duration) (int64, error) {
	if maxAge <= 0 {
		maxAge = 60 * time.Second
	}

	tx, err := d.conn.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var cachedSizeBytes int64
	var cachedSizeUpdatedAt *time.Time
	const selectRegistryCmd = `SELECT cached_size_bytes, cached_size_updated_at
		FROM registries
		WHERE id = $1
		FOR UPDATE`
	if err := tx.QueryRow(ctx, selectRegistryCmd, registryID).Scan(&cachedSizeBytes, &cachedSizeUpdatedAt); err != nil {
		if isNoRows(err) {
			return 0, ErrNotFound
		}
		return 0, err
	}

	if cachedSizeUpdatedAt != nil && time.Since(cachedSizeUpdatedAt.UTC()) < maxAge {
		if err := tx.Commit(ctx); err != nil {
			return 0, err
		}
		return cachedSizeBytes, nil
	}

	const recomputeCmd = `SELECT COALESCE(SUM(b.size_bytes), 0)
		FROM (
			SELECT DISTINCT mb.blob_digest
			FROM repositories r
			JOIN manifest_refs mr
			  ON mr.repository_id = r.id
			JOIN manifest_blob_refs mb
			  ON mb.repository_id = mr.repository_id
			 AND mb.manifest_digest = mr.manifest_digest
			WHERE r.registry_id = $1
		) referenced
		JOIN blobs b
		  ON b.digest = referenced.blob_digest`

	var computedSizeBytes int64
	if err := tx.QueryRow(ctx, recomputeCmd, registryID).Scan(&computedSizeBytes); err != nil {
		return 0, err
	}

	const updateCacheCmd = `UPDATE registries
		SET cached_size_bytes = $2,
		    cached_size_updated_at = NOW()
		WHERE id = $1`
	if _, err := tx.Exec(ctx, updateCacheCmd, registryID, computedSizeBytes); err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return computedSizeBytes, nil
}
