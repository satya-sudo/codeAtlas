package repository

import (
	"context"
	"errors"
	"fmt"

	"codeatlas/apps/repo-service/internal/installations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInstallationNotFound = errors.New("installation not found")

type InstallationRepository struct {
	db *pgxpool.Pool
}

func NewInstallationRepository(db *pgxpool.Pool) *InstallationRepository {
	return &InstallationRepository{db: db}
}

func (r *InstallationRepository) UpsertFromSetupCallback(ctx context.Context, installationID int64, setupAction *string) (installations.Installation, error) {
	const query = `
		INSERT INTO github_app_installations (installation_id, setup_action)
		VALUES ($1, $2)
		ON CONFLICT (installation_id)
		DO UPDATE SET
			setup_action = EXCLUDED.setup_action,
			updated_at = NOW()
		RETURNING
			id,
			installation_id,
			installed_by_user_id,
			account_login,
			account_type,
			setup_action,
			status,
			created_at,
			updated_at
	`

	var installation installations.Installation
	err := r.db.QueryRow(ctx, query, installationID, setupAction).Scan(
		&installation.ID,
		&installation.InstallationID,
		&installation.InstalledByUserID,
		&installation.AccountLogin,
		&installation.AccountType,
		&installation.SetupAction,
		&installation.Status,
		&installation.CreatedAt,
		&installation.UpdatedAt,
	)
	if err != nil {
		return installations.Installation{}, fmt.Errorf("upsert installation from setup callback: %w", err)
	}

	return installation, nil
}

func (r *InstallationRepository) ClaimInstallation(ctx context.Context, installationID int64, userID int64) (installations.Installation, error) {
	const query = `
		UPDATE github_app_installations
		SET installed_by_user_id = $2,
			status = 'active',
			updated_at = NOW()
		WHERE installation_id = $1
		RETURNING
			id,
			installation_id,
			installed_by_user_id,
			account_login,
			account_type,
			setup_action,
			status,
			created_at,
			updated_at
	`

	var installation installations.Installation
	err := r.db.QueryRow(ctx, query, installationID, userID).Scan(
		&installation.ID,
		&installation.InstallationID,
		&installation.InstalledByUserID,
		&installation.AccountLogin,
		&installation.AccountType,
		&installation.SetupAction,
		&installation.Status,
		&installation.CreatedAt,
		&installation.UpdatedAt,
	)
	if err != nil {
		return installations.Installation{}, fmt.Errorf("claim installation: %w", err)
	}

	return installation, nil
}

func (r *InstallationRepository) FindClaimedInstallationForUser(ctx context.Context, installationID int64, userID int64) (installations.Installation, error) {
	const query = `
		SELECT
			id,
			installation_id,
			installed_by_user_id,
			account_login,
			account_type,
			setup_action,
			status,
			created_at,
			updated_at
		FROM github_app_installations
		WHERE installation_id = $1
		  AND installed_by_user_id = $2
	`

	var installation installations.Installation
	err := r.db.QueryRow(ctx, query, installationID, userID).Scan(
		&installation.ID,
		&installation.InstallationID,
		&installation.InstalledByUserID,
		&installation.AccountLogin,
		&installation.AccountType,
		&installation.SetupAction,
		&installation.Status,
		&installation.CreatedAt,
		&installation.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return installations.Installation{}, ErrInstallationNotFound
		}
		return installations.Installation{}, fmt.Errorf("find claimed installation for user: %w", err)
	}

	return installation, nil
}

func (r *InstallationRepository) ListInstallationsForUser(ctx context.Context, userID int64) ([]installations.Installation, error) {
	const query = `
		SELECT
			id,
			installation_id,
			installed_by_user_id,
			account_login,
			account_type,
			setup_action,
			status,
			created_at,
			updated_at
		FROM github_app_installations
		WHERE installed_by_user_id = $1
		ORDER BY updated_at DESC, created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list installations for user: %w", err)
	}
	defer rows.Close()

	var items []installations.Installation
	for rows.Next() {
		var installation installations.Installation
		if err := rows.Scan(
			&installation.ID,
			&installation.InstallationID,
			&installation.InstalledByUserID,
			&installation.AccountLogin,
			&installation.AccountType,
			&installation.SetupAction,
			&installation.Status,
			&installation.CreatedAt,
			&installation.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan installation row: %w", err)
		}

		items = append(items, installation)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate installations: %w", err)
	}

	return items, nil
}
