package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"bin2.io/internal/db"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var destructive bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize Postgres database and run migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd.Context(), destructive)
		},
	}
	cmd.Flags().BoolVar(&destructive, "destructive", false, "drop and recreate database before migrations")
	return cmd
}

func runInit(ctx context.Context, destructive bool) error {
	cfg, err := db.NewConfigFromEnv()
	if err != nil {
		return fmt.Errorf("could not read postgres configuration: %w", err)
	}

	exists, err := db.AdminCheckIfDBExists(ctx, cfg)
	if err != nil {
		return fmt.Errorf("could not check database existence: %w", err)
	}

	if exists && destructive {
		log.Printf("database %s exists; dropping and recreating", cfg.Database)
		if err := db.AdminCreateDB(ctx, cfg, true); err != nil {
			return fmt.Errorf("could not recreate database: %w", err)
		}
	} else if !exists {
		log.Printf("database %s does not exist; creating", cfg.Database)
		if err := db.AdminCreateDB(ctx, cfg, false); err != nil {
			return fmt.Errorf("could not create database: %w", err)
		}
	} else {
		log.Printf("database %s already exists; skipping create", cfg.Database)
	}

	log.Println("running migrations")
	if err := db.RunMigrations(ctx, cfg); err != nil {
		return fmt.Errorf("could not run migrations: %w", err)
	}
	log.Println("database initialization complete")
	return nil
}
