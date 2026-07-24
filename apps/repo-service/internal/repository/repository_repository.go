package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"codeatlas/apps/repo-service/internal/repos"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrRepositoryNotFound = errors.New("repository not found")

type RepositoryRepository struct {
	db *pgxpool.Pool
}

func NewRepositoryRepository(db *pgxpool.Pool) *RepositoryRepository {
	return &RepositoryRepository{db: db}
}

func (r *RepositoryRepository) ConnectRepository(ctx context.Context, input repos.ConnectRepositoryInput) (repos.ConnectRepositoryResult, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return repos.ConnectRepositoryResult{}, fmt.Errorf("begin connect repository transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	fullName := input.Owner + "/" + input.Name
	connectionStatus := repos.ConnectionStatusCreated

	existingRepo, alreadyLinked, err := r.findRepositoryForUserByGitHubRepoID(ctx, tx, input.UserID, input.GitHubRepoID)
	if err != nil {
		return repos.ConnectRepositoryResult{}, fmt.Errorf("find existing repository connection: %w", err)
	}

	if alreadyLinked {
		if repositoryMetadataChanged(existingRepo, input, fullName) {
			connectionStatus = repos.ConnectionStatusUpdated
		} else {
			connectionStatus = repos.ConnectionStatusAlreadyConnected
		}
	}

	const upsertRepositoryQuery = `
		INSERT INTO repositories (
			github_repo_id,
			owner,
			name,
			full_name,
			default_branch,
			is_private,
			installation_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (github_repo_id)
		DO UPDATE SET
			owner = EXCLUDED.owner,
			name = EXCLUDED.name,
			full_name = EXCLUDED.full_name,
			default_branch = EXCLUDED.default_branch,
			is_private = EXCLUDED.is_private,
			installation_id = COALESCE(EXCLUDED.installation_id, repositories.installation_id),
			updated_at = NOW()
		RETURNING
			id,
			github_repo_id,
			owner,
			name,
			full_name,
			default_branch,
			is_private,
			installation_id,
			webhook_id,
			sync_status,
			last_synced_at,
			created_at,
			updated_at
	`

	var repo repos.Repository
	err = tx.QueryRow(
		ctx,
		upsertRepositoryQuery,
		input.GitHubRepoID,
		input.Owner,
		input.Name,
		fullName,
		input.DefaultBranch,
		input.IsPrivate,
		input.InstallationID,
	).Scan(
		&repo.ID,
		&repo.GitHubRepoID,
		&repo.Owner,
		&repo.Name,
		&repo.FullName,
		&repo.DefaultBranch,
		&repo.IsPrivate,
		&repo.InstallationID,
		&repo.WebhookID,
		&repo.SyncStatus,
		&repo.LastSyncedAt,
		&repo.CreatedAt,
		&repo.UpdatedAt,
	)
	if err != nil {
		return repos.ConnectRepositoryResult{}, fmt.Errorf("upsert repository: %w", err)
	}

	const upsertUserRepositoryQuery = `
		INSERT INTO user_repositories (user_id, repository_id, role)
		VALUES ($1, $2, 'owner')
		ON CONFLICT (user_id, repository_id)
		DO UPDATE SET role = EXCLUDED.role
	`

	if _, err := tx.Exec(ctx, upsertUserRepositoryQuery, input.UserID, repo.ID); err != nil {
		return repos.ConnectRepositoryResult{}, fmt.Errorf("link user to repository: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return repos.ConnectRepositoryResult{}, fmt.Errorf("commit connect repository transaction: %w", err)
	}

	return repos.ConnectRepositoryResult{
		Repository:       repo,
		ConnectionStatus: connectionStatus,
	}, nil
}

func (r *RepositoryRepository) ListRepositoriesForUser(ctx context.Context, userID int64) ([]repos.Repository, error) {
	const query = `
		SELECT
			r.id,
			r.github_repo_id,
			r.owner,
			r.name,
			r.full_name,
			r.default_branch,
			r.is_private,
			r.installation_id,
			r.webhook_id,
			r.sync_status,
			r.last_synced_at,
			r.created_at,
			r.updated_at
		FROM repositories r
		INNER JOIN user_repositories ur ON ur.repository_id = r.id
		WHERE ur.user_id = $1
		ORDER BY r.created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list repositories for user: %w", err)
	}
	defer rows.Close()

	var repositories []repos.Repository
	for rows.Next() {
		var repo repos.Repository
		if err := rows.Scan(
			&repo.ID,
			&repo.GitHubRepoID,
			&repo.Owner,
			&repo.Name,
			&repo.FullName,
			&repo.DefaultBranch,
			&repo.IsPrivate,
			&repo.InstallationID,
			&repo.WebhookID,
			&repo.SyncStatus,
			&repo.LastSyncedAt,
			&repo.CreatedAt,
			&repo.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan repository row: %w", err)
		}

		repositories = append(repositories, repo)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repositories: %w", err)
	}

	return repositories, nil
}

func (r *RepositoryRepository) UpdateRepositoryWebhookID(ctx context.Context, userID int64, repositoryID int64, webhookID int64) (repos.Repository, error) {
	const query = `
		UPDATE repositories r
		SET webhook_id = $3,
			updated_at = NOW()
		FROM user_repositories ur
		WHERE ur.user_id = $1
		  AND ur.repository_id = r.id
		  AND r.id = $2
		RETURNING
			r.id,
			r.github_repo_id,
			r.owner,
			r.name,
			r.full_name,
			r.default_branch,
			r.is_private,
			r.installation_id,
			r.webhook_id,
			r.sync_status,
			r.last_synced_at,
			r.created_at,
			r.updated_at
	`

	var repo repos.Repository
	err := r.db.QueryRow(ctx, query, userID, repositoryID, webhookID).Scan(
		&repo.ID,
		&repo.GitHubRepoID,
		&repo.Owner,
		&repo.Name,
		&repo.FullName,
		&repo.DefaultBranch,
		&repo.IsPrivate,
		&repo.InstallationID,
		&repo.WebhookID,
		&repo.SyncStatus,
		&repo.LastSyncedAt,
		&repo.CreatedAt,
		&repo.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repos.Repository{}, ErrRepositoryNotFound
		}
		return repos.Repository{}, fmt.Errorf("update repository webhook id: %w", err)
	}

	return repo, nil
}

func (r *RepositoryRepository) FindRepositoryForUser(ctx context.Context, userID int64, repositoryID int64) (repos.Repository, error) {
	const query = `
		SELECT
			r.id,
			r.github_repo_id,
			r.owner,
			r.name,
			r.full_name,
			r.default_branch,
			r.is_private,
			r.installation_id,
			r.webhook_id,
			r.sync_status,
			r.last_synced_at,
			r.created_at,
			r.updated_at
		FROM repositories r
		INNER JOIN user_repositories ur ON ur.repository_id = r.id
		WHERE ur.user_id = $1 AND r.id = $2
	`

	var repo repos.Repository
	err := r.db.QueryRow(ctx, query, userID, repositoryID).Scan(
		&repo.ID,
		&repo.GitHubRepoID,
		&repo.Owner,
		&repo.Name,
		&repo.FullName,
		&repo.DefaultBranch,
		&repo.IsPrivate,
		&repo.InstallationID,
		&repo.WebhookID,
		&repo.SyncStatus,
		&repo.LastSyncedAt,
		&repo.CreatedAt,
		&repo.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repos.Repository{}, ErrRepositoryNotFound
		}
		return repos.Repository{}, fmt.Errorf("find repository for user: %w", err)
	}

	return repo, nil
}

func (r *RepositoryRepository) CreateSyncRunForRepository(ctx context.Context, userID int64, repositoryID int64, syncType string) (repos.SyncRunRequestResult, error) {
	activeRun, found, err := r.findActiveSyncRunForRepository(ctx, userID, repositoryID)
	if err != nil {
		return repos.SyncRunRequestResult{}, fmt.Errorf("find active sync run: %w", err)
	}
	if found {
		requestStatus := repos.SyncRequestStatusAlreadyQueued
		if activeRun.Status == repos.SyncRunStatusRunning {
			requestStatus = repos.SyncRequestStatusAlreadyRunning
		}
		return repos.SyncRunRequestResult{
			SyncRun:       activeRun,
			RequestStatus: requestStatus,
		}, nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return repos.SyncRunRequestResult{}, fmt.Errorf("begin create sync run transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const query = `
		INSERT INTO repository_sync_runs (
			repository_id,
			sync_type,
			status
		)
		SELECT
			r.id,
			$3,
			'queued'
		FROM repositories r
		INNER JOIN user_repositories ur ON ur.repository_id = r.id
		WHERE ur.user_id = $1 AND r.id = $2
		RETURNING
			id,
			repository_id,
			sync_type,
			trigger_source,
			trigger_delivery_id,
			trigger_ref,
			before_sha,
			after_sha,
			status,
			error_message,
			contributors_count,
			commits_count,
			commit_files_count,
			modules_count,
			files_count,
			duration_ms,
			started_at,
			completed_at,
			created_at
	`

	var run repos.SyncRun
	err = tx.QueryRow(ctx, query, userID, repositoryID, syncType).Scan(
		&run.ID,
		&run.RepositoryID,
		&run.SyncType,
		&run.TriggerSource,
		&run.TriggerDeliveryID,
		&run.TriggerRef,
		&run.BeforeSHA,
		&run.AfterSHA,
		&run.Status,
		&run.ErrorMessage,
		&run.Summary.ContributorsCount,
		&run.Summary.CommitsCount,
		&run.Summary.CommitFilesCount,
		&run.Summary.ModulesCount,
		&run.Summary.FilesCount,
		&run.Summary.DurationMS,
		&run.StartedAt,
		&run.CompletedAt,
		&run.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			_ = tx.Rollback(ctx)

			activeRun, found, activeErr := r.findActiveSyncRunForRepository(ctx, userID, repositoryID)
			if activeErr != nil {
				return repos.SyncRunRequestResult{}, fmt.Errorf("re-fetch active sync run after unique violation: %w", activeErr)
			}
			if found {
				requestStatus := repos.SyncRequestStatusAlreadyQueued
				if activeRun.Status == repos.SyncRunStatusRunning {
					requestStatus = repos.SyncRequestStatusAlreadyRunning
				}
				return repos.SyncRunRequestResult{
					SyncRun:       activeRun,
					RequestStatus: requestStatus,
				}, nil
			}
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return repos.SyncRunRequestResult{}, ErrRepositoryNotFound
		}
		return repos.SyncRunRequestResult{}, fmt.Errorf("create sync run: %w", err)
	}

	tag, err := tx.Exec(
		ctx,
		`UPDATE repositories SET sync_status = 'pending', updated_at = NOW() WHERE id = $1`,
		repositoryID,
	)
	if err != nil {
		return repos.SyncRunRequestResult{}, fmt.Errorf("mark repository pending: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repos.SyncRunRequestResult{}, ErrRepositoryNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return repos.SyncRunRequestResult{}, fmt.Errorf("commit create sync run transaction: %w", err)
	}

	return repos.SyncRunRequestResult{
		SyncRun:       run,
		RequestStatus: repos.SyncRequestStatusQueued,
	}, nil
}

func (r *RepositoryRepository) ListSyncRunsForRepository(ctx context.Context, userID int64, repositoryID int64) ([]repos.SyncRun, error) {
	const query = `
		SELECT
			sr.id,
			sr.repository_id,
			sr.sync_type,
			sr.trigger_source,
			sr.trigger_delivery_id,
			sr.trigger_ref,
			sr.before_sha,
			sr.after_sha,
			sr.status,
			sr.error_message,
			sr.contributors_count,
			sr.commits_count,
			sr.commit_files_count,
			sr.modules_count,
			sr.files_count,
			sr.duration_ms,
			sr.started_at,
			sr.completed_at,
			sr.created_at
		FROM repository_sync_runs sr
		INNER JOIN user_repositories ur ON ur.repository_id = sr.repository_id
		WHERE ur.user_id = $1 AND sr.repository_id = $2
		ORDER BY sr.created_at DESC, sr.id DESC
	`

	rows, err := r.db.Query(ctx, query, userID, repositoryID)
	if err != nil {
		return nil, fmt.Errorf("list sync runs for repository: %w", err)
	}
	defer rows.Close()

	var runs []repos.SyncRun
	for rows.Next() {
		var run repos.SyncRun
		if err := rows.Scan(
			&run.ID,
			&run.RepositoryID,
			&run.SyncType,
			&run.TriggerSource,
			&run.TriggerDeliveryID,
			&run.TriggerRef,
			&run.BeforeSHA,
			&run.AfterSHA,
			&run.Status,
			&run.ErrorMessage,
			&run.Summary.ContributorsCount,
			&run.Summary.CommitsCount,
			&run.Summary.CommitFilesCount,
			&run.Summary.ModulesCount,
			&run.Summary.FilesCount,
			&run.Summary.DurationMS,
			&run.StartedAt,
			&run.CompletedAt,
			&run.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan sync run row: %w", err)
		}
		runs = append(runs, run)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sync runs: %w", err)
	}

	return runs, nil
}

func (r *RepositoryRepository) ListLatestSyncStatusForUser(ctx context.Context, userID int64) ([]repos.RepositorySyncStatus, error) {
	const query = `
		WITH latest_runs AS (
			SELECT
				sr.id,
				sr.repository_id,
				sr.sync_type,
				sr.trigger_source,
				sr.trigger_delivery_id,
				sr.trigger_ref,
				sr.before_sha,
				sr.after_sha,
				sr.status,
				sr.error_message,
				sr.contributors_count,
				sr.commits_count,
				sr.commit_files_count,
				sr.modules_count,
				sr.files_count,
				sr.duration_ms,
				sr.started_at,
				sr.completed_at,
				sr.created_at,
				ROW_NUMBER() OVER (
					PARTITION BY sr.repository_id
					ORDER BY sr.created_at DESC, sr.id DESC
				) AS row_num
			FROM repository_sync_runs sr
		)
		SELECT
			r.id,
			r.github_repo_id,
			r.owner,
			r.name,
			r.full_name,
			r.default_branch,
			r.is_private,
			r.installation_id,
			r.webhook_id,
			r.sync_status,
			r.last_synced_at,
			r.created_at,
			r.updated_at,
			lr.id,
			lr.repository_id,
			lr.sync_type,
			lr.trigger_source,
			lr.trigger_delivery_id,
			lr.trigger_ref,
			lr.before_sha,
			lr.after_sha,
			lr.status,
			lr.error_message,
			lr.contributors_count,
			lr.commits_count,
			lr.commit_files_count,
			lr.modules_count,
			lr.files_count,
			lr.duration_ms,
			lr.started_at,
			lr.completed_at,
			lr.created_at
		FROM repositories r
		INNER JOIN user_repositories ur ON ur.repository_id = r.id
		LEFT JOIN latest_runs lr
			ON lr.repository_id = r.id
		   AND lr.row_num = 1
		WHERE ur.user_id = $1
		ORDER BY r.created_at DESC, r.id DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list latest sync status for user: %w", err)
	}
	defer rows.Close()

	statuses := make([]repos.RepositorySyncStatus, 0)
	for rows.Next() {
		var status repos.RepositorySyncStatus
		var latestSyncRunID *int64
		var latestSyncRunRepositoryID *int64
		var latestSyncRunSyncType *string
		var latestSyncRunTriggerSource *string
		var latestSyncRunTriggerDeliveryID *string
		var latestSyncRunTriggerRef *string
		var latestSyncRunBeforeSHA *string
		var latestSyncRunAfterSHA *string
		var latestSyncRunStatus *string
		var latestSyncRunContributorsCount *int
		var latestSyncRunCommitsCount *int
		var latestSyncRunCommitFilesCount *int
		var latestSyncRunModulesCount *int
		var latestSyncRunFilesCount *int
		var latestSyncRunCreatedAt *time.Time
		var latestSyncRunStartedAt *time.Time
		var latestSyncRunCompletedAt *time.Time
		var latestSyncRunErrorMessage *string
		var latestSyncRunDurationMS *int64

		if err := rows.Scan(
			&status.Repository.ID,
			&status.Repository.GitHubRepoID,
			&status.Repository.Owner,
			&status.Repository.Name,
			&status.Repository.FullName,
			&status.Repository.DefaultBranch,
			&status.Repository.IsPrivate,
			&status.Repository.InstallationID,
			&status.Repository.WebhookID,
			&status.Repository.SyncStatus,
			&status.Repository.LastSyncedAt,
			&status.Repository.CreatedAt,
			&status.Repository.UpdatedAt,
			&latestSyncRunID,
			&latestSyncRunRepositoryID,
			&latestSyncRunSyncType,
			&latestSyncRunTriggerSource,
			&latestSyncRunTriggerDeliveryID,
			&latestSyncRunTriggerRef,
			&latestSyncRunBeforeSHA,
			&latestSyncRunAfterSHA,
			&latestSyncRunStatus,
			&latestSyncRunErrorMessage,
			&latestSyncRunContributorsCount,
			&latestSyncRunCommitsCount,
			&latestSyncRunCommitFilesCount,
			&latestSyncRunModulesCount,
			&latestSyncRunFilesCount,
			&latestSyncRunDurationMS,
			&latestSyncRunStartedAt,
			&latestSyncRunCompletedAt,
			&latestSyncRunCreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan latest sync status: %w", err)
		}

		if latestSyncRunID != nil && latestSyncRunRepositoryID != nil && latestSyncRunSyncType != nil && latestSyncRunStatus != nil && latestSyncRunCreatedAt != nil {
			run := repos.SyncRun{
				ID:                *latestSyncRunID,
				RepositoryID:      *latestSyncRunRepositoryID,
				SyncType:          *latestSyncRunSyncType,
				TriggerSource:     valueOrDefault(latestSyncRunTriggerSource, "manual"),
				TriggerDeliveryID: latestSyncRunTriggerDeliveryID,
				TriggerRef:        latestSyncRunTriggerRef,
				BeforeSHA:         latestSyncRunBeforeSHA,
				AfterSHA:          latestSyncRunAfterSHA,
				Status:            *latestSyncRunStatus,
				ErrorMessage:      latestSyncRunErrorMessage,
				StartedAt:         latestSyncRunStartedAt,
				CompletedAt:       latestSyncRunCompletedAt,
				CreatedAt:         *latestSyncRunCreatedAt,
			}
			if latestSyncRunContributorsCount != nil {
				run.Summary.ContributorsCount = *latestSyncRunContributorsCount
			}
			if latestSyncRunCommitsCount != nil {
				run.Summary.CommitsCount = *latestSyncRunCommitsCount
			}
			if latestSyncRunCommitFilesCount != nil {
				run.Summary.CommitFilesCount = *latestSyncRunCommitFilesCount
			}
			if latestSyncRunModulesCount != nil {
				run.Summary.ModulesCount = *latestSyncRunModulesCount
			}
			if latestSyncRunFilesCount != nil {
				run.Summary.FilesCount = *latestSyncRunFilesCount
			}
			run.Summary.DurationMS = latestSyncRunDurationMS
			status.LatestSyncRun = &run
		}

		statuses = append(statuses, status)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate latest sync statuses: %w", err)
	}

	return statuses, nil
}

func (r *RepositoryRepository) FindSyncRunForRepository(ctx context.Context, userID int64, repositoryID int64, runID int64) (repos.SyncRun, error) {
	const query = `
		SELECT
			sr.id,
			sr.repository_id,
			sr.sync_type,
			sr.trigger_source,
			sr.trigger_delivery_id,
			sr.trigger_ref,
			sr.before_sha,
			sr.after_sha,
			sr.status,
			sr.error_message,
			sr.contributors_count,
			sr.commits_count,
			sr.commit_files_count,
			sr.modules_count,
			sr.files_count,
			sr.duration_ms,
			sr.started_at,
			sr.completed_at,
			sr.created_at
		FROM repository_sync_runs sr
		INNER JOIN user_repositories ur ON ur.repository_id = sr.repository_id
		WHERE ur.user_id = $1 AND sr.repository_id = $2 AND sr.id = $3
	`

	var run repos.SyncRun
	err := r.db.QueryRow(ctx, query, userID, repositoryID, runID).Scan(
		&run.ID,
		&run.RepositoryID,
		&run.SyncType,
		&run.TriggerSource,
		&run.TriggerDeliveryID,
		&run.TriggerRef,
		&run.BeforeSHA,
		&run.AfterSHA,
		&run.Status,
		&run.ErrorMessage,
		&run.Summary.ContributorsCount,
		&run.Summary.CommitsCount,
		&run.Summary.CommitFilesCount,
		&run.Summary.ModulesCount,
		&run.Summary.FilesCount,
		&run.Summary.DurationMS,
		&run.StartedAt,
		&run.CompletedAt,
		&run.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repos.SyncRun{}, ErrRepositoryNotFound
		}
		return repos.SyncRun{}, fmt.Errorf("find sync run for repository: %w", err)
	}

	return run, nil
}

