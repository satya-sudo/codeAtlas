package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	serviceconfig "codeatlas/apps/sync-service/internal/config"
	"codeatlas/apps/sync-service/internal/integrations"
	"codeatlas/apps/sync-service/internal/repository"
	"codeatlas/packages/database"
	"codeatlas/packages/events"
	sharedgithub "codeatlas/packages/github"
	sharedkafka "codeatlas/packages/kafka"
	"codeatlas/packages/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	config                 serviceconfig.Config
	logger                 *slog.Logger
	repositorySyncConsumer sharedkafka.Consumer
	githubPushConsumer     sharedkafka.Consumer
	db                     *pgxpool.Pool
	repos                  *repository.RepositoryRepository
	github                 *integrations.GitHubApp
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

	var repositorySyncConsumer sharedkafka.Consumer
	var githubPushConsumer sharedkafka.Consumer
	if cfg.KafkaEnabled {
		repositorySyncConsumer = sharedkafka.NewConsumer(sharedkafka.ConsumerConfig{
			Brokers: cfg.KafkaBrokers,
			GroupID: cfg.RepositorySyncConsumerGroup,
			Topic:   cfg.RepositorySyncTopic,
		})
		githubPushConsumer = sharedkafka.NewConsumer(sharedkafka.ConsumerConfig{
			Brokers: cfg.KafkaBrokers,
			GroupID: cfg.GitHubPushConsumerGroup,
			Topic:   cfg.GitHubPushTopic,
		})
	}

	return &App{
		config:                 cfg,
		logger:                 log,
		repositorySyncConsumer: repositorySyncConsumer,
		githubPushConsumer:     githubPushConsumer,
		db:                     dbPool,
		repos:                  repository.NewRepositoryRepository(dbPool),
		github:                 githubApp,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	defer a.db.Close()

	if a.repositorySyncConsumer != nil || a.githubPushConsumer != nil {
		if a.repositorySyncConsumer != nil {
			defer a.repositorySyncConsumer.Close()
		}
		if a.githubPushConsumer != nil {
			defer a.githubPushConsumer.Close()
		}
		return a.runKafkaConsumers(ctx)
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

func (a *App) runKafkaConsumers(ctx context.Context) error {
	errCh := make(chan error, 2)

	if a.config.StaleSyncRunTimeout > 0 {
		if err := a.sweepStaleSyncRuns(ctx); err != nil {
			a.logger.Error("initial stale sync run sweep failed", "error", err)
		}

		if a.config.StaleSyncSweepInterval > 0 {
			go a.runStaleSyncRunSweeper(ctx)
		}
	}

	if a.repositorySyncConsumer != nil {
		go func() {
			errCh <- a.runRepositorySyncConsumer(ctx)
		}()
	}

	if a.githubPushConsumer != nil {
		go func() {
			errCh <- a.runGitHubPushConsumer(ctx)
		}()
	}

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("worker stopped")
			return nil
		case err := <-errCh:
			if err == nil || ctx.Err() != nil {
				a.logger.Info("worker stopped")
				return nil
			}
			return err
		}
	}
}

