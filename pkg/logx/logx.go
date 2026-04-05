package logx

import (
	"log/slog"
	"os"
)

func New(component string) *slog.Logger {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(handler).With("component", component)
}
