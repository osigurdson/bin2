package db

import (
	"context"
	"fmt"
)

func AdminCreateDB(ctx context.Context, cfg DBConfig, drop bool) error {
	adminCfg := cfg
	adminCfg.Database = "postgres"

	conn, err := createConnection(ctx, adminCfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	if drop {
		terminateCmd := "SELECT pg_terminate_backend(pg_stat_activity.pid) " +
			"FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid();"
		if _, err := conn.Exec(ctx, terminateCmd, cfg.Database); err != nil {
			return err
		}

		dropCmd := fmt.Sprintf("DROP DATABASE IF EXISTS %s", cfg.Database)
		if _, err := conn.Exec(ctx, dropCmd); err != nil {
			return err
		}
	}

	var exists bool
	existsCmd := "SELECT EXISTS (SELECT FROM pg_database WHERE datname = $1);"
	if err := conn.QueryRow(ctx, existsCmd, cfg.Database).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("database %q already exists", cfg.Database)
	}

	createCmd := fmt.Sprintf("CREATE DATABASE %s", cfg.Database)
	if _, err := conn.Exec(ctx, createCmd); err != nil {
		return err
	}
	return nil
}

func AdminCheckIfDBExists(ctx context.Context, cfg DBConfig) (bool, error) {
	database := cfg.Database
	cfg.Database = "postgres"

	conn, err := createConnection(ctx, cfg)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	var exists bool
	cmd := "SELECT EXISTS (SELECT FROM pg_database WHERE datname = $1);"
	if err := conn.QueryRow(ctx, cmd, database).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}
