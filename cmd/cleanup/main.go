package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"bin2.io/internal/db"
	"bin2.io/internal/registrygc"
	"bin2.io/internal/registrystorage"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	level := slog.LevelInfo
	if os.Getenv("DEBUG") == "1" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	dbCfg, err := db.NewConfigFromEnv()
	if err != nil {
		slog.Error("db config", slog.Any("err", err))
		os.Exit(1)
	}
	database, err := db.New(ctx, dbCfg)
	if err != nil {
		slog.Error("db connect", slog.Any("err", err))
		os.Exit(1)
	}
	defer database.Close()

	storage, err := registrystorage.NewFromEnv()
	if err != nil {
		slog.Error("storage init", slog.Any("err", err))
		os.Exit(1)
	}

	cfg := registrygc.Config{
		BatchSize: envInt("GC_BATCH_SIZE", 100),
		MinAge:    envDuration("GC_MIN_AGE", 24*time.Hour),
	}

	runner := registrygc.New(database, storage, cfg)
	runner.Diagnose(ctx)
	n, err := runner.RunOnce(ctx)
	if err != nil {
		slog.Error("gc failed", slog.Any("err", err))
		os.Exit(1)
	}
	slog.Info("gc complete", slog.Int("collected", n))
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= 0 {
			return d
		}
	}
	return fallback
}
