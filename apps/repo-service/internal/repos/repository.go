package repos

import "time"

type Repository struct {
	ID             int64      `json:"id"`
	GitHubRepoID   int64      `json:"github_repo_id"`
	Owner          string     `json:"owner"`
	Name           string     `json:"name"`
	FullName       string     `json:"full_name"`
	DefaultBranch  string     `json:"default_branch"`
	IsPrivate      bool       `json:"is_private"`
	InstallationID *int64     `json:"installation_id,omitempty"`
	WebhookID      *int64     `json:"webhook_id,omitempty"`
	SyncStatus     string     `json:"sync_status"`
	LastSyncedAt   *time.Time `json:"last_synced_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type ConnectRepositoryInput struct {
	UserID        int64  `json:"-"`
	GitHubRepoID  int64  `json:"github_repo_id"`
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	DefaultBranch string `json:"default_branch"`
	IsPrivate     bool   `json:"is_private"`
}
