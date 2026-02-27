package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound     = errors.New("record not found")
	ErrConflict     = errors.New("conflict")
	ErrScopeConflict = errors.New("scope conflict")
)

type DB struct {
	conn *pgxpool.Pool
}

func New(ctx context.Context, cfg DBConfig) (*DB, error) {
	conn, err := createConnection(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &DB{conn: conn}, nil
}

func (d *DB) Close() {
	if d == nil || d.conn == nil {
		return
	}
	d.conn.Close()
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
