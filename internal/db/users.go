package db

import (
	"context"

	"github.com/google/uuid"
)

type User struct {
	ID    uuid.UUID
	Sub   string
	Email string
}

func (d *DB) EnsureUser(ctx context.Context, sub, email string) (User, error) {
	user, err := d.GetUserBySub(ctx, sub)
	if err == nil {
		// Keep email in sync when Clerk-provided email changes.
		if email != "" && user.Email != email {
			const cmd = `UPDATE users SET email = $1 WHERE id = $2`
			if _, updateErr := d.conn.Exec(ctx, cmd, email, user.ID); updateErr == nil {
				user.Email = email
			}
		}
		return user, nil
	}
	if err != ErrNotFound {
		return User{}, err
	}
	if email == "" {
		return User{}, ErrNotFound
	}

	newUser := User{
		ID:    uuid.New(),
		Sub:   sub,
		Email: email,
	}

	const cmd = `INSERT INTO users (id, sub, email)
		VALUES ($1, $2, $3)
		ON CONFLICT (sub) DO UPDATE SET email = EXCLUDED.email
		RETURNING id, sub, email`
	if err := d.conn.QueryRow(ctx, cmd, newUser.ID, newUser.Sub, newUser.Email).
		Scan(&newUser.ID, &newUser.Sub, &newUser.Email); err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrConflict
		}
		return User{}, err
	}

	return newUser, nil
}

func (d *DB) GetUserBySub(ctx context.Context, sub string) (User, error) {
	const cmd = `SELECT id, sub, email FROM users WHERE sub = $1`
	var user User
	if err := d.conn.QueryRow(ctx, cmd, sub).Scan(&user.ID, &user.Sub, &user.Email); err != nil {
		if isNoRows(err) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	return user, nil
}

func (d *DB) GetUserByEmail(ctx context.Context, email string) (User, error) {
	const cmd = `SELECT id, sub, email FROM users WHERE email = $1`
	var user User
	if err := d.conn.QueryRow(ctx, cmd, email).Scan(&user.ID, &user.Sub, &user.Email); err != nil {
		if isNoRows(err) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	return user, nil
}

func (d *DB) UpdateUserSub(ctx context.Context, userID uuid.UUID, sub string) (User, error) {
	const cmd = `UPDATE users
		SET sub = $1
		WHERE id = $2
		RETURNING id, sub, email`
	var user User
	if err := d.conn.QueryRow(ctx, cmd, sub, userID).Scan(&user.ID, &user.Sub, &user.Email); err != nil {
		if isNoRows(err) {
			return User{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return User{}, ErrConflict
		}
		return User{}, err
	}
	return user, nil
}
