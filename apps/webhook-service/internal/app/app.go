package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	serviceconfig "codeatlas/apps/webhook-service/internal/config"
	httpapi "codeatlas/apps/webhook-service/internal/http"
	"codeatlas/packages/kafka"
	"codeatlas/packages/logger"
)

type App struct {
	config   serviceconfig.Config
	logger   *slog.Logger
	server   *http.Server
	producer kafka.Producer
}

func New(cfg serviceconfig.Config) (*App, error) {
	log, err := logger.New(logger.Config{
		Service: cfg.ServiceName,
		Level:   cfg.LogLevel,
		JSON:    cfg.LogJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize logger: %w", err)
	}

	mux := http.NewServeMux()
	producer := kafka.NewProducer(kafka.ProducerConfig{
		Logger:   log,
		Enabled:  cfg.KafkaEnabled,
		Brokers:  cfg.KafkaBrokers,
		ClientID: cfg.ServiceName,
	})
	handler := httpapi.NewHandler(cfg, log, producer)
	handler.Register(mux)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &App{
		config:   cfg,
		logger:   log,
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
		defer a.producer.Close()
		return a.server.Shutdown(shutdownCtx)
	case err := <-errCh:
		_ = a.producer.Close()
		return err
	}
}
