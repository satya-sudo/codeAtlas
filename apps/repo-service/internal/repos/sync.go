package repos

import "time"

const (
	SyncTypeInitial = "initial"

	SyncRunStatusQueued    = "queued"
	SyncRunStatusRunning   = "running"
	SyncRunStatusSucceeded = "succeeded"
	SyncRunStatusFailed    = "failed"
)

type SyncRequestStatus string

const (
	SyncRequestStatusQueued         SyncRequestStatus = "queued"
	SyncRequestStatusAlreadyQueued  SyncRequestStatus = "already_queued"
	SyncRequestStatusAlreadyRunning SyncRequestStatus = "already_running"
)

type SyncRun struct {
	ID           int64      `json:"id"`
	RepositoryID int64      `json:"repository_id"`
	SyncType     string     `json:"sync_type"`
	Status       string     `json:"status"`
	ErrorMessage *string    `json:"error_message,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type Contributor struct {
	ID                 int64      `json:"id"`
	RepositoryID       int64      `json:"repository_id"`
	GitHubUserID       int64      `json:"github_user_id"`
	Username           string     `json:"username"`
	AvatarURL          *string    `json:"avatar_url,omitempty"`
	ContributionsCount int        `json:"contributions_count"`
	LastSeenAt         *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type SyncRunRequestResult struct {
	SyncRun       SyncRun           `json:"sync_run"`
	RequestStatus SyncRequestStatus `json:"request_status"`
}
