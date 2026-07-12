package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	serviceconfig "codeatlas/apps/sync-service/internal/config"
	"codeatlas/apps/sync-service/internal/integrations"
	"codeatlas/apps/sync-service/internal/repository"
	"codeatlas/packages/database"
	"codeatlas/packages/events"
	sharedkafka "codeatlas/packages/kafka"
	"codeatlas/packages/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	config   serviceconfig.Config
	logger   *slog.Logger
	consumer sharedkafka.Consumer
	db       *pgxpool.Pool
	repos    *repository.RepositoryRepository
	github   *integrations.GitHubApp
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

	dbCfg, err := database.LoadPostgresConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("load postgres config: %w", err)
	}

	dbPool, err := database.NewPostgresPool(context.Background(), dbCfg)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	githubApp, err := integrations.NewGitHubApp(integrations.GitHubAppConfig{
		Slug:           cfg.GitHubAppSlug,
		AppID:          cfg.GitHubAppID,
		ClientID:       cfg.GitHubAppClientID,
		PrivateKeyPath: cfg.GitHubAppPrivateKeyPath,
		APIBaseURL:     cfg.GitHubAPIBaseURL,
		APITimeout:     cfg.GitHubAPITimeout,
	})
	if err != nil {
		dbPool.Close()
		return nil, fmt.Errorf("configure github app integration: %w", err)
	}

	var consumer sharedkafka.Consumer
	if cfg.KafkaEnabled {
		consumer = sharedkafka.NewConsumer(sharedkafka.ConsumerConfig{
			Brokers: cfg.KafkaBrokers,
			GroupID: cfg.RepositorySyncConsumerGroup,
			Topic:   cfg.RepositorySyncTopic,
		})
	}

	return &App{
		config:   cfg,
		logger:   log,
		consumer: consumer,
		db:       dbPool,
		repos:    repository.NewRepositoryRepository(dbPool),
		github:   githubApp,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	defer a.db.Close()

	if a.consumer != nil {
		defer a.consumer.Close()
		return a.runKafkaConsumer(ctx)
	}

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

func (a *App) runKafkaConsumer(ctx context.Context) error {
	a.logger.Info(
		"starting kafka consumer",
		"brokers", a.config.KafkaBrokers,
		"topic", a.config.RepositorySyncTopic,
		"group_id", a.config.RepositorySyncConsumerGroup,
	)

	for {
		msg, err := a.consumer.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				a.logger.Info("worker stopped")
				return nil
			}
			return err
		}

		a.logger.Debug(
			"fetched kafka message",
			"topic", msg.Topic,
			"key", string(msg.Key),
			"payload_bytes", len(msg.Value),
		)

		var event events.RepositorySyncRequested
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			a.logger.Error("decode repository sync event", "error", err, "payload", string(msg.Value))
			continue
		}

		a.logger.Info(
			"received repository sync request",
			"sync_run_id", event.SyncRunID,
			"sync_run_created_at", event.SyncRunCreatedAt,
			"repository_id", event.RepositoryID,
			"sync_type", event.SyncType,
			"requested_by_user_id", event.RequestedByUserID,
		)

		if err := a.processRepositorySyncRequested(ctx, event); err != nil {
			if errors.Is(err, repository.ErrSyncRunNotFound) {
				a.logger.Info(
					"skipping stale repository sync request",
					"sync_run_id", event.SyncRunID,
					"repository_id", event.RepositoryID,
				)
				if err := a.consumer.CommitMessages(ctx, msg); err != nil {
					return err
				}
				continue
			}
			a.logger.Error(
				"process repository sync request",
				"sync_run_id", event.SyncRunID,
				"repository_id", event.RepositoryID,
				"error", err,
			)
			if markErr := a.repos.MarkSyncRunFailed(ctx, event.SyncRunID, event.RepositoryID, event.SyncRunCreatedAt, err.Error()); markErr != nil {
				if errors.Is(markErr, repository.ErrSyncRunNotFound) {
					a.logger.Info(
						"skipping failure update for stale repository sync request",
						"sync_run_id", event.SyncRunID,
						"sync_run_created_at", event.SyncRunCreatedAt,
						"repository_id", event.RepositoryID,
					)
					if err := a.consumer.CommitMessages(ctx, msg); err != nil {
						return err
					}
					continue
				}
				a.logger.Error(
					"mark sync run failed",
					"sync_run_id", event.SyncRunID,
					"repository_id", event.RepositoryID,
					"error", markErr,
				)
			}
		}

		if err := a.consumer.CommitMessages(ctx, msg); err != nil {
			return err
		}
	}
}

