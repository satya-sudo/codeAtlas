package repos

type RepositoryDashboard struct {
	Repository       Repository         `json:"repository"`
	Overview         RepositoryOverview `json:"overview"`
	Hotspots         []RepositoryHotspot `json:"hotspots"`
	LatestSyncRun    *SyncRun           `json:"latest_sync_run,omitempty"`
	RecentSyncRuns   []SyncRun          `json:"recent_sync_runs"`
	TopContributors  []Contributor      `json:"top_contributors"`
}

type RepositoryOverview struct {
	TotalCommits         int        `json:"total_commits"`
	TotalContributors    int        `json:"total_contributors"`
	TotalFiles           int        `json:"total_files"`
	TotalModules         int        `json:"total_modules"`
	LastSyncedAt         *time.Time `json:"last_synced_at,omitempty"`
	LatestSyncDurationMS *int64     `json:"latest_sync_duration_ms,omitempty"`
}

type RepositoryHotspot struct {
	Path         string `json:"path"`
	CommitCount  int    `json:"commit_count"`
	LinesAdded   int    `json:"lines_added"`
	LinesDeleted int    `json:"lines_deleted"`
	Churn        int    `json:"churn"`
}
