package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	serviceconfig "codeatlas/apps/api-gateway/internal/config"
	httpapi "codeatlas/apps/api-gateway/internal/http"
	"codeatlas/packages/logger"
)

type App struct {
	config serviceconfig.Config
	logger *slog.Logger
	server *http.Server
}

func New(cfg serviceconfig.Config) (*App, error) {
	log := logger.Must(logger.Config{
		Service: cfg.ServiceName,
		Level:   cfg.LogLevel,
		JSON:    cfg.LogJSON,
	})

	authURL, err := url.Parse(cfg.AuthServiceURL)
	if err != nil {
		return nil, fmt.Errorf("parse auth service url: %w", err)
	}

	repoURL, err := url.Parse(cfg.RepoServiceURL)
	if err != nil {
		return nil, fmt.Errorf("parse repo service url: %w", err)
	}

	webhookURL, err := url.Parse(cfg.WebhookServiceURL)
	if err != nil {
		return nil, fmt.Errorf("parse webhook service url: %w", err)
	}

	mux := http.NewServeMux()
	handler := httpapi.NewProxyHandler(log, []httpapi.Route{
		{Prefix: "/auth/", Target: authURL},
		{Prefix: "/repos", Target: repoURL},
		{Prefix: "/repos/", Target: repoURL},
		{Prefix: "/integrations/github/", Target: repoURL},
		{Prefix: "/webhooks/github", Target: webhookURL},
		{Prefix: "/webhooks/github/", Target: webhookURL},
	})
	if err := handler.Register(mux); err != nil {
		return nil, err
	}

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           httpapi.CORSMiddleware(cfg.FrontendOrigin)(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &App{
		config: cfg,
		logger: log,
		server: server,
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
		return a.server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
