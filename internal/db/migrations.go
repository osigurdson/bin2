package db

import (
	"context"
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

var (
	//go:embed migrations/*.sql
	migrationFS embed.FS
)

func RunMigrations(ctx context.Context, cfg DBConfig) error {
	source, err := iofs.New(migrationFS, "migrations")
	if err != nil {
		return err
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, getDsn(cfg))
	if err != nil {
		return fmt.Errorf("migration init error: %w", err)
	}

	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			return nil
		}
		return fmt.Errorf("migration failed: %w", err)
	}
	return nil
}
