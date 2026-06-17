package repository

import (
	"context"
	"fmt"

	"codeatlas/apps/repo-service/internal/repos"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RepositoryRepository struct {
	db *pgxpool.Pool
}

func NewRepositoryRepository(db *pgxpool.Pool) *RepositoryRepository {
	return &RepositoryRepository{db: db}
}

func (r *RepositoryRepository) ConnectRepository(ctx context.Context, input repos.ConnectRepositoryInput) (repos.Repository, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return repos.Repository{}, fmt.Errorf("begin connect repository transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	fullName := input.Owner + "/" + input.Name

	const upsertRepositoryQuery = `
		INSERT INTO repositories (
			github_repo_id,
			owner,
			name,
			full_name,
			default_branch,
			is_private
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (github_repo_id)
		DO UPDATE SET
			owner = EXCLUDED.owner,
			name = EXCLUDED.name,
			full_name = EXCLUDED.full_name,
			default_branch = EXCLUDED.default_branch,
			is_private = EXCLUDED.is_private,
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
		return repos.Repository{}, fmt.Errorf("upsert repository: %w", err)
	}

	const upsertUserRepositoryQuery = `
		INSERT INTO user_repositories (user_id, repository_id, role)
		VALUES ($1, $2, 'owner')
		ON CONFLICT (user_id, repository_id)
		DO UPDATE SET role = EXCLUDED.role
	`

	if _, err := tx.Exec(ctx, upsertUserRepositoryQuery, input.UserID, repo.ID); err != nil {
		return repos.Repository{}, fmt.Errorf("link user to repository: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return repos.Repository{}, fmt.Errorf("commit connect repository transaction: %w", err)
	}

	return repo, nil
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
		return repos.Repository{}, fmt.Errorf("find repository for user: %w", err)
	}

	return repo, nil
}
