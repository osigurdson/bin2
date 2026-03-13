package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	MetricStorageBytes = "storage-bytes"
	MetricPushOpCount  = "push-op-count"
	MetricPullOpCount  = "pull-op-count"
)

type UsageEvent struct {
	ID         uuid.UUID
	CreatedAt  time.Time
	TenantID   uuid.UUID
	RegistryID *uuid.UUID
	RepoID     *uuid.UUID
	Digest     string
	Metric     string
	Value      int64
}

func (d *DB) InsertUsageEvents(ctx context.Context, events []UsageEvent) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := d.conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	const cmd = `INSERT INTO usage_events (id, tenant_id, registry_id, repo_id, digest, metric, value)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7)
		ON CONFLICT (id) DO NOTHING`
	for _, e := range events {
		if _, err := tx.Exec(ctx, cmd, e.ID, e.TenantID, e.RegistryID, e.RepoID, e.Digest, e.Metric, e.Value); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (d *DB) ListUsageEventsByTenant(ctx context.Context, tenantID uuid.UUID, metric string, limit int, after time.Time) ([]UsageEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	args := []any{tenantID}
	where := []string{"tenant_id = $1"}

	if metric != "" {
		args = append(args, metric)
		where = append(where, fmt.Sprintf("metric = $%d", len(args)))
	}
	if !after.IsZero() {
		args = append(args, after)
		where = append(where, fmt.Sprintf("created_at > $%d", len(args)))
	}
	args = append(args, limit)

	query := `SELECT id, created_at, tenant_id, registry_id, repo_id, COALESCE(digest, ''), metric, value
		FROM usage_events
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY created_at ASC
		LIMIT $` + fmt.Sprintf("%d", len(args))

	rows, err := d.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]UsageEvent, 0)
	for rows.Next() {
		var e UsageEvent
		if err := rows.Scan(&e.ID, &e.CreatedAt, &e.TenantID, &e.RegistryID, &e.RepoID, &e.Digest, &e.Metric, &e.Value); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// TenantHasBlob reports whether any repository in the tenant already has a
// repository_objects entry for the given blob digest.
func (d *DB) TenantHasBlob(ctx context.Context, tenantID uuid.UUID, digest string) (bool, error) {
	const cmd = `SELECT EXISTS (
		SELECT 1 FROM repository_objects ro
		JOIN repositories r  ON ro.repository_id = r.id
		JOIN registries reg  ON r.registry_id    = reg.id
		WHERE ro.digest = $1 AND reg.tenant_id = $2
	)`
	var exists bool
	err := d.conn.QueryRow(ctx, cmd, strings.TrimSpace(digest), tenantID).Scan(&exists)
	return exists, err
}

// GetRegistryTenantID returns the tenant_id for a registry.
func (d *DB) GetRegistryTenantID(ctx context.Context, registryID uuid.UUID) (uuid.UUID, error) {
	const cmd = `SELECT tenant_id FROM registries WHERE id = $1`
	var tenantID uuid.UUID
	err := d.conn.QueryRow(ctx, cmd, registryID).Scan(&tenantID)
	if err != nil {
		if isNoRows(err) {
			return uuid.Nil, ErrNotFound
		}
		return uuid.Nil, err
	}
	return tenantID, nil
}
