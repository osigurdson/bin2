package main

import (
	"context"
	"log/slog"
	"os"

	"bin2.io/internal/server"
)

func main() {
	ctx := context.Background()

	level := slog.LevelInfo
	if os.Getenv("DEBUG") == "1" {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)

	srv, err := server.New()
	if err != nil {
		slog.Error("could not create server", slog.Any("err", err))
		return
	}
	defer srv.Close()

	if err := srv.Run(ctx, ":5000"); err != nil {
		slog.Error("server exited", slog.Any("err", err))
	}
}
