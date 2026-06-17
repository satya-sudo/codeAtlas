package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	serviceconfig "codeatlas/apps/analytics-worker/internal/config"
	"codeatlas/packages/database"
	"codeatlas/packages/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	config serviceconfig.Config
	logger *slog.Logger
	db     *pgxpool.Pool
}

func New(ctx context.Context, cfg serviceconfig.Config) (*App, error) {
	log, err := logger.New(logger.Config{
		Service: cfg.ServiceName,
		Level:   cfg.LogLevel,
		JSON:    cfg.LogJSON,
	})
	if err != nil {
		return nil, err
	}

	dbCfg, err := database.LoadPostgresConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("load postgres config: %w", err)
	}

	dbPool, err := database.NewPostgresPool(ctx, dbCfg)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	return &App{
		config: cfg,
		logger: log,
		db:     dbPool,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	a.logger.Info("starting worker", "poll_interval", a.config.PollInterval.String())

	ticker := time.NewTicker(a.config.PollInterval)
	defer ticker.Stop()
	defer a.db.Close()

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
