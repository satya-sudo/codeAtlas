package repository

import (
	"context"
	"fmt"

	"codeatlas/apps/auth-service/internal/users"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) UpsertGitHubUser(ctx context.Context, githubID int64, username string, avatarURL string) (users.User, error) {
	const query = `
		INSERT INTO users (github_id, username, avatar_url)
		VALUES ($1, $2, $3)
		ON CONFLICT (github_id)
		DO UPDATE SET
			username = EXCLUDED.username,
			avatar_url = EXCLUDED.avatar_url,
			updated_at = NOW()
		RETURNING id, github_id, username, avatar_url, created_at, updated_at
	`

	var user users.User
	err := r.db.QueryRow(ctx, query, githubID, username, avatarURL).Scan(
		&user.ID,
		&user.GitHubID,
		&user.Username,
		&user.AvatarURL,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return users.User{}, fmt.Errorf("upsert user: %w", err)
	}

	return user, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id int64) (users.User, error) {
	const query = `
		SELECT id, github_id, username, avatar_url, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user users.User
	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.GitHubID,
		&user.Username,
		&user.AvatarURL,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return users.User{}, fmt.Errorf("find user by id: %w", err)
	}

	return user, nil
}