func (r *RepositoryRepository) ListContributorsForRepository(ctx context.Context, userID int64, repositoryID int64) ([]repos.Contributor, error) {
	const query = `
		SELECT
			rc.id,
			rc.repository_id,
			rc.github_user_id,
			rc.username,
			rc.avatar_url,
			rc.contributions_count,
			rc.last_seen_at,
			rc.created_at,
			rc.updated_at
		FROM repository_contributors rc
		INNER JOIN user_repositories ur ON ur.repository_id = rc.repository_id
		WHERE ur.user_id = $1 AND rc.repository_id = $2
		ORDER BY rc.contributions_count DESC, rc.username ASC
	`

	rows, err := r.db.Query(ctx, query, userID, repositoryID)
	if err != nil {
		return nil, fmt.Errorf("list contributors for repository: %w", err)
	}
	defer rows.Close()

	var contributors []repos.Contributor
	for rows.Next() {
		var contributor repos.Contributor
		if err := rows.Scan(
			&contributor.ID,
			&contributor.RepositoryID,
			&contributor.GitHubUserID,
			&contributor.Username,
			&contributor.AvatarURL,
			&contributor.ContributionsCount,
			&contributor.LastSeenAt,
			&contributor.CreatedAt,
			&contributor.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan contributor row: %w", err)
		}
		contributors = append(contributors, contributor)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contributors: %w", err)
	}

	return contributors, nil
}

