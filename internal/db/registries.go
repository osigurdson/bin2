package db

import (
	"context"

	"github.com/google/uuid"
)

type Registry struct {
	ID     uuid.UUID
	UserID uuid.UUID
	Name   string
}

type AddRegistryArgs struct {
	UserID uuid.UUID
	Name   string
}

func (d *DB) AddRegistry(ctx context.Context, args AddRegistryArgs) (Registry, error) {
	registry := Registry{
		ID:     uuid.New(),
		UserID: args.UserID,
		Name:   args.Name,
	}

	const cmd = `INSERT INTO registries (id, user_id, name)
		VALUES ($1, $2, $3)`
	if _, err := d.conn.Exec(ctx, cmd, registry.ID, registry.UserID, registry.Name); err != nil {
		if isUniqueViolation(err) {
			return Registry{}, ErrConflict
		}
		return Registry{}, err
	}

	return registry, nil
}

func (d *DB) ListRegistriesByUser(ctx context.Context, userID uuid.UUID) ([]Registry, error) {
	const cmd = `SELECT id, user_id, name
		FROM registries
		WHERE user_id = $1
		ORDER BY name ASC`
	rows, err := d.conn.Query(ctx, cmd, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	registries := make([]Registry, 0)
	for rows.Next() {
		var registry Registry
		if err := rows.Scan(&registry.ID, &registry.UserID, &registry.Name); err != nil {
			return nil, err
		}
		registries = append(registries, registry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return registries, nil
}
