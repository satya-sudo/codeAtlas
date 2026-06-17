package app

import (
	"context"
	"log/slog"
	"time"

	serviceconfig "codeatlas/apps/sync-service/internal/config"
	"codeatlas/packages/logger"
)

type App struct {
	config serviceconfig.Config
	logger *slog.Logger
}

func New(cfg serviceconfig.Config) (*App, error) {
	log, err := logger.New(logger.Config{
		Service: cfg.ServiceName,
		Level:   cfg.LogLevel,
		JSON:    cfg.LogJSON,
	})
	if err != nil {
		return nil, err
	}

	return &App{
		config: cfg,
		logger: log,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	a.logger.Info("starting worker", "poll_interval", a.config.PollInterval.String())

	ticker := time.NewTicker(a.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("worker stopped")
			return nil
		case <-ticker.C:
			a.logger.Debug("worker heartbeat")
		}
	}
}