func (r *RepositoryRepository) BuildDashboardForRepository(ctx context.Context, userID int64, repositoryID int64) (repos.RepositoryDashboard, error) {
	repo, err := r.FindRepositoryForUser(ctx, userID, repositoryID)
	if err != nil {
		return repos.RepositoryDashboard{}, err
	}

	runs, err := r.ListSyncRunsForRepository(ctx, userID, repositoryID)
	if err != nil {
		return repos.RepositoryDashboard{}, fmt.Errorf("list sync runs for dashboard: %w", err)
	}

	contributors, err := r.ListContributorsForRepository(ctx, userID, repositoryID)
	if err != nil {
		return repos.RepositoryDashboard{}, fmt.Errorf("list contributors for dashboard: %w", err)
	}

	overview, err := r.buildOverviewForRepository(ctx, userID, repositoryID)
	if err != nil {
		return repos.RepositoryDashboard{}, fmt.Errorf("build overview for dashboard: %w", err)
	}

	hotspots, err := r.listHotspotsForRepository(ctx, userID, repositoryID, 8)
	if err != nil {
		return repos.RepositoryDashboard{}, fmt.Errorf("list hotspots for dashboard: %w", err)
	}

	var latestSyncRun *repos.SyncRun
	if len(runs) > 0 {
		latestSyncRun = &runs[0]
	}

	recentSyncRuns := runs
	if len(recentSyncRuns) > 5 {
		recentSyncRuns = recentSyncRuns[:5]
	}

	topContributors := contributors
	if len(topContributors) > 5 {
		topContributors = topContributors[:5]
	}

	return repos.RepositoryDashboard{
		Repository:      repo,
		Overview:        overview,
		Hotspots:        hotspots,
		LatestSyncRun:   latestSyncRun,
		RecentSyncRuns:  recentSyncRuns,
		TopContributors: topContributors,
	}, nil
}

