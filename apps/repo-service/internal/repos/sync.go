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
	ID                int64       `json:"id"`
	RepositoryID      int64       `json:"repository_id"`
	SyncType          string      `json:"sync_type"`
	TriggerSource     string      `json:"trigger_source"`
	TriggerDeliveryID *string     `json:"trigger_delivery_id,omitempty"`
	TriggerRef        *string     `json:"trigger_ref,omitempty"`
	BeforeSHA         *string     `json:"before_sha,omitempty"`
	AfterSHA          *string     `json:"after_sha,omitempty"`
	Status            string      `json:"status"`
	ErrorMessage      *string     `json:"error_message,omitempty"`
	Summary           SyncSummary `json:"summary"`
	StartedAt         *time.Time  `json:"started_at,omitempty"`
	CompletedAt       *time.Time  `json:"completed_at,omitempty"`
	CreatedAt         time.Time   `json:"created_at"`
}

type SyncSummary struct {
	ContributorsCount int    `json:"contributors_count"`
	CommitsCount      int    `json:"commits_count"`
	CommitFilesCount  int    `json:"commit_files_count"`
	ModulesCount      int    `json:"modules_count"`
	FilesCount        int    `json:"files_count"`
	DurationMS        *int64 `json:"duration_ms,omitempty"`
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