func (a *App) runRepositorySyncConsumer(ctx context.Context) error {
	a.logger.Info(
		"starting repository sync kafka consumer",
		"brokers", a.config.KafkaBrokers,
		"topic", a.config.RepositorySyncTopic,
		"group_id", a.config.RepositorySyncConsumerGroup,
	)

	for {
		msg, err := a.repositorySyncConsumer.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
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
			if err := a.repositorySyncConsumer.CommitMessages(ctx, msg); err != nil {
				return err
			}
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
				if err := a.repositorySyncConsumer.CommitMessages(ctx, msg); err != nil {
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
					if err := a.repositorySyncConsumer.CommitMessages(ctx, msg); err != nil {
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

		if err := a.repositorySyncConsumer.CommitMessages(ctx, msg); err != nil {
			return err
		}
	}
}

func (a *App) runGitHubPushConsumer(ctx context.Context) error {
	a.logger.Info(
		"starting github push kafka consumer",
		"brokers", a.config.KafkaBrokers,
		"topic", a.config.GitHubPushTopic,
		"group_id", a.config.GitHubPushConsumerGroup,
	)

	for {
		msg, err := a.githubPushConsumer.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		var event events.GitHubPushReceived
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			a.logger.Error("decode github push event", "error", err, "payload", string(msg.Value))
			if err := a.githubPushConsumer.CommitMessages(ctx, msg); err != nil {
				return err
			}
			continue
		}

		a.logger.Info(
			"received github push event",
			"delivery_id", event.DeliveryID,
			"repository_id", event.RepositoryID,
			"repository_full_name", event.RepositoryFullName,
			"ref", event.Ref,
			"before_sha", event.BeforeSHA,
			"after_sha", event.AfterSHA,
		)

		if err := a.processGitHubPushReceived(ctx, event); err != nil {
			a.logger.Error(
				"process github push event",
				"delivery_id", event.DeliveryID,
				"repository_id", event.RepositoryID,
				"error", err,
			)
		}

		if err := a.githubPushConsumer.CommitMessages(ctx, msg); err != nil {
			return err
		}
	}
}

func (a *App) runStaleSyncRunSweeper(ctx context.Context) {
	ticker := time.NewTicker(a.config.StaleSyncSweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.sweepStaleSyncRuns(ctx); err != nil {
				a.logger.Error("stale sync run sweep failed", "error", err)
			}
		}
	}
}

func (a *App) sweepStaleSyncRuns(ctx context.Context) error {
	count, err := a.repos.MarkStaleSyncRunsFailed(ctx, a.config.StaleSyncRunTimeout)
	if err != nil {
		return err
	}

	if count > 0 {
		a.logger.Warn(
			"marked stale sync runs failed",
			"count", count,
			"timeout", a.config.StaleSyncRunTimeout.String(),
		)
	}

	return nil
}

func (a *App) processRepositorySyncRequested(ctx context.Context, event events.RepositorySyncRequested) error {
	return a.processSyncRun(ctx, syncRunRequest{
		SyncRunID:        event.SyncRunID,
		SyncRunCreatedAt: event.SyncRunCreatedAt,
		RepositoryID:     event.RepositoryID,
		SyncType:         event.SyncType,
	}, nil)
}

type syncRunRequest struct {
	SyncRunID        int64
	SyncRunCreatedAt time.Time
	RepositoryID     int64
	SyncType         string
}

func (a *App) processGitHubPushReceived(ctx context.Context, event events.GitHubPushReceived) error {
	if !isDefaultBranchPush(event.Ref, event.RepositoryDefaultBranch) {
		a.logger.Info(
			"ignoring github push for non-default branch",
			"delivery_id", event.DeliveryID,
			"repository_id", event.RepositoryID,
			"ref", event.Ref,
			"default_branch", event.RepositoryDefaultBranch,
		)
		return nil
	}

	if isZeroGitCommitSHA(event.AfterSHA) {
		a.logger.Info(
			"ignoring github push for deleted ref",
			"delivery_id", event.DeliveryID,
			"repository_id", event.RepositoryID,
			"ref", event.Ref,
		)
		return nil
	}

	repo, err := a.repos.FindRepositoryForGitHubPush(ctx, event.RepositoryID, event.InstallationID)
	if err != nil {
		if errors.Is(err, repository.ErrRepositoryNotFound) {
			a.logger.Info(
				"ignoring github push for unconnected repository",
				"delivery_id", event.DeliveryID,
				"github_repository_id", event.RepositoryID,
				"installation_id", event.InstallationID,
			)
			return nil
		}
		return err
	}

	ref := event.Ref
	beforeSHA := event.BeforeSHA
	afterSHA := event.AfterSHA

	result, err := a.repos.CreateInternalSyncRunForRepository(ctx, repo.ID, "incremental", repository.SyncRunTrigger{
		Source:     "webhook",
		DeliveryID: &event.DeliveryID,
		Ref:        &ref,
		BeforeSHA:  &beforeSHA,
		AfterSHA:   &afterSHA,
	})
	if err != nil {
		return err
	}
	if result.RequestStatus != repository.SyncRunRequestStatusQueued {
		a.logger.Info(
			"skipping github push because sync run already active",
			"delivery_id", event.DeliveryID,
			"repository_id", repo.ID,
			"sync_run_id", result.SyncRun.ID,
			"request_status", result.RequestStatus,
		)
		return nil
	}

	return a.processSyncRun(ctx, syncRunRequest{
		SyncRunID:        result.SyncRun.ID,
		SyncRunCreatedAt: result.SyncRun.CreatedAt,
		RepositoryID:     repo.ID,
		SyncType:         "incremental",
	}, &event)
}