func (r *RepositoryRepository) buildOverviewForRepository(ctx context.Context, userID int64, repositoryID int64) (repos.RepositoryOverview, error) {
	const query = `
		SELECT
			COALESCE((SELECT COUNT(*) FROM commits c WHERE c.repository_id = r.id), 0) AS total_commits,
			COALESCE((SELECT COUNT(*) FROM repository_contributors rc WHERE rc.repository_id = r.id), 0) AS total_contributors,
			COALESCE((SELECT COUNT(*) FROM files f WHERE f.repository_id = r.id AND f.is_deleted = FALSE), 0) AS total_files,
			COALESCE((SELECT COUNT(*) FROM modules m WHERE m.repository_id = r.id), 0) AS total_modules,
			r.last_synced_at,
			(
				SELECT sr.duration_ms
				FROM repository_sync_runs sr
				WHERE sr.repository_id = r.id
				  AND sr.status = 'succeeded'
				ORDER BY sr.completed_at DESC NULLS LAST, sr.id DESC
				LIMIT 1
			) AS latest_sync_duration_ms
		FROM repositories r
		INNER JOIN user_repositories ur ON ur.repository_id = r.id
		WHERE ur.user_id = $1 AND r.id = $2
	`

	var overview repos.RepositoryOverview
	err := r.db.QueryRow(ctx, query, userID, repositoryID).Scan(
		&overview.TotalCommits,
		&overview.TotalContributors,
		&overview.TotalFiles,
		&overview.TotalModules,
		&overview.LastSyncedAt,
		&overview.LatestSyncDurationMS,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repos.RepositoryOverview{}, ErrRepositoryNotFound
		}
		return repos.RepositoryOverview{}, fmt.Errorf("query repository overview: %w", err)
	}

	return overview, nil
}

