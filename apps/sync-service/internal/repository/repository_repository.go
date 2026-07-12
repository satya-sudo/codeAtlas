package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path"
	"sort"
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

type moduleContributorAggregate struct {
	ModuleID          int64
	ModuleName        string
	PathPrefix        string
	GitHubUserID      *int64
	Username          *string
	CommitCount       int
	FilesTouchedCount int
	ChangesCount      int
	RecentCommitCount int
	LastCommitAt      *time.Time
}

type moduleContributorAnalytics struct {
	GitHubUserID      *int64
	Username          string
	CommitCount       int
	FilesTouchedCount int
	ChangesCount      int
	RecentCommitCount int
	LastCommitAt      *time.Time
	RawScore          int
	Score             int
	OwnershipPercent  float64
}

type moduleAnalytics struct {
	ModuleID           int64
	ModuleName         string
	PathPrefix         string
	Contributors       []moduleContributorAnalytics
	ActiveContributors int
	BusFactor          int
	TopOwnerPercent    float64
	Risk               string
}

func (r *RepositoryRepository) RebuildModuleAnalytics(ctx context.Context, repositoryID int64) error {
	const aggregateQuery = `
		SELECT
			m.id,
			m.name,
			m.path_prefix,
			c.author_github_user_id,
			CASE
				WHEN c.id IS NULL THEN NULL
				ELSE COALESCE(NULLIF(c.author_username, ''), NULLIF(c.author_name, ''), NULLIF(c.author_email, ''), 'unknown')
			END AS username,
			COUNT(DISTINCT c.id) AS commit_count,
			COUNT(DISTINCT COALESCE(cf.file_id::text, cf.path)) AS files_touched_count,
			COALESCE(SUM(GREATEST(cf.changes, 1)), 0) AS changes_count,
			COUNT(DISTINCT CASE WHEN c.committed_at >= $2 THEN c.id END) AS recent_commit_count,
			MAX(c.committed_at) AS last_commit_at
		FROM modules m
		LEFT JOIN commit_files cf
			ON cf.module_id = m.id
		   AND cf.repository_id = m.repository_id
		LEFT JOIN commits c
			ON c.id = cf.commit_id
		WHERE m.repository_id = $1
		GROUP BY
			m.id,
			m.name,
			m.path_prefix,
			c.author_github_user_id,
			CASE
				WHEN c.id IS NULL THEN NULL
				ELSE COALESCE(NULLIF(c.author_username, ''), NULLIF(c.author_name, ''), NULLIF(c.author_email, ''), 'unknown')
			END
		ORDER BY m.path_prefix ASC, m.name ASC
	`

	rows, err := r.db.Query(ctx, aggregateQuery, repositoryID, time.Now().UTC().AddDate(0, 0, -90))
	if err != nil {
		return fmt.Errorf("query module analytics aggregates: %w", err)
	}
	defer rows.Close()

	modulesByID := make(map[int64]*moduleAnalytics)
	moduleOrder := make([]int64, 0)

	for rows.Next() {
		var row moduleContributorAggregate
		if err := rows.Scan(
			&row.ModuleID,
			&row.ModuleName,
			&row.PathPrefix,
			&row.GitHubUserID,
			&row.Username,
			&row.CommitCount,
			&row.FilesTouchedCount,
			&row.ChangesCount,
			&row.RecentCommitCount,
			&row.LastCommitAt,
		); err != nil {
			return fmt.Errorf("scan module analytics aggregate: %w", err)
		}

		module, ok := modulesByID[row.ModuleID]
		if !ok {
			module = &moduleAnalytics{
				ModuleID:   row.ModuleID,
				ModuleName: row.ModuleName,
				PathPrefix: row.PathPrefix,
			}
			modulesByID[row.ModuleID] = module
			moduleOrder = append(moduleOrder, row.ModuleID)
		}

		if row.Username == nil || strings.TrimSpace(*row.Username) == "" {
			continue
		}

		module.Contributors = append(module.Contributors, moduleContributorAnalytics{
			GitHubUserID:      row.GitHubUserID,
			Username:          strings.TrimSpace(*row.Username),
			CommitCount:       row.CommitCount,
			FilesTouchedCount: row.FilesTouchedCount,
			ChangesCount:      row.ChangesCount,
			RecentCommitCount: row.RecentCommitCount,
			LastCommitAt:      row.LastCommitAt,
		})
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate module analytics aggregates: %w", err)
	}

	modules := make([]moduleAnalytics, 0, len(moduleOrder))
	for _, moduleID := range moduleOrder {
		module := modulesByID[moduleID]
		finalizeModuleAnalytics(module)
		modules = append(modules, *module)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin rebuild module analytics transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM module_ownership WHERE module_id IN (SELECT id FROM modules WHERE repository_id = $1)`, repositoryID); err != nil {
		return fmt.Errorf("delete old module ownership: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM module_expertise WHERE module_id IN (SELECT id FROM modules WHERE repository_id = $1)`, repositoryID); err != nil {
		return fmt.Errorf("delete old module expertise: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM module_metrics WHERE module_id IN (SELECT id FROM modules WHERE repository_id = $1)`, repositoryID); err != nil {
		return fmt.Errorf("delete old module metrics: %w", err)
	}

	const insertModuleMetricsQuery = `
		INSERT INTO module_metrics (
			module_id,
			bus_factor,
			active_contributors,
			top_owner_percent,
			risk
		)
		VALUES ($1, $2, $3, $4, $5)
	`
	const insertModuleOwnershipQuery = `
		INSERT INTO module_ownership (
			module_id,
			github_user_id,
			username,
			ownership_percent,
			commit_count,
			changes_count,
			files_touched_count,
			rank
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	const insertModuleExpertiseQuery = `
		INSERT INTO module_expertise (
			module_id,
			github_user_id,
			username,
			score,
			raw_score,
			commit_count,
			files_touched_count,
			recent_commit_count,
			last_commit_at,
			rank
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	for _, module := range modules {
		if _, err := tx.Exec(
			ctx,
			insertModuleMetricsQuery,
			module.ModuleID,
			module.BusFactor,
			module.ActiveContributors,
			module.TopOwnerPercent,
			module.Risk,
		); err != nil {
			return fmt.Errorf("insert module metrics for %s: %w", module.ModuleName, err)
		}

		ownershipContributors := append([]moduleContributorAnalytics(nil), module.Contributors...)
		sort.SliceStable(ownershipContributors, func(i, j int) bool {
			if ownershipContributors[i].OwnershipPercent != ownershipContributors[j].OwnershipPercent {
				return ownershipContributors[i].OwnershipPercent > ownershipContributors[j].OwnershipPercent
			}
			if ownershipContributors[i].ChangesCount != ownershipContributors[j].ChangesCount {
				return ownershipContributors[i].ChangesCount > ownershipContributors[j].ChangesCount
			}
			if ownershipContributors[i].CommitCount != ownershipContributors[j].CommitCount {
				return ownershipContributors[i].CommitCount > ownershipContributors[j].CommitCount
			}
			return ownershipContributors[i].Username < ownershipContributors[j].Username
		})

		for idx, contributor := range ownershipContributors {
			if _, err := tx.Exec(
				ctx,
				insertModuleOwnershipQuery,
				module.ModuleID,
				contributor.GitHubUserID,
				contributor.Username,
				contributor.OwnershipPercent,
				contributor.CommitCount,
				contributor.ChangesCount,
				contributor.FilesTouchedCount,
				idx+1,
			); err != nil {
				return fmt.Errorf("insert module ownership for %s/%s: %w", module.ModuleName, contributor.Username, err)
			}
		}

		expertiseContributors := append([]moduleContributorAnalytics(nil), module.Contributors...)
		sort.SliceStable(expertiseContributors, func(i, j int) bool {
			if expertiseContributors[i].Score != expertiseContributors[j].Score {
				return expertiseContributors[i].Score > expertiseContributors[j].Score
			}
			if expertiseContributors[i].RecentCommitCount != expertiseContributors[j].RecentCommitCount {
				return expertiseContributors[i].RecentCommitCount > expertiseContributors[j].RecentCommitCount
			}
			return compareTimesDesc(expertiseContributors[i].LastCommitAt, expertiseContributors[j].LastCommitAt)
		})

		for idx, contributor := range expertiseContributors {
			if _, err := tx.Exec(
				ctx,
				insertModuleExpertiseQuery,
				module.ModuleID,
				contributor.GitHubUserID,
				contributor.Username,
				contributor.Score,
				contributor.RawScore,
				contributor.CommitCount,
				contributor.FilesTouchedCount,
				contributor.RecentCommitCount,
				contributor.LastCommitAt,
				idx+1,
			); err != nil {
				return fmt.Errorf("insert module expertise for %s/%s: %w", module.ModuleName, contributor.Username, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit rebuild module analytics transaction: %w", err)
	}

	return nil
}

func (r *RepositoryRepository) RebuildFileCoChange(ctx context.Context, repositoryID int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin rebuild file co-change transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM file_cochange WHERE repository_id = $1`, repositoryID); err != nil {
		return fmt.Errorf("delete old file co-change: %w", err)
	}

	const insertQuery = `
		INSERT INTO file_cochange (
			repository_id,
			left_file_id,
			left_path,
			right_file_id,
			right_path,
			cochange_count,
			last_cochanged_at
		)
		SELECT
			cf1.repository_id,
			MAX(cf1.file_id) AS left_file_id,
			cf1.path AS left_path,
			MAX(cf2.file_id) AS right_file_id,
			cf2.path AS right_path,
			COUNT(DISTINCT cf1.commit_id) AS cochange_count,
			MAX(c.committed_at) AS last_cochanged_at
		FROM commit_files cf1
		INNER JOIN commit_files cf2
			ON cf2.repository_id = cf1.repository_id
		   AND cf2.commit_id = cf1.commit_id
		   AND cf1.path < cf2.path
		INNER JOIN commits c
			ON c.id = cf1.commit_id
		WHERE cf1.repository_id = $1
		GROUP BY cf1.repository_id, cf1.path, cf2.path
		HAVING COUNT(DISTINCT cf1.commit_id) > 0
	`

	if _, err := tx.Exec(ctx, insertQuery, repositoryID); err != nil {
		return fmt.Errorf("insert rebuilt file co-change: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit rebuild file co-change transaction: %w", err)
	}

	return nil
}

func (r *RepositoryRepository) RebuildModuleCoChange(ctx context.Context, repositoryID int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin rebuild module co-change transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM module_cochange WHERE repository_id = $1`, repositoryID); err != nil {
		return fmt.Errorf("delete old module co-change: %w", err)
	}

	const insertQuery = `
		INSERT INTO module_cochange (
			repository_id,
			left_module_id,
			left_module_name,
			left_path_prefix,
			right_module_id,
			right_module_name,
			right_path_prefix,
			cochange_count,
			last_cochanged_at
		)
		SELECT
			cf1.repository_id,
			m1.id AS left_module_id,
			m1.name AS left_module_name,
			m1.path_prefix AS left_path_prefix,
			m2.id AS right_module_id,
			m2.name AS right_module_name,
			m2.path_prefix AS right_path_prefix,
			COUNT(DISTINCT cf1.commit_id) AS cochange_count,
			MAX(c.committed_at) AS last_cochanged_at
		FROM commit_files cf1
		INNER JOIN commit_files cf2
			ON cf2.repository_id = cf1.repository_id
		   AND cf2.commit_id = cf1.commit_id
		   AND cf1.module_id IS NOT NULL
		   AND cf2.module_id IS NOT NULL
		   AND cf1.module_id < cf2.module_id
		INNER JOIN modules m1
			ON m1.id = cf1.module_id
		INNER JOIN modules m2
			ON m2.id = cf2.module_id
		INNER JOIN commits c
			ON c.id = cf1.commit_id
		WHERE cf1.repository_id = $1
		GROUP BY
			cf1.repository_id,
			m1.id, m1.name, m1.path_prefix,
			m2.id, m2.name, m2.path_prefix
		HAVING COUNT(DISTINCT cf1.commit_id) > 0
	`

	if _, err := tx.Exec(ctx, insertQuery, repositoryID); err != nil {
		return fmt.Errorf("insert rebuilt module co-change: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit rebuild module co-change transaction: %w", err)
	}

	return nil
}

func finalizeModuleAnalytics(module *moduleAnalytics) {
	module.ActiveContributors = len(module.Contributors)
	if len(module.Contributors) == 0 {
		module.BusFactor = 0
		module.TopOwnerPercent = 0
		module.Risk = "unknown"
		return
	}

	totalChanges := 0
	maxRawScore := 0
	for idx := range module.Contributors {
		module.Contributors[idx].RawScore = module.Contributors[idx].CommitCount*5 +
			module.Contributors[idx].FilesTouchedCount*3 +
			module.Contributors[idx].RecentCommitCount*7
		if module.Contributors[idx].RawScore > maxRawScore {
			maxRawScore = module.Contributors[idx].RawScore
		}
		totalChanges += module.Contributors[idx].ChangesCount
	}

	for idx := range module.Contributors {
		if totalChanges > 0 {
			module.Contributors[idx].OwnershipPercent = roundToTwoDecimalPlaces(float64(module.Contributors[idx].ChangesCount) * 100 / float64(totalChanges))
		}
		if maxRawScore > 0 {
			module.Contributors[idx].Score = int(math.Round(float64(module.Contributors[idx].RawScore) * 100 / float64(maxRawScore)))
		}
	}

	owners := append([]moduleContributorAnalytics(nil), module.Contributors...)
	sort.SliceStable(owners, func(i, j int) bool {
		if owners[i].OwnershipPercent != owners[j].OwnershipPercent {
			return owners[i].OwnershipPercent > owners[j].OwnershipPercent
		}
		if owners[i].ChangesCount != owners[j].ChangesCount {
			return owners[i].ChangesCount > owners[j].ChangesCount
		}
		if owners[i].CommitCount != owners[j].CommitCount {
			return owners[i].CommitCount > owners[j].CommitCount
		}
		return owners[i].Username < owners[j].Username
	})

	cumulativePercent := 0.0
	for idx, contributor := range owners {
		if idx == 0 {
			module.TopOwnerPercent = contributor.OwnershipPercent
		}
		cumulativePercent += contributor.OwnershipPercent
		if module.BusFactor == 0 && cumulativePercent >= 50 {
			module.BusFactor = idx + 1
		}
	}
	if module.BusFactor == 0 {
		module.BusFactor = len(owners)
	}
	module.Risk = busFactorRisk(module.BusFactor, module.ActiveContributors)
}

func busFactorRisk(busFactor int, activeContributors int) string {
	if activeContributors == 0 {
		return "unknown"
	}
	if busFactor <= 1 {
		return "high"
	}
	if busFactor == 2 {
		return "medium"
	}
	return "low"
}

func roundToTwoDecimalPlaces(value float64) float64 {
	return math.Round(value*100) / 100
}

func compareTimesDesc(left *time.Time, right *time.Time) bool {
	if left == nil && right == nil {
		return false
	}
	if left == nil {
		return false
	}
	if right == nil {
		return true
	}
	if left.Equal(*right) {
		return false
	}
	return left.After(*right)
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