func (a *App) processSyncRun(ctx context.Context, request syncRunRequest, pushEvent *events.GitHubPushReceived) error {
	syncStartedAt := time.Now()

	repo, err := a.repos.FindRepositoryForSync(ctx, request.RepositoryID)
	if err != nil {
		return err
	}

	a.logger.Debug(
		"resolved repository for sync",
		"sync_run_id", request.SyncRunID,
		"sync_run_created_at", request.SyncRunCreatedAt,
		"repository_id", repo.ID,
		"repository_name", repo.Owner+"/"+repo.Name,
		"installation_id", repo.InstallationID,
		"sync_type", request.SyncType,
	)

	if repo.InstallationID == nil || *repo.InstallationID == 0 {
		return fmt.Errorf("repository %d has no github app installation id", repo.ID)
	}

	if err := a.repos.MarkSyncRunRunning(ctx, request.SyncRunID, request.RepositoryID, request.SyncRunCreatedAt); err != nil {
		return err
	}

	a.logger.Debug(
		"marked sync run running",
		"sync_run_id", request.SyncRunID,
		"sync_run_created_at", request.SyncRunCreatedAt,
		"repository_id", request.RepositoryID,
	)

	contributorsStartedAt := time.Now()
	contributors, err := a.github.ListRepositoryContributors(ctx, *repo.InstallationID, repo.Owner, repo.Name)
	if err != nil {
		return err
	}
	a.logger.Debug(
		"fetched repository contributors",
		"sync_run_id", request.SyncRunID,
		"repository_id", request.RepositoryID,
		"repository_name", repo.Owner+"/"+repo.Name,
		"contributors_count", len(contributors),
		"duration_ms", time.Since(contributorsStartedAt).Milliseconds(),
	)

	if err := a.repos.ReplaceContributors(ctx, repo.ID, contributors); err != nil {
		return err
	}
	a.logger.Debug(
		"stored repository contributors",
		"sync_run_id", request.SyncRunID,
		"repository_id", request.RepositoryID,
		"contributors_count", len(contributors),
	)

	commitsStartedAt := time.Now()
	var commits []sharedgithub.RepositoryCommit
	if pushEvent != nil && request.SyncType == "incremental" {
		commits, err = a.listIncrementalCommits(ctx, repo, *pushEvent)
		if err != nil {
			return err
		}
	} else {
		fullCommits, err := a.github.ListRepositoryCommits(ctx, *repo.InstallationID, repo.Owner, repo.Name)
		if err != nil {
			return err
		}
		commits = fullCommits
	}
	a.logger.Debug(
		"fetched repository commits",
		"sync_run_id", request.SyncRunID,
		"repository_id", request.RepositoryID,
		"repository_name", repo.Owner+"/"+repo.Name,
		"commits_count", len(commits),
		"duration_ms", time.Since(commitsStartedAt).Milliseconds(),
	)

	commitWriteStartedAt := time.Now()
	var commitStats repository.CommitImportStats
	if pushEvent != nil && request.SyncType == "incremental" {
		commitStats, err = a.repos.UpsertCommitDataIncremental(ctx, repo.ID, commits)
		if err != nil {
			return err
		}
	} else {
		commitStats, err = a.repos.ReplaceCommitData(ctx, repo.ID, commits)
		if err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}
	a.logger.Debug(
		"stored repository commit data",
		"sync_run_id", request.SyncRunID,
		"repository_id", request.RepositoryID,
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
		"sync_run_id", request.SyncRunID,
		"repository_id", request.RepositoryID,
		"duration_ms", time.Since(analyticsStartedAt).Milliseconds(),
	)

	coChangeStartedAt := time.Now()
	if err := a.repos.RebuildFileCoChange(ctx, repo.ID); err != nil {
		return err
	}
	a.logger.Debug(
		"rebuilt file co-change analytics",
		"sync_run_id", request.SyncRunID,
		"repository_id", request.RepositoryID,
		"duration_ms", time.Since(coChangeStartedAt).Milliseconds(),
	)

	moduleCoChangeStartedAt := time.Now()
	if err := a.repos.RebuildModuleCoChange(ctx, repo.ID); err != nil {
		return err
	}
	a.logger.Debug(
		"rebuilt module co-change analytics",
		"sync_run_id", request.SyncRunID,
		"repository_id", request.RepositoryID,
		"duration_ms", time.Since(moduleCoChangeStartedAt).Milliseconds(),
	)

	summary, err := a.repos.RepositorySnapshotSummary(ctx, repo.ID)
	if err != nil {
		return err
	}
	summary.DurationMS = time.Since(syncStartedAt).Milliseconds()

	if err := a.repos.MarkSyncRunSucceeded(ctx, request.SyncRunID, request.RepositoryID, request.SyncRunCreatedAt, summary); err != nil {
		return err
	}

	a.logger.Info(
		"completed repository sync request",
		"sync_run_id", request.SyncRunID,
		"sync_run_created_at", request.SyncRunCreatedAt,
		"repository_id", request.RepositoryID,
		"sync_type", request.SyncType,
		"contributors_count", summary.ContributorsCount,
		"commits_count", summary.CommitsCount,
		"commit_files_count", summary.CommitFilesCount,
		"modules_count", summary.ModulesCount,
		"files_count", summary.FilesCount,
		"duration_ms", summary.DurationMS,
	)

	return nil
}

