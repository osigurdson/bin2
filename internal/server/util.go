package server

import (
	"log/slog"
	"runtime/debug"
)

func logError(err error) {
	slog.Error("Unhandled error",
		slog.Any("err", err),
		slog.String("stack", string(debug.Stack())),
	)
}
