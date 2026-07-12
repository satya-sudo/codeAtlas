package repos

import "time"

type RepositoryCoChange struct {
	LeftPath        string     `json:"left_path"`
	RightPath       string     `json:"right_path"`
	CoChangeCount   int        `json:"cochange_count"`
	LastCochangedAt *time.Time `json:"last_cochanged_at,omitempty"`
}

type ModuleCoChange struct {
	LeftModuleID    *int64     `json:"left_module_id,omitempty"`
	LeftModuleName  string     `json:"left_module_name"`
	LeftPathPrefix  string     `json:"left_path_prefix"`
	RightModuleID   *int64     `json:"right_module_id,omitempty"`
	RightModuleName string     `json:"right_module_name"`
	RightPathPrefix string     `json:"right_path_prefix"`
	CoChangeCount   int        `json:"cochange_count"`
	LastCochangedAt *time.Time `json:"last_cochanged_at,omitempty"`
}