func (a *App) listIncrementalCommits(ctx context.Context, repo repository.Repository, pushEvent events.GitHubPushReceived) ([]sharedgithub.RepositoryCommit, error) {
	if isZeroGitCommitSHA(pushEvent.BeforeSHA) {
		a.logger.Info(
			"falling back to full commit import for new-branch style push",
			"delivery_id", pushEvent.DeliveryID,
			"repository_id", repo.ID,
			"before_sha", pushEvent.BeforeSHA,
			"after_sha", pushEvent.AfterSHA,
		)
		return a.github.ListRepositoryCommits(ctx, *repo.InstallationID, repo.Owner, repo.Name)
	}

	commits, err := a.github.ListRepositoryCommitsInRange(ctx, *repo.InstallationID, repo.Owner, repo.Name, pushEvent.BeforeSHA, pushEvent.AfterSHA)
	if err != nil {
		a.logger.Warn(
			"falling back to full commit import after compare-range failure",
			"delivery_id", pushEvent.DeliveryID,
			"repository_id", repo.ID,
			"before_sha", pushEvent.BeforeSHA,
			"after_sha", pushEvent.AfterSHA,
			"error", err,
		)
		return a.github.ListRepositoryCommits(ctx, *repo.InstallationID, repo.Owner, repo.Name)
	}

	return commits, nil
}

func isDefaultBranchPush(ref string, defaultBranch string) bool {
	branch := strings.TrimSpace(defaultBranch)
	if branch == "" {
		return true
	}
	return strings.TrimSpace(ref) == "refs/heads/"+branch
}

func isZeroGitCommitSHA(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed != "" && strings.Trim(trimmed, "0") == ""
}
