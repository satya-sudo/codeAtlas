package events

import "time"

const RepositorySyncRequestedTopic = "repository.sync.requested"

type RepositorySyncRequested struct {
	SyncRunID         int64     `json:"sync_run_id"`
	RepositoryID      int64     `json:"repository_id"`
	SyncType          string    `json:"sync_type"`
	RequestedByUserID int64     `json:"requested_by_user_id"`
	RequestedAt       time.Time `json:"requested_at"`
}
