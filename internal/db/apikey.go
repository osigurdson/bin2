package db

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type APIKey struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	KeyName       string
	Prefix        string
	SecretKeyHash string
	CreatedAt     time.Time
	LastUsedAt    *time.Time
}

func (d *DB) AddAPIKey(ctx context.Context, apiKey APIKey) (APIKey, error) {
	const cmd = `INSERT INTO api_keys (id, user_id, name, key_hash, prefix)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at`
	err := d.conn.QueryRow(
		ctx,
		cmd,
		apiKey.ID,
		apiKey.UserID,
		apiKey.KeyName,
		apiKey.SecretKeyHash,
		apiKey.Prefix,
	).Scan(&apiKey.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return APIKey{}, ErrConflict
		}
		return APIKey{}, err
	}
	return apiKey, nil
}

func (d *DB) ListAPIKeysByUser(ctx context.Context, userID uuid.UUID) ([]APIKey, error) {
	const cmd = `SELECT id, name, key_hash, prefix, created_at, last_used_at
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
			&key.KeyName,
			&key.SecretKeyHash,
			&key.Prefix,
			&key.CreatedAt,
			&key.LastUsedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, key)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
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