func (a *App) processRepositorySyncRequested(ctx context.Context, event events.RepositorySyncRequested) error {
	syncStartedAt := time.Now()

	repo, err := a.repos.FindRepositoryForSync(ctx, event.RepositoryID)
	if err != nil {
		return err
	}

	a.logger.Debug(
		"resolved repository for sync",
		"sync_run_id", event.SyncRunID,
		"sync_run_created_at", event.SyncRunCreatedAt,
		"repository_id", repo.ID,
		"repository_name", repo.Owner+"/"+repo.Name,
		"installation_id", repo.InstallationID,
	)

	if repo.InstallationID == nil || *repo.InstallationID == 0 {
		return fmt.Errorf("repository %d has no github app installation id", repo.ID)
	}

	if err := a.repos.MarkSyncRunRunning(ctx, event.SyncRunID, event.RepositoryID, event.SyncRunCreatedAt); err != nil {
		return err
	}

	a.logger.Debug(
		"marked sync run running",
		"sync_run_id", event.SyncRunID,
		"sync_run_created_at", event.SyncRunCreatedAt,
		"repository_id", event.RepositoryID,
	)

	contributorsStartedAt := time.Now()
	contributors, err := a.github.ListRepositoryContributors(ctx, *repo.InstallationID, repo.Owner, repo.Name)
	if err != nil {
		return err
	}
	a.logger.Debug(
		"fetched repository contributors",
		"sync_run_id", event.SyncRunID,
		"repository_id", event.RepositoryID,
		"repository_name", repo.Owner+"/"+repo.Name,
		"contributors_count", len(contributors),
		"duration_ms", time.Since(contributorsStartedAt).Milliseconds(),
	)

	if err := a.repos.ReplaceContributors(ctx, repo.ID, contributors); err != nil {
		return err
	}
	a.logger.Debug(
		"stored repository contributors",
		"sync_run_id", event.SyncRunID,
		"repository_id", event.RepositoryID,
		"contributors_count", len(contributors),
	)

	commitsStartedAt := time.Now()
	commits, err := a.github.ListRepositoryCommits(ctx, *repo.InstallationID, repo.Owner, repo.Name)
	if err != nil {
		return err
	}
	a.logger.Debug(
		"fetched repository commits",
		"sync_run_id", event.SyncRunID,
		"repository_id", event.RepositoryID,
		"repository_name", repo.Owner+"/"+repo.Name,
		"commits_count", len(commits),
		"duration_ms", time.Since(commitsStartedAt).Milliseconds(),
	)

	commitWriteStartedAt := time.Now()
	commitStats, err := a.repos.ReplaceCommitData(ctx, repo.ID, commits)
	if err != nil {
		return err
	}
	a.logger.Debug(
		"stored repository commit data",
		"sync_run_id", event.SyncRunID,
		"repository_id", event.RepositoryID,
		"commits_count", commitStats.CommitsCount,
		"commit_files_count", commitStats.CommitFilesCount,
		"duration_ms", time.Since(commitWriteStartedAt).Milliseconds(),
	)

	analyticsStartedAt := time.Now()
	if err := a.repos.RebuildModuleAnalytics(ctx, repo.ID); err != nil {
		return err
	}
	a.logger.Debug(
		"rebuilt module analytics",
		"sync_run_id", event.SyncRunID,
		"repository_id", event.RepositoryID,
		"duration_ms", time.Since(analyticsStartedAt).Milliseconds(),
	)

	coChangeStartedAt := time.Now()
	if err := a.repos.RebuildFileCoChange(ctx, repo.ID); err != nil {
		return err
	}
	a.logger.Debug(
		"rebuilt file co-change analytics",
		"sync_run_id", event.SyncRunID,
		"repository_id", event.RepositoryID,
		"duration_ms", time.Since(coChangeStartedAt).Milliseconds(),
	)

	moduleCoChangeStartedAt := time.Now()
	if err := a.repos.RebuildModuleCoChange(ctx, repo.ID); err != nil {
		return err
	}
	a.logger.Debug(
		"rebuilt module co-change analytics",
		"sync_run_id", event.SyncRunID,
		"repository_id", event.RepositoryID,
		"duration_ms", time.Since(moduleCoChangeStartedAt).Milliseconds(),
	)

	summary := repository.SyncRunSummary{
		ContributorsCount: len(contributors),
		CommitsCount:      commitStats.CommitsCount,
		CommitFilesCount:  commitStats.CommitFilesCount,
		ModulesCount:      commitStats.ModulesCount,
		FilesCount:        commitStats.FilesCount,
		DurationMS:        time.Since(syncStartedAt).Milliseconds(),
	}

	if err := a.repos.MarkSyncRunSucceeded(ctx, event.SyncRunID, event.RepositoryID, event.SyncRunCreatedAt, summary); err != nil {
		return err
	}

	a.logger.Info(
		"completed repository sync request",
		"sync_run_id", event.SyncRunID,
		"sync_run_created_at", event.SyncRunCreatedAt,
		"repository_id", event.RepositoryID,
		"contributors_count", len(contributors),
		"commits_count", commitStats.CommitsCount,
		"commit_files_count", commitStats.CommitFilesCount,
		"modules_count", commitStats.ModulesCount,
		"files_count", commitStats.FilesCount,
		"duration_ms", summary.DurationMS,
	)

	return nil
}
