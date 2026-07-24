package repos

import "time"

type ModuleOwnershipEntry struct {
	GitHubUserID      *int64  `json:"github_user_id,omitempty"`
	Username          string  `json:"username"`
	OwnershipPercent  float64 `json:"ownership_percent"`
	CommitCount       int     `json:"commit_count"`
	ChangesCount      int     `json:"changes_count"`
	FilesTouchedCount int     `json:"files_touched_count"`
	Rank              int     `json:"rank"`
}

type ModuleOwnershipInsight struct {
	ModuleID           int64                  `json:"module_id"`
	ModuleName         string                 `json:"module_name"`
	PathPrefix         string                 `json:"path_prefix"`
	BusFactor          int                    `json:"bus_factor"`
	ActiveContributors int                    `json:"active_contributors"`
	TopOwnerPercent    float64                `json:"top_owner_percent"`
	Risk               string                 `json:"risk"`
	Owners             []ModuleOwnershipEntry `json:"owners"`
}

type ModuleExpertiseEntry struct {
	GitHubUserID      *int64     `json:"github_user_id,omitempty"`
	Username          string     `json:"username"`
	Score             int        `json:"score"`
	RawScore          int        `json:"raw_score"`
	CommitCount       int        `json:"commit_count"`
	FilesTouchedCount int        `json:"files_touched_count"`
	RecentCommitCount int        `json:"recent_commit_count"`
	LastCommitAt      *time.Time `json:"last_commit_at,omitempty"`
	Rank              int        `json:"rank"`
}

type ModuleExpertiseInsight struct {
	ModuleID   int64                  `json:"module_id"`
	ModuleName string                 `json:"module_name"`
	PathPrefix string                 `json:"path_prefix"`
	Experts    []ModuleExpertiseEntry `json:"experts"`
}

type ModuleBusFactor struct {
	ModuleID           int64   `json:"module_id"`
	ModuleName         string  `json:"module_name"`
	PathPrefix         string  `json:"path_prefix"`
	BusFactor          int     `json:"bus_factor"`
	ActiveContributors int     `json:"active_contributors"`
	TopOwnerPercent    float64 `json:"top_owner_percent"`
	Risk               string  `json:"risk"`
}

type ModuleCoChangePartner struct {
	ModuleID         int64      `json:"module_id"`
	ModuleName       string     `json:"module_name"`
	PathPrefix       string     `json:"path_prefix"`
	CoChangeCount    int        `json:"cochange_count"`
	LastCochangedAt  *time.Time `json:"last_cochanged_at,omitempty"`
}

type ModuleDetail struct {
	ModuleID           int64                  `json:"module_id"`
	ModuleName         string                 `json:"module_name"`
	PathPrefix         string                 `json:"path_prefix"`
	BusFactor          int                    `json:"bus_factor"`
	ActiveContributors int                    `json:"active_contributors"`
	TopOwnerPercent    float64                `json:"top_owner_percent"`
	Risk               string                 `json:"risk"`
	Owners             []ModuleOwnershipEntry `json:"owners"`
	Experts            []ModuleExpertiseEntry `json:"experts"`
	Hotspots           []RepositoryHotspot    `json:"hotspots"`
	CoChangePartners   []ModuleCoChangePartner `json:"cochange_partners"`
}
