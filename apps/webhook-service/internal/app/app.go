package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	serviceconfig "codeatlas/apps/webhook-service/internal/config"
	httpapi "codeatlas/apps/webhook-service/internal/http"
	"codeatlas/apps/webhook-service/internal/repository"
	"codeatlas/packages/database"
	"codeatlas/packages/kafka"
	"codeatlas/packages/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	config   serviceconfig.Config
	logger   *slog.Logger
	db       *pgxpool.Pool
	server   *http.Server
	producer kafka.Producer
}

func New(ctx context.Context, cfg serviceconfig.Config) (*App, error) {
	log, err := logger.New(logger.Config{
		Service: cfg.ServiceName,
		Level:   cfg.LogLevel,
		JSON:    cfg.LogJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize logger: %w", err)
	}

	dbPool, err := database.NewPostgresPool(ctx, cfg.Postgres)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	mux := http.NewServeMux()
	producer := kafka.NewProducer(kafka.ProducerConfig{
		Logger:   log,
		Enabled:  cfg.KafkaEnabled,
		Brokers:  cfg.KafkaBrokers,
		ClientID: cfg.ServiceName,
	})
	deliveryRepo := repository.NewWebhookDeliveryRepository(dbPool)
	handler := httpapi.NewHandler(cfg, log, producer, deliveryRepo)
	handler.Register(mux)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &App{
		config:   cfg,
		logger:   log,
		db:       dbPool,
		server:   server,
		producer: producer,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		a.logger.Info("starting http server", "port", a.config.HTTPPort)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.config.ShutdownTimeout)
		defer cancel()

		a.logger.Info("shutting down")
		defer a.db.Close()
		defer a.producer.Close()
		return a.server.Shutdown(shutdownCtx)
	case err := <-errCh:
		a.db.Close()
		_ = a.producer.Close()
		return err
	}
}
