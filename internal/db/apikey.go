package db

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type APIKeyPermission string

const (
	APIKeyPermissionRead  APIKeyPermission = "read"
	APIKeyPermissionWrite APIKeyPermission = "write"
	APIKeyPermissionAdmin APIKeyPermission = "admin"
)

type APIKeyScope struct {
	ID         uuid.UUID
	APIKeyID   uuid.UUID
	RegistryID uuid.UUID
	Repository *string
	Permission APIKeyPermission
	CreatedAt  time.Time
}

type APIKey struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	KeyName          string
	Prefix           string
	SecretEncrypted  string
	CreatedAt        time.Time
	LastUsedAt       *time.Time
	Scopes           []APIKeyScope
}

type AddAPIKeyScopeInput struct {
	RegistryID uuid.UUID
	Repository *string
	Permission APIKeyPermission
}

type AddAPIKeyArgs struct {
	UserID          uuid.UUID
	KeyName         string
	SecretEncrypted string
	Prefix          string
	Scopes          []AddAPIKeyScopeInput
}

func (d *DB) AddAPIKey(ctx context.Context, args AddAPIKeyArgs) (APIKey, error) {
	tx, err := d.conn.Begin(ctx)
	if err != nil {
		return APIKey{}, err
	}
	defer tx.Rollback(ctx)

	apiKey := APIKey{
		ID:              uuid.New(),
		UserID:          args.UserID,
		KeyName:         args.KeyName,
		Prefix:          args.Prefix,
		SecretEncrypted: args.SecretEncrypted,
		Scopes:          make([]APIKeyScope, 0, len(args.Scopes)),
	}

	const insertKeyCmd = `INSERT INTO api_keys (id, user_id, name, secret_encrypted, prefix)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at`
	err = tx.QueryRow(
		ctx,
		insertKeyCmd,
		apiKey.ID,
		apiKey.UserID,
		apiKey.KeyName,
		apiKey.SecretEncrypted,
		apiKey.Prefix,
	).Scan(&apiKey.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return APIKey{}, ErrConflict
		}
		return APIKey{}, err
	}

	const insertScopeCmd = `INSERT INTO api_key_scopes (id, api_key_id, registry_id, repository, permission)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at`
	for _, scope := range args.Scopes {
		apiKeyScope := APIKeyScope{
			ID:         uuid.New(),
			APIKeyID:   apiKey.ID,
			RegistryID: scope.RegistryID,
			Repository: scope.Repository,
			Permission: scope.Permission,
		}
		err = tx.QueryRow(
			ctx,
			insertScopeCmd,
			apiKeyScope.ID,
			apiKeyScope.APIKeyID,
			apiKeyScope.RegistryID,
			apiKeyScope.Repository,
			apiKeyScope.Permission,
		).Scan(&apiKeyScope.CreatedAt)
		if err != nil {
			if isUniqueViolation(err) {
				return APIKey{}, ErrScopeConflict
			}
			return APIKey{}, err
		}
		apiKey.Scopes = append(apiKey.Scopes, apiKeyScope)
	}

	if err := tx.Commit(ctx); err != nil {
		return APIKey{}, err
	}
	return apiKey, nil
}

func (d *DB) ListAPIKeysByUser(ctx context.Context, userID uuid.UUID) ([]APIKey, error) {
	const cmd = `SELECT id, user_id, name, secret_encrypted, prefix, created_at, last_used_at
		FROM api_keys
		WHERE user_id = $1
		ORDER BY created_at DESC`
	rows, err := d.conn.Query(ctx, cmd, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]APIKey, 0)
	for rows.Next() {
		var key APIKey
		if err := rows.Scan(
			&key.ID,
			&key.UserID,
			&key.KeyName,
			&key.SecretEncrypted,
			&key.Prefix,
			&key.CreatedAt,
			&key.LastUsedAt,
		); err != nil {
			return nil, err
		}
		key.Scopes, err = d.ListAPIKeyScopesByAPIKeyID(ctx, key.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, key)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (d *DB) GetAPIKeyByPrefix(ctx context.Context, prefix string) (APIKey, error) {
	const cmd = `SELECT id, user_id, name, secret_encrypted, prefix, created_at, last_used_at
		FROM api_keys
		WHERE prefix = $1`
	var apiKey APIKey
	err := d.conn.QueryRow(ctx, cmd, prefix).Scan(
		&apiKey.ID,
		&apiKey.UserID,
		&apiKey.KeyName,
		&apiKey.SecretEncrypted,
		&apiKey.Prefix,
		&apiKey.CreatedAt,
		&apiKey.LastUsedAt,
	)
	if err != nil {
		if isNoRows(err) {
			return APIKey{}, ErrNotFound
		}
		return APIKey{}, err
	}
	return apiKey, nil
}

func (d *DB) ListAPIKeyScopesByAPIKeyID(ctx context.Context, apiKeyID uuid.UUID) ([]APIKeyScope, error) {
	const cmd = `SELECT id, api_key_id, registry_id, repository, permission, created_at
		FROM api_key_scopes
		WHERE api_key_id = $1
		ORDER BY created_at ASC`
	rows, err := d.conn.Query(ctx, cmd, apiKeyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	scopes := make([]APIKeyScope, 0)
	for rows.Next() {
		var scope APIKeyScope
		if err := rows.Scan(
			&scope.ID,
			&scope.APIKeyID,
			&scope.RegistryID,
			&scope.Repository,
			&scope.Permission,
			&scope.CreatedAt,
		); err != nil {
			return nil, err
		}
		scopes = append(scopes, scope)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return scopes, nil
}

func (d *DB) RemoveAPIKey(ctx context.Context, userID, id uuid.UUID) error {
	const cmd = `DELETE FROM api_keys WHERE user_id = $1 AND id = $2`
	tag, err := d.conn.Exec(ctx, cmd, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (d *DB) UpdateAPIKeyLastUsedAt(ctx context.Context, id uuid.UUID) (time.Time, error) {
	var lastUsedAt time.Time
	const cmd = `UPDATE api_keys
		SET last_used_at = NOW()
		WHERE id = $1
		RETURNING last_used_at`
	if err := d.conn.QueryRow(ctx, cmd, id).Scan(&lastUsedAt); err != nil {
		if isNoRows(err) {
			return time.Time{}, ErrNotFound
		}
		return time.Time{}, err
	}
	return lastUsedAt, nil
}
