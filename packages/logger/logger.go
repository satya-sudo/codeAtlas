package logger

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	Service string
	Level   string
	JSON    bool
}

func New(cfg Config) (*slog.Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if cfg.JSON {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	base := slog.New(handler)
	if strings.TrimSpace(cfg.Service) == "" {
		return base, nil
	}

	return base.With("service", cfg.Service), nil
}

func Must(cfg Config) *slog.Logger {
	log, err := New(cfg)
	if err != nil {
		panic(err)
	}

	return log
}

func parseLevel(level string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported log level %q", level)
	}
}
