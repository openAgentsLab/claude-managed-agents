// Package observability sets up the process-wide structured logger.
package observability

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"forge/internal/config"
)

// InitLogger configures the global slog logger from LogConfig and returns it.
// Call this once during program initialisation before any other component starts.
// If cfg.File is set, logs are written to that file (appended, created if absent);
// otherwise logs go to stderr.
func InitLogger(cfg config.LogConfig) *slog.Logger {
	level := parseLevel(cfg.Level)
	out := logWriter(cfg.File)

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if strings.ToLower(cfg.Format) == "json" {
		handler = slog.NewJSONHandler(out, opts)
	} else {
		handler = slog.NewTextHandler(out, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

// logWriter returns the io.Writer for log output.
// Creates parent directories as needed, then opens the file in append mode.
// Falls back to stderr on any error.
func logWriter(path string) io.Writer {
	if path == "" {
		return os.Stderr
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		slog.Warn("failed to create log directory, falling back to stderr", "path", path, "err", err)
		return os.Stderr
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		slog.Warn("failed to open log file, falling back to stderr", "path", path, "err", err)
		return os.Stderr
	}
	return f
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
