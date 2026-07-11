package repository

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	sharedgithub "codeatlas/packages/github"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrRepositoryNotFound = errors.New("repository not found")
	ErrSyncRunNotFound    = errors.New("sync run not found")
)

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

type CommitImportStats struct {
	CommitsCount     int
	CommitFilesCount int
	ModulesCount     int
	FilesCount       int
}

type SyncRunSummary struct {
	ContributorsCount int
	CommitsCount      int
	CommitFilesCount  int
	ModulesCount      int
	FilesCount        int
	DurationMS        int64
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

func (r *RepositoryRepository) MarkSyncRunRunning(ctx context.Context, syncRunID int64, repositoryID int64, expectedCreatedAt time.Time) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin mark running transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(
		ctx,
		`UPDATE repository_sync_runs
		 SET status = 'running',
		     started_at = NOW(),
		     error_message = NULL
		 WHERE id = $1
		   AND repository_id = $2
		   AND created_at = $3
		   AND status = 'queued'`,
		syncRunID,
		repositoryID,
		expectedCreatedAt,
	)
	if err != nil {
		return fmt.Errorf("mark sync run running: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSyncRunNotFound
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

func (r *RepositoryRepository) ReplaceCommitData(ctx context.Context, repositoryID int64, commits []sharedgithub.RepositoryCommit) (CommitImportStats, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return CommitImportStats{}, fmt.Errorf("begin replace commit data transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM commits WHERE repository_id = $1`, repositoryID); err != nil {
		return CommitImportStats{}, fmt.Errorf("delete old commits: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM files WHERE repository_id = $1`, repositoryID); err != nil {
		return CommitImportStats{}, fmt.Errorf("delete old files: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM modules WHERE repository_id = $1`, repositoryID); err != nil {
		return CommitImportStats{}, fmt.Errorf("delete old modules: %w", err)
	}

	reverseCommitsInPlace(commits)

	moduleIDs := make(map[string]int64)
	fileIDs := make(map[string]int64)
	stats := CommitImportStats{}

	const insertModuleQuery = `
		INSERT INTO modules (repository_id, name, path_prefix)
		VALUES ($1, $2, $3)
		RETURNING id
	`
	const insertFileQuery = `
		INSERT INTO files (
			repository_id,
			module_id,
			path,
			extension,
			is_deleted,
			first_seen_commit_sha,
			last_seen_commit_sha
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	const updateFileQuery = `
		UPDATE files
		SET module_id = $2,
			extension = $3,
			is_deleted = $4,
			last_seen_commit_sha = $5,
			updated_at = NOW()
		WHERE id = $1
	`
	const markFileDeletedQuery = `
		UPDATE files
		SET is_deleted = TRUE,
			last_seen_commit_sha = $2,
			updated_at = NOW()
		WHERE id = $1
	`
	const insertCommitQuery = `
		INSERT INTO commits (
			repository_id,
			github_commit_sha,
			author_github_user_id,
			author_username,
			author_name,
			author_email,
			committed_at,
			message,
			parent_count,
			additions,
			deletions,
			total_changes
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id
	`
	const insertCommitFileQuery = `
		INSERT INTO commit_files (
			commit_id,
			repository_id,
			file_id,
			module_id,
			path,
			previous_path,
			change_type,
			additions,
			deletions,
			changes,
			patch_text
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	for _, commit := range commits {
		var authorGitHubUserID any
		if commit.AuthorGitHubUserID != nil {
			authorGitHubUserID = *commit.AuthorGitHubUserID
		}

		var commitID int64
		err := tx.QueryRow(
			ctx,
			insertCommitQuery,
			repositoryID,
			commit.SHA,
			authorGitHubUserID,
			nullIfEmpty(commit.AuthorUsername),
			nullIfEmpty(commit.AuthorName),
			nullIfEmpty(commit.AuthorEmail),
			commit.CommittedAt,
			nullIfEmpty(commit.Message),
			commit.ParentCount,
			commit.Additions,
			commit.Deletions,
			commit.TotalChanges,
		).Scan(&commitID)
		if err != nil {
			return CommitImportStats{}, fmt.Errorf("insert commit %s: %w", commit.SHA, err)
		}
		stats.CommitsCount++

		for _, changedFile := range commit.Files {
			moduleName, modulePrefix := deriveModule(changedFile.Path)
			moduleKey := moduleName + "|" + modulePrefix

			moduleID, ok := moduleIDs[moduleKey]
			if !ok {
				if err := tx.QueryRow(ctx, insertModuleQuery, repositoryID, moduleName, modulePrefix).Scan(&moduleID); err != nil {
					return CommitImportStats{}, fmt.Errorf("insert module %s: %w", moduleName, err)
				}
				moduleIDs[moduleKey] = moduleID
				stats.ModulesCount++
			}

			fileID, ok := fileIDs[changedFile.Path]
			extension := normalizeExtension(changedFile.Path)
			isDeleted := changedFile.ChangeType == "deleted"
			if !ok {
				if err := tx.QueryRow(
					ctx,
					insertFileQuery,
					repositoryID,
					moduleID,
					changedFile.Path,
					nullIfEmpty(extension),
					isDeleted,
					commit.SHA,
					commit.SHA,
				).Scan(&fileID); err != nil {
					return CommitImportStats{}, fmt.Errorf("insert file %s: %w", changedFile.Path, err)
				}
				fileIDs[changedFile.Path] = fileID
				stats.FilesCount++
			} else {
				if _, err := tx.Exec(
					ctx,
					updateFileQuery,
					fileID,
					moduleID,
					nullIfEmpty(extension),
					isDeleted,
					commit.SHA,
				); err != nil {
					return CommitImportStats{}, fmt.Errorf("update file %s: %w", changedFile.Path, err)
				}
			}

			if changedFile.PreviousPath != nil {
				if previousFileID, found := fileIDs[*changedFile.PreviousPath]; found {
					if _, err := tx.Exec(ctx, markFileDeletedQuery, previousFileID, commit.SHA); err != nil {
						return CommitImportStats{}, fmt.Errorf("mark previous file deleted %s: %w", *changedFile.PreviousPath, err)
					}
				}
			}

			if _, err := tx.Exec(
				ctx,
				insertCommitFileQuery,
				commitID,
				repositoryID,
				fileID,
				moduleID,
				changedFile.Path,
				changedFile.PreviousPath,
				changedFile.ChangeType,
				changedFile.Additions,
				changedFile.Deletions,
				changedFile.Changes,
				changedFile.PatchText,
			); err != nil {
				return CommitImportStats{}, fmt.Errorf("insert commit file %s for commit %s: %w", changedFile.Path, commit.SHA, err)
			}
			stats.CommitFilesCount++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return CommitImportStats{}, fmt.Errorf("commit replace commit data transaction: %w", err)
	}

	return stats, nil
}

func (r *RepositoryRepository) MarkSyncRunSucceeded(ctx context.Context, syncRunID int64, repositoryID int64, expectedCreatedAt time.Time, summary SyncRunSummary) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin mark succeeded transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(
		ctx,
		`UPDATE repository_sync_runs
		 SET status = 'succeeded',
		     completed_at = NOW(),
		     error_message = NULL,
		     contributors_count = $3,
		     commits_count = $4,
		     commit_files_count = $5,
		     modules_count = $6,
		     files_count = $7,
		     duration_ms = $8
		 WHERE id = $1
		   AND repository_id = $2
		   AND created_at = $9
		   AND status = 'running'`,
		syncRunID,
		repositoryID,
		summary.ContributorsCount,
		summary.CommitsCount,
		summary.CommitFilesCount,
		summary.ModulesCount,
		summary.FilesCount,
		summary.DurationMS,
		expectedCreatedAt,
	)
	if err != nil {
		return fmt.Errorf("mark sync run succeeded: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSyncRunNotFound
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

func (r *RepositoryRepository) MarkSyncRunFailed(ctx context.Context, syncRunID int64, repositoryID int64, expectedCreatedAt time.Time, message string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin mark failed transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(
		ctx,
		`UPDATE repository_sync_runs
		 SET status = 'failed',
		     completed_at = NOW(),
		     error_message = $3,
		     duration_ms = CASE
		         WHEN started_at IS NULL THEN duration_ms
		         ELSE GREATEST((EXTRACT(EPOCH FROM (NOW() - started_at)) * 1000)::BIGINT, 0)
		     END
		 WHERE id = $1
		   AND repository_id = $2
		   AND created_at = $4
		   AND status IN ('queued', 'running')`,
		syncRunID,
		repositoryID,
		message,
		expectedCreatedAt,
	)
	if err != nil {
		return fmt.Errorf("mark sync run failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSyncRunNotFound
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

func reverseCommitsInPlace(commits []sharedgithub.RepositoryCommit) {
	for left, right := 0, len(commits)-1; left < right; left, right = left+1, right-1 {
		commits[left], commits[right] = commits[right], commits[left]
	}
}

func deriveModule(filePath string) (string, string) {
	normalized := strings.Trim(strings.TrimSpace(filePath), "/")
	if normalized == "" {
		return "root", ""
	}

	parts := strings.Split(normalized, "/")
	if len(parts) == 1 {
		return "root", ""
	}

	return parts[0], parts[0] + "/"
}

func normalizeExtension(filePath string) string {
	ext := strings.TrimPrefix(path.Ext(filePath), ".")
	return strings.TrimSpace(ext)
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
