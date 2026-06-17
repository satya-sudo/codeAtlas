package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const createSchemaMigrationsTableQuery = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

type Migration struct {
	Version string
	Path    string
	SQL     string
}

func LoadMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations directory: %w", err)
	}

	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if filepath.Ext(entry.Name()) != ".sql" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}

		version := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		migrations = append(migrations, Migration{
			Version: version,
			Path:    path,
			SQL:     string(content),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

func ApplyMigrations(ctx context.Context, db *pgxpool.Pool, migrations []Migration) error {
	if _, err := db.Exec(ctx, createSchemaMigrationsTableQuery); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	appliedVersions, err := loadAppliedVersions(ctx, db)
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		if _, alreadyApplied := appliedVersions[migration.Version]; alreadyApplied {
			continue
		}

		tx, err := db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration transaction for %q: %w", migration.Version, err)
		}

		if _, err := tx.Exec(ctx, migration.SQL); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %q: %w", migration.Version, err)
		}

		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, migration.Version); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %q: %w", migration.Version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %q: %w", migration.Version, err)
		}
	}

	return nil
}

func loadAppliedVersions(ctx context.Context, db *pgxpool.Pool) (map[string]struct{}, error) {
	rows, err := db.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("load applied migrations: %w", err)
	}
	defer rows.Close()

	versions := make(map[string]struct{})
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		versions[version] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}

	return versions, nil
}
