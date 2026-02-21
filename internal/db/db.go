package db

import (
	"context"
	"fmt"
	"os"
	"strconv"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DBConfig struct {
	Username string
	Password string
	Hostname string
	Port     int
	Database string
}

func NewConfigFromEnv() (DBConfig, error) {
	var cfg DBConfig

	cfg.Username = os.Getenv("POSTGRES_USERNAME")
	if cfg.Username == "" {
		return DBConfig{}, fmt.Errorf("POSTGRES_USERNAME not set")
	}

	cfg.Password = os.Getenv("POSTGRES_PASSWORD")
	if cfg.Password == "" {
		return DBConfig{}, fmt.Errorf("POSTGRES_PASSWORD not set")
	}

	cfg.Hostname = os.Getenv("POSTGRES_HOSTNAME")
	if cfg.Hostname == "" {
		return DBConfig{}, fmt.Errorf("POSTGRES_HOSTNAME not set")
	}

	cfg.Database = os.Getenv("POSTGRES_DBNAME")
	if cfg.Database == "" {
		return DBConfig{}, fmt.Errorf("POSTGRES_DBNAME not set")
	}

	portRaw := os.Getenv("POSTGRES_PORT")
	if portRaw == "" {
		cfg.Port = 5432
		return cfg, nil
	}

	port, err := strconv.Atoi(portRaw)
	if err != nil {
		return DBConfig{}, fmt.Errorf("invalid POSTGRES_PORT %q: %w", portRaw, err)
	}
	cfg.Port = port
	return cfg, nil
}

func createConnection(ctx context.Context, cfg DBConfig) (*pgxpool.Pool, error) {
	pgxCfg, err := pgxpool.ParseConfig(getDsn(cfg))
	if err != nil {
		return nil, err
	}
	return pgxpool.NewWithConfig(ctx, pgxCfg)
}

func getDsn(cfg DBConfig) string {
	if cfg.Port == 0 {
		cfg.Port = 5432
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.Username,
		cfg.Password,
		cfg.Hostname,
		cfg.Port,
		cfg.Database,
	)
}
