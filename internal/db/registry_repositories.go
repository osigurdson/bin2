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