func (r *RepositoryRepository) ListHotspotsForRepository(ctx context.Context, userID int64, repositoryID int64, limit int) ([]repos.RepositoryHotspot, error) {
	if _, err := r.FindRepositoryForUser(ctx, userID, repositoryID); err != nil {
		return nil, err
	}

	return r.listHotspotsForRepository(ctx, userID, repositoryID, limit)
}

func (r *RepositoryRepository) ListCoChangeForRepository(ctx context.Context, userID int64, repositoryID int64, limit int, pathFilter string) ([]repos.RepositoryCoChange, error) {
	if _, err := r.FindRepositoryForUser(ctx, userID, repositoryID); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 20
	}

	const query = `
		SELECT
			fc.left_path,
			fc.right_path,
			fc.cochange_count,
			fc.last_cochanged_at
		FROM file_cochange fc
		INNER JOIN user_repositories ur ON ur.repository_id = fc.repository_id
		WHERE ur.user_id = $1
		  AND fc.repository_id = $2
		  AND ($3 = '' OR fc.left_path = $3 OR fc.right_path = $3)
		ORDER BY fc.cochange_count DESC, fc.last_cochanged_at DESC NULLS LAST, fc.left_path ASC, fc.right_path ASC
		LIMIT $4
	`

	rows, err := r.db.Query(ctx, query, userID, repositoryID, pathFilter, limit)
	if err != nil {
		return nil, fmt.Errorf("query repository co-change: %w", err)
	}
	defer rows.Close()

	pairs := make([]repos.RepositoryCoChange, 0, limit)
	for rows.Next() {
		var pair repos.RepositoryCoChange
		if err := rows.Scan(
			&pair.LeftPath,
			&pair.RightPath,
			&pair.CoChangeCount,
			&pair.LastCochangedAt,
		); err != nil {
			return nil, fmt.Errorf("scan repository co-change: %w", err)
		}
		pairs = append(pairs, pair)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repository co-change: %w", err)
	}

	return pairs, nil
}

