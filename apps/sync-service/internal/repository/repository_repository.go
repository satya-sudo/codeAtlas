package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	sharedgithub "codeatlas/packages/github"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrRepositoryNotFound = errors.New("repository not found")

type Repository struct {
	ID             int64
	GitHubRepoID   int64
	Owner          string
	Name           string
	InstallationID *int64
}

type RepositoryRepository struct {
	db *pgxpool.Pool
}

func NewRepositoryRepository(db *pgxpool.Pool) *RepositoryRepository {
	return &RepositoryRepository{db: db}
}

func (r *RepositoryRepository) FindRepositoryForSync(ctx context.Context, repositoryID int64) (Repository, error) {
	const query = `
		SELECT id, github_repo_id, owner, name, installation_id
		FROM repositories
		WHERE id = $1
	`

	var repo Repository
	err := r.db.QueryRow(ctx, query, repositoryID).Scan(
		&repo.ID,
		&repo.GitHubRepoID,
		&repo.Owner,
		&repo.Name,
		&repo.InstallationID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Repository{}, ErrRepositoryNotFound
		}
		return Repository{}, fmt.Errorf("find repository for sync: %w", err)
	}

	return repo, nil
}

func (r *RepositoryRepository) MarkSyncRunRunning(ctx context.Context, syncRunID int64, repositoryID int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin mark running transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(
		ctx,
		`UPDATE repository_sync_runs SET status = 'running', started_at = NOW(), error_message = NULL WHERE id = $1 AND repository_id = $2`,
		syncRunID,
		repositoryID,
	); err != nil {
		return fmt.Errorf("mark sync run running: %w", err)
	}

	if _, err := tx.Exec(
		ctx,
		`UPDATE repositories SET sync_status = 'importing', updated_at = NOW() WHERE id = $1`,
		repositoryID,
	); err != nil {
		return fmt.Errorf("mark repository importing: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit mark running transaction: %w", err)
	}

	return nil
}

func (r *RepositoryRepository) ReplaceContributors(ctx context.Context, repositoryID int64, contributors []sharedgithub.RepositoryContributor) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin replace contributors transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM repository_contributors WHERE repository_id = $1`, repositoryID); err != nil {
		return fmt.Errorf("delete old contributors: %w", err)
	}

	const insertQuery = `
		INSERT INTO repository_contributors (
			repository_id,
			github_user_id,
			username,
			avatar_url,
			contributions_count,
			last_seen_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	now := time.Now().UTC()
	for _, contributor := range contributors {
		var avatarURL *string
		if contributor.AvatarURL != "" {
			avatarURL = &contributor.AvatarURL
		}

		if _, err := tx.Exec(
			ctx,
			insertQuery,
			repositoryID,
			contributor.ID,
			contributor.Login,
			avatarURL,
			contributor.Contributions,
			now,
		); err != nil {
			return fmt.Errorf("insert contributor %s: %w", contributor.Login, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit replace contributors transaction: %w", err)
	}

	return nil
}

func (r *RepositoryRepository) MarkSyncRunSucceeded(ctx context.Context, syncRunID int64, repositoryID int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin mark succeeded transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(
		ctx,
		`UPDATE repository_sync_runs SET status = 'succeeded', completed_at = NOW(), error_message = NULL WHERE id = $1 AND repository_id = $2`,
		syncRunID,
		repositoryID,
	); err != nil {
		return fmt.Errorf("mark sync run succeeded: %w", err)
	}

	if _, err := tx.Exec(
		ctx,
		`UPDATE repositories SET sync_status = 'ready', last_synced_at = NOW(), updated_at = NOW() WHERE id = $1`,
		repositoryID,
	); err != nil {
		return fmt.Errorf("mark repository ready: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit mark succeeded transaction: %w", err)
	}

	return nil
}

func (r *RepositoryRepository) MarkSyncRunFailed(ctx context.Context, syncRunID int64, repositoryID int64, message string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin mark failed transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(
		ctx,
		`UPDATE repository_sync_runs SET status = 'failed', completed_at = NOW(), error_message = $3 WHERE id = $1 AND repository_id = $2`,
		syncRunID,
		repositoryID,
		message,
	); err != nil {
		return fmt.Errorf("mark sync run failed: %w", err)
	}

	if _, err := tx.Exec(
		ctx,
		`UPDATE repositories SET sync_status = 'failed', updated_at = NOW() WHERE id = $1`,
		repositoryID,
	); err != nil {
		return fmt.Errorf("mark repository failed: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit mark failed transaction: %w", err)
	}

	return nil
}
