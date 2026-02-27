package db

import (
	"context"

	"github.com/google/uuid"
)

type Registry struct {
	ID    uuid.UUID
	OrgID uuid.UUID
	Name  string
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

func (d *DB) ListRegistriesByOrg(ctx context.Context, orgID uuid.UUID) ([]Registry, error) {
	const cmd = `SELECT id, org_id, name
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
		if err := rows.Scan(&registry.ID, &registry.OrgID, &registry.Name); err != nil {
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
	const cmd = `SELECT id, org_id, name
		FROM registries
		WHERE id = $1`
	var registry Registry
	err := d.conn.QueryRow(ctx, cmd, id).Scan(
		&registry.ID,
		&registry.OrgID,
		&registry.Name,
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
	const cmd = `SELECT id, org_id, name
		FROM registries
		WHERE name = $1`
	var registry Registry
	err := d.conn.QueryRow(ctx, cmd, name).Scan(
		&registry.ID,
		&registry.OrgID,
		&registry.Name,
	)
	if err != nil {
		if isNoRows(err) {
			return Registry{}, ErrNotFound
		}
		return Registry{}, err
	}
	return registry, nil
}