func (r *RepositoryRepository) ListModuleCoChangeForRepository(ctx context.Context, userID int64, repositoryID int64, limit int) ([]repos.ModuleCoChange, error) {
	if _, err := r.FindRepositoryForUser(ctx, userID, repositoryID); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 20
	}

	const query = `
		SELECT
			mc.left_module_id,
			mc.left_module_name,
			mc.left_path_prefix,
			mc.right_module_id,
			mc.right_module_name,
			mc.right_path_prefix,
			mc.cochange_count,
			mc.last_cochanged_at
		FROM module_cochange mc
		INNER JOIN user_repositories ur ON ur.repository_id = mc.repository_id
		WHERE ur.user_id = $1
		  AND mc.repository_id = $2
		ORDER BY mc.cochange_count DESC, mc.last_cochanged_at DESC NULLS LAST, mc.left_path_prefix ASC, mc.right_path_prefix ASC
		LIMIT $3
	`

	rows, err := r.db.Query(ctx, query, userID, repositoryID, limit)
	if err != nil {
		return nil, fmt.Errorf("query module co-change: %w", err)
	}
	defer rows.Close()

	pairs := make([]repos.ModuleCoChange, 0, limit)
	for rows.Next() {
		var pair repos.ModuleCoChange
		if err := rows.Scan(
			&pair.LeftModuleID,
			&pair.LeftModuleName,
			&pair.LeftPathPrefix,
			&pair.RightModuleID,
			&pair.RightModuleName,
			&pair.RightPathPrefix,
			&pair.CoChangeCount,
			&pair.LastCochangedAt,
		); err != nil {
			return nil, fmt.Errorf("scan module co-change: %w", err)
		}
		pairs = append(pairs, pair)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate module co-change: %w", err)
	}

	return pairs, nil
}

func (r *RepositoryRepository) listHotspotsForRepository(ctx context.Context, userID int64, repositoryID int64, limit int) ([]repos.RepositoryHotspot, error) {
	if limit <= 0 {
		limit = 8
	}

	const query = `
		SELECT
			cf.path,
			COUNT(DISTINCT cf.commit_id) AS commit_count,
			COALESCE(SUM(cf.additions), 0) AS lines_added,
			COALESCE(SUM(cf.deletions), 0) AS lines_deleted,
			COALESCE(SUM(cf.changes), 0) AS churn
		FROM commit_files cf
		INNER JOIN user_repositories ur ON ur.repository_id = cf.repository_id
		WHERE ur.user_id = $1
		  AND cf.repository_id = $2
		GROUP BY cf.path
		ORDER BY churn DESC, commit_count DESC, cf.path ASC
		LIMIT $3
	`

	rows, err := r.db.Query(ctx, query, userID, repositoryID, limit)
	if err != nil {
		return nil, fmt.Errorf("query repository hotspots: %w", err)
	}
	defer rows.Close()

	hotspots := make([]repos.RepositoryHotspot, 0, limit)
	for rows.Next() {
		var hotspot repos.RepositoryHotspot
		if err := rows.Scan(
			&hotspot.Path,
			&hotspot.CommitCount,
			&hotspot.LinesAdded,
			&hotspot.LinesDeleted,
			&hotspot.Churn,
		); err != nil {
			return nil, fmt.Errorf("scan repository hotspot: %w", err)
		}
		hotspots = append(hotspots, hotspot)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repository hotspots: %w", err)
	}

	return hotspots, nil
}

