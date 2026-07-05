package repository

import (
	"context"
	"errors"
	"fmt"

	"codeatlas/apps/repo-service/internal/repos"
	"github.com/jackc/pgx/v5"
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

func (r *RepositoryRepository) CreateSyncRunForRepository(ctx context.Context, userID int64, repositoryID int64, syncType string) (repos.SyncRun, error) {
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
			status,
			error_message,
			started_at,
			completed_at,
			created_at
	`

	var run repos.SyncRun
	err := r.db.QueryRow(ctx, query, userID, repositoryID, syncType).Scan(
		&run.ID,
		&run.RepositoryID,
		&run.SyncType,
		&run.Status,
		&run.ErrorMessage,
		&run.StartedAt,
		&run.CompletedAt,
		&run.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repos.SyncRun{}, ErrRepositoryNotFound
		}
		return repos.SyncRun{}, fmt.Errorf("create sync run: %w", err)
	}

	return run, nil
}

func (r *RepositoryRepository) ListSyncRunsForRepository(ctx context.Context, userID int64, repositoryID int64) ([]repos.SyncRun, error) {
	const query = `
		SELECT
			sr.id,
			sr.repository_id,
			sr.sync_type,
			sr.status,
			sr.error_message,
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
			&run.Status,
			&run.ErrorMessage,
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

func (r *RepositoryRepository) FindSyncRunForRepository(ctx context.Context, userID int64, repositoryID int64, runID int64) (repos.SyncRun, error) {
	const query = `
		SELECT
			sr.id,
			sr.repository_id,
			sr.sync_type,
			sr.status,
			sr.error_message,
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
		&run.Status,
		&run.ErrorMessage,
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
