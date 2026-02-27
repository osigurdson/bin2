package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type RegistryRepository struct {
	ID           uuid.UUID
	RegistryID   uuid.UUID
	Name         string
	CreatedAt    time.Time
	LastPushedAt time.Time
	LastTag      *string
}

func (d *DB) EnsureRegistryRepository(ctx context.Context, registryID uuid.UUID, name string) (RegistryRepository, error) {
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" {
		return RegistryRepository{}, fmt.Errorf("repository name is required")
	}

	repo := RegistryRepository{ID: uuid.New(), RegistryID: registryID, Name: normalizedName}
	const cmd = `INSERT INTO repositories (id, registry_id, name)
		VALUES ($1, $2, $3)
		ON CONFLICT (registry_id, name)
		DO UPDATE SET name = EXCLUDED.name
		RETURNING id, registry_id, name, created_at, last_pushed_at`
	if err := d.conn.QueryRow(ctx, cmd, repo.ID, repo.RegistryID, repo.Name).Scan(
		&repo.ID,
		&repo.RegistryID,
		&repo.Name,
		&repo.CreatedAt,
		&repo.LastPushedAt,
	); err != nil {
		return RegistryRepository{}, err
	}
	return repo, nil
}

func (d *DB) TouchRegistryRepositoryPush(ctx context.Context, registryID uuid.UUID, name string) (RegistryRepository, error) {
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" {
		return RegistryRepository{}, fmt.Errorf("repository name is required")
	}

	repo := RegistryRepository{ID: uuid.New(), RegistryID: registryID, Name: normalizedName}
	const cmd = `INSERT INTO repositories (id, registry_id, name, last_pushed_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (registry_id, name)
		DO UPDATE SET last_pushed_at = NOW()
		RETURNING id, registry_id, name, created_at, last_pushed_at`
	if err := d.conn.QueryRow(ctx, cmd, repo.ID, repo.RegistryID, repo.Name).Scan(
		&repo.ID,
		&repo.RegistryID,
		&repo.Name,
		&repo.CreatedAt,
		&repo.LastPushedAt,
	); err != nil {
		return RegistryRepository{}, err
	}
	return repo, nil
}

func (d *DB) ListRepositoriesByRegistryID(ctx context.Context, registryID uuid.UUID) ([]RegistryRepository, error) {
	const cmd = `SELECT
			r.id,
			r.registry_id,
			r.name,
			r.created_at,
			r.last_pushed_at,
			last_tag.reference
		FROM repositories r
		LEFT JOIN LATERAL (
			SELECT mr.reference
			FROM manifest_refs mr
			WHERE mr.repository_id = r.id
			  AND mr.reference !~ '^sha256:[a-f0-9]{64}$'
			ORDER BY mr.updated_at DESC
			LIMIT 1
		) AS last_tag ON TRUE
		WHERE r.registry_id = $1
		ORDER BY r.last_pushed_at DESC, r.name ASC`
	rows, err := d.conn.Query(ctx, cmd, registryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	repos := make([]RegistryRepository, 0)
	for rows.Next() {
		var repo RegistryRepository
		if err := rows.Scan(
			&repo.ID,
			&repo.RegistryID,
			&repo.Name,
			&repo.CreatedAt,
			&repo.LastPushedAt,
			&repo.LastTag,
		); err != nil {
			return nil, err
		}
		repos = append(repos, repo)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return repos, nil
}