func (r *RepositoryRepository) ListModuleOwnershipForRepository(ctx context.Context, userID int64, repositoryID int64) ([]repos.ModuleOwnershipInsight, error) {
	if _, err := r.FindRepositoryForUser(ctx, userID, repositoryID); err != nil {
		return nil, err
	}

	const query = `
		SELECT
			m.id,
			m.name,
			m.path_prefix,
			COALESCE(mm.bus_factor, 0) AS bus_factor,
			COALESCE(mm.active_contributors, 0) AS active_contributors,
			COALESCE(mm.top_owner_percent::double precision, 0) AS top_owner_percent,
			COALESCE(mm.risk, 'unknown') AS risk,
			mo.github_user_id,
			mo.username,
			COALESCE(mo.ownership_percent::double precision, 0) AS ownership_percent,
			COALESCE(mo.commit_count, 0) AS commit_count,
			COALESCE(mo.changes_count, 0) AS changes_count,
			COALESCE(mo.files_touched_count, 0) AS files_touched_count,
			mo.rank
		FROM modules m
		INNER JOIN user_repositories ur
			ON ur.repository_id = m.repository_id
		LEFT JOIN module_metrics mm
			ON mm.module_id = m.id
		LEFT JOIN module_ownership mo
			ON mo.module_id = m.id
		WHERE ur.user_id = $1
		  AND m.repository_id = $2
		ORDER BY m.path_prefix ASC, m.name ASC, mo.rank ASC NULLS LAST, mo.username ASC NULLS LAST
	`

	rows, err := r.db.Query(ctx, query, userID, repositoryID)
	if err != nil {
		return nil, fmt.Errorf("query module ownership: %w", err)
	}
	defer rows.Close()

	modules := make([]repos.ModuleOwnershipInsight, 0)
	moduleIndexByID := make(map[int64]int)

	for rows.Next() {
		var (
			moduleID           int64
			moduleName         string
			pathPrefix         string
			busFactor          int
			activeContributors int
			topOwnerPercent    float64
			risk               string
			githubUserID       *int64
			username           *string
			ownershipPercent   float64
			commitCount        int
			changesCount       int
			filesTouchedCount  int
			rank               *int
		)

		if err := rows.Scan(
			&moduleID,
			&moduleName,
			&pathPrefix,
			&busFactor,
			&activeContributors,
			&topOwnerPercent,
			&risk,
			&githubUserID,
			&username,
			&ownershipPercent,
			&commitCount,
			&changesCount,
			&filesTouchedCount,
			&rank,
		); err != nil {
			return nil, fmt.Errorf("scan module ownership: %w", err)
		}

		index, ok := moduleIndexByID[moduleID]
		if !ok {
			modules = append(modules, repos.ModuleOwnershipInsight{
				ModuleID:           moduleID,
				ModuleName:         moduleName,
				PathPrefix:         pathPrefix,
				BusFactor:          busFactor,
				ActiveContributors: activeContributors,
				TopOwnerPercent:    topOwnerPercent,
				Risk:               risk,
				Owners:             []repos.ModuleOwnershipEntry{},
			})
			index = len(modules) - 1
			moduleIndexByID[moduleID] = index
		}

		if username == nil || *username == "" || rank == nil {
			continue
		}

		modules[index].Owners = append(modules[index].Owners, repos.ModuleOwnershipEntry{
			GitHubUserID:      githubUserID,
			Username:          *username,
			OwnershipPercent:  ownershipPercent,
			CommitCount:       commitCount,
			ChangesCount:      changesCount,
			FilesTouchedCount: filesTouchedCount,
			Rank:              *rank,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate module ownership rows: %w", err)
	}

	return modules, nil
}

func (r *RepositoryRepository) ListModuleExpertiseForRepository(ctx context.Context, userID int64, repositoryID int64) ([]repos.ModuleExpertiseInsight, error) {
	if _, err := r.FindRepositoryForUser(ctx, userID, repositoryID); err != nil {
		return nil, err
	}

	const query = `
		SELECT
			m.id,
			m.name,
			m.path_prefix,
			me.github_user_id,
			me.username,
			me.score,
			me.raw_score,
			me.commit_count,
			me.files_touched_count,
			me.recent_commit_count,
			me.last_commit_at,
			me.rank
		FROM modules m
		INNER JOIN user_repositories ur
			ON ur.repository_id = m.repository_id
		LEFT JOIN module_expertise me
			ON me.module_id = m.id
		WHERE ur.user_id = $1
		  AND m.repository_id = $2
		ORDER BY m.path_prefix ASC, m.name ASC, me.rank ASC NULLS LAST, me.username ASC NULLS LAST
	`

	rows, err := r.db.Query(ctx, query, userID, repositoryID)
	if err != nil {
		return nil, fmt.Errorf("query module expertise: %w", err)
	}
	defer rows.Close()

	modules := make([]repos.ModuleExpertiseInsight, 0)
	moduleIndexByID := make(map[int64]int)

	for rows.Next() {
		var (
			moduleID          int64
			moduleName        string
			pathPrefix        string
			githubUserID      *int64
			username          *string
			score             *int
			rawScore          *int
			commitCount       *int
			filesTouchedCount *int
			recentCommitCount *int
			lastCommitAt      *time.Time
			rank              *int
		)

		if err := rows.Scan(
			&moduleID,
			&moduleName,
			&pathPrefix,
			&githubUserID,
			&username,
			&score,
			&rawScore,
			&commitCount,
			&filesTouchedCount,
			&recentCommitCount,
			&lastCommitAt,
			&rank,
		); err != nil {
			return nil, fmt.Errorf("scan module expertise: %w", err)
		}

		index, ok := moduleIndexByID[moduleID]
		if !ok {
			modules = append(modules, repos.ModuleExpertiseInsight{
				ModuleID:   moduleID,
				ModuleName: moduleName,
				PathPrefix: pathPrefix,
				Experts:    []repos.ModuleExpertiseEntry{},
			})
			index = len(modules) - 1
			moduleIndexByID[moduleID] = index
		}

		if username == nil || *username == "" || rank == nil || score == nil || rawScore == nil || commitCount == nil || filesTouchedCount == nil || recentCommitCount == nil {
			continue
		}

		modules[index].Experts = append(modules[index].Experts, repos.ModuleExpertiseEntry{
			GitHubUserID:      githubUserID,
			Username:          *username,
			Score:             *score,
			RawScore:          *rawScore,
			CommitCount:       *commitCount,
			FilesTouchedCount: *filesTouchedCount,
			RecentCommitCount: *recentCommitCount,
			LastCommitAt:      lastCommitAt,
			Rank:              *rank,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate module expertise rows: %w", err)
	}

	return modules, nil
}

func (r *RepositoryRepository) ListModuleBusFactorForRepository(ctx context.Context, userID int64, repositoryID int64) ([]repos.ModuleBusFactor, error) {
	if _, err := r.FindRepositoryForUser(ctx, userID, repositoryID); err != nil {
		return nil, err
	}

	const query = `
		SELECT
			m.id,
			m.name,
			m.path_prefix,
			COALESCE(mm.bus_factor, 0) AS bus_factor,
			COALESCE(mm.active_contributors, 0) AS active_contributors,
			COALESCE(mm.top_owner_percent::double precision, 0) AS top_owner_percent,
			COALESCE(mm.risk, 'unknown') AS risk
		FROM modules m
		INNER JOIN user_repositories ur
			ON ur.repository_id = m.repository_id
		LEFT JOIN module_metrics mm
			ON mm.module_id = m.id
		WHERE ur.user_id = $1
		  AND m.repository_id = $2
		ORDER BY
			CASE COALESCE(mm.risk, 'unknown')
				WHEN 'high' THEN 1
				WHEN 'medium' THEN 2
				WHEN 'low' THEN 3
				ELSE 4
			END,
			m.path_prefix ASC,
			m.name ASC
	`

	rows, err := r.db.Query(ctx, query, userID, repositoryID)
	if err != nil {
		return nil, fmt.Errorf("query module bus factor: %w", err)
	}
	defer rows.Close()

	modules := make([]repos.ModuleBusFactor, 0)
	for rows.Next() {
		var module repos.ModuleBusFactor
		if err := rows.Scan(
			&module.ModuleID,
			&module.ModuleName,
			&module.PathPrefix,
			&module.BusFactor,
			&module.ActiveContributors,
			&module.TopOwnerPercent,
			&module.Risk,
		); err != nil {
			return nil, fmt.Errorf("scan module bus factor: %w", err)
		}
		modules = append(modules, module)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate module bus factor rows: %w", err)
	}

	return modules, nil
}

func (r *RepositoryRepository) findActiveSyncRunForRepository(ctx context.Context, userID int64, repositoryID int64) (repos.SyncRun, bool, error) {
	const query = `
		SELECT
			sr.id,
			sr.repository_id,
			sr.sync_type,
			sr.trigger_source,
			sr.trigger_delivery_id,
			sr.trigger_ref,
			sr.before_sha,
			sr.after_sha,
			sr.status,
			sr.error_message,
			sr.contributors_count,
			sr.commits_count,
			sr.commit_files_count,
			sr.modules_count,
			sr.files_count,
			sr.duration_ms,
			sr.started_at,
			sr.completed_at,
			sr.created_at
		FROM repository_sync_runs sr
		INNER JOIN user_repositories ur ON ur.repository_id = sr.repository_id
		WHERE ur.user_id = $1
		  AND sr.repository_id = $2
		  AND sr.status IN ('queued', 'running')
		ORDER BY sr.created_at DESC, sr.id DESC
		LIMIT 1
	`

	var run repos.SyncRun
	err := r.db.QueryRow(ctx, query, userID, repositoryID).Scan(
		&run.ID,
		&run.RepositoryID,
		&run.SyncType,
		&run.TriggerSource,
		&run.TriggerDeliveryID,
		&run.TriggerRef,
		&run.BeforeSHA,
		&run.AfterSHA,
		&run.Status,
		&run.ErrorMessage,
		&run.Summary.ContributorsCount,
		&run.Summary.CommitsCount,
		&run.Summary.CommitFilesCount,
		&run.Summary.ModulesCount,
		&run.Summary.FilesCount,
		&run.Summary.DurationMS,
		&run.StartedAt,
		&run.CompletedAt,
		&run.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repos.SyncRun{}, false, nil
		}
		return repos.SyncRun{}, false, err
	}

	return run, true, nil
}

func (r *RepositoryRepository) findRepositoryForUserByGitHubRepoID(ctx context.Context, tx pgx.Tx, userID int64, githubRepoID int64) (repos.Repository, bool, error) {
	const query = `
		SELECT
			r.id,
			r.github_repo_id,
			r.owner,
			r.name,
			r.full_name,
			r.default_branch,
			r.is_private,
			r.installation_id,
			r.webhook_id,
			r.sync_status,
			r.last_synced_at,
			r.created_at,
			r.updated_at
		FROM repositories r
		INNER JOIN user_repositories ur ON ur.repository_id = r.id
		WHERE ur.user_id = $1 AND r.github_repo_id = $2
	`

	var repo repos.Repository
	err := tx.QueryRow(ctx, query, userID, githubRepoID).Scan(
		&repo.ID,
		&repo.GitHubRepoID,
		&repo.Owner,
		&repo.Name,
		&repo.FullName,
		&repo.DefaultBranch,
		&repo.IsPrivate,
		&repo.InstallationID,
		&repo.WebhookID,
		&repo.SyncStatus,
		&repo.LastSyncedAt,
		&repo.CreatedAt,
		&repo.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return repos.Repository{}, false, nil
		}
		return repos.Repository{}, false, err
	}

	return repo, true, nil
}

func repositoryMetadataChanged(existing repos.Repository, input repos.ConnectRepositoryInput, fullName string) bool {
	if existing.Owner != input.Owner || existing.Name != input.Name || existing.FullName != fullName || existing.DefaultBranch != input.DefaultBranch || existing.IsPrivate != input.IsPrivate {
		return true
	}

	if existing.InstallationID == nil && input.InstallationID == nil {
		return false
	}
	if existing.InstallationID == nil || input.InstallationID == nil {
		return true
	}

	return *existing.InstallationID != *input.InstallationID
}

func valueOrDefault(value *string, fallback string) string {
	if value == nil {
		return fallback
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return fallback
	}

	return trimmed
}
