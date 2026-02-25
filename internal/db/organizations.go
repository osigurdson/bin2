package db

import (
	"context"

	"github.com/google/uuid"
)

type Organization struct {
	ID          uuid.UUID
	WorkOSOrgID *string
	Slug        string
	Name        string
}

func (d *DB) CreateOrganization(ctx context.Context, id uuid.UUID, slug, name string, workosOrgID *string) (Organization, error) {
	const cmd = `INSERT INTO organizations (id, workos_org_id, slug, name)
		VALUES ($1, $2, $3, $4)`
	if _, err := d.conn.Exec(ctx, cmd, id, workosOrgID, slug, name); err != nil {
		if isUniqueViolation(err) {
			return Organization{}, ErrConflict
		}
		return Organization{}, err
	}
	return Organization{
		ID:          id,
		WorkOSOrgID: workosOrgID,
		Slug:        slug,
		Name:        name,
	}, nil
}

func (d *DB) AddOrgMember(ctx context.Context, orgID, userID uuid.UUID, role string) error {
	const cmd = `INSERT INTO org_members (org_id, user_id, role) VALUES ($1, $2, $3)`
	if _, err := d.conn.Exec(ctx, cmd, orgID, userID, role); err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return err
	}
	return nil
}

func (d *DB) GetPersonalOrgByUser(ctx context.Context, userID uuid.UUID) (Organization, error) {
	const cmd = `SELECT o.id, o.workos_org_id, o.slug, o.name
		FROM organizations o
		JOIN org_members m ON m.org_id = o.id
		WHERE m.user_id = $1 AND o.workos_org_id IS NULL
		LIMIT 1`
	var org Organization
	if err := d.conn.QueryRow(ctx, cmd, userID).Scan(&org.ID, &org.WorkOSOrgID, &org.Slug, &org.Name); err != nil {
		if isNoRows(err) {
			return Organization{}, ErrNotFound
		}
		return Organization{}, err
	}
	return org, nil
}

func (d *DB) IsOrgMember(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	const cmd = `SELECT EXISTS(SELECT 1 FROM org_members WHERE org_id = $1 AND user_id = $2)`
	var exists bool
	if err := d.conn.QueryRow(ctx, cmd, orgID, userID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}
