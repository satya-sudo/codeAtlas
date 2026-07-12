package repos

type RepositorySyncStatus struct {
	Repository    Repository `json:"repository"`
	LatestSyncRun *SyncRun   `json:"latest_sync_run,omitempty"`
}
