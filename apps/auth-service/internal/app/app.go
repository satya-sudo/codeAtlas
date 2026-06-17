package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	serviceconfig "codeatlas/apps/auth-service/internal/config"
	httpapi "codeatlas/apps/auth-service/internal/http"
	"codeatlas/apps/auth-service/internal/oauth"
	"codeatlas/apps/auth-service/internal/repository"
	"codeatlas/apps/auth-service/internal/tokens"
	"codeatlas/packages/database"
	"codeatlas/packages/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	config serviceconfig.Config
	logger *slog.Logger
	db     *pgxpool.Pool
	server *http.Server
}

func New(ctx context.Context, cfg serviceconfig.Config) (*App, error) {
	log := logger.Must(logger.Config{
		Service: cfg.ServiceName,
		Level:   cfg.LogLevel,
		JSON:    cfg.LogJSON,
	})

	dbCfg, err := database.LoadPostgresConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("load postgres config: %w", err)
	}

	dbPool, err := database.NewPostgresPool(ctx, dbCfg)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	userRepo := repository.NewUserRepository(dbPool)
	githubClient := oauth.NewGitHubClient(cfg.GitHubClientID, cfg.GitHubClientSecret, cfg.GitHubRedirectURL, cfg.GitHubPrompt)
	tokenManager := tokens.NewManager(cfg.JWTSecret, cfg.JWTTTL)

	mux := http.NewServeMux()
	handler := httpapi.NewHandler(cfg, log, githubClient, userRepo, tokenManager)
	handler.Register(mux)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           httpapi.CORSMiddleware(cfg.FrontendOrigin)(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &App{
		config: cfg,
		logger: log,
		db:     dbPool,
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
		defer a.db.Close()

		return a.server.Shutdown(shutdownCtx)
	case err := <-errCh:
		a.db.Close()
		return err
	}
}
