package repository

import (
	"context"
	"fmt"

	"codeatlas/apps/repo-service/internal/installations"
	"github.com/jackc/pgx/v5/pgxpool"
)

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
