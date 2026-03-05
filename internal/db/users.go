package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID
	Sub       string
	TenantID  uuid.UUID
	Onboarded bool
}

func (d *DB) GetOrCreateUser(
	ctx context.Context,
	sub string,
	org string,
) (User, error) {
	if sub == "" {
		return User{}, fmt.Errorf("sub not specified")
	}

	tenantName := org
	if tenantName == "" {
		tenantName = "personal__" + sub
	}

	var user User
	err := d.conn.QueryRow(ctx, `
		WITH t AS (
			INSERT INTO tenants (id, name)
			VALUES (gen_random_uuid(), $1)
			ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
			RETURNING id, onboarded
		)
		INSERT INTO users (id, tenant_id, sub)
		VALUES (gen_random_uuid(), (SELECT id FROM t), $2)
		ON CONFLICT (sub) DO UPDATE SET tenant_id = (SELECT id FROM t)
		RETURNING id, tenant_id, sub, (SELECT onboarded FROM t)
	`, tenantName, sub).Scan(&user.ID, &user.TenantID, &user.Sub, &user.Onboarded)
	if err != nil {
		return user, err
	}
	return user, nil
}

func (d *DB) SetUserOnboarded(ctx context.Context, userID uuid.UUID, onboarded bool) error {
	const cmd = `UPDATE tenants SET onboarded = $1 WHERE id = (SELECT tenant_id FROM users where id = $2)`
	tag, err := d.conn.Exec(ctx, cmd, onboarded, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
