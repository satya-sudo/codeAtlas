# PostgreSQL Read Model

## Core Tables

- `users`
- `repositories`
- `user_repositories`
- `repository_sync_runs`
- `modules`
- `files`
- `commits`
- `commit_files`

## Derived Analytics Tables

- `file_metrics`
- `module_metrics`
- `developer_metrics`
- `module_ownership`
- `module_expertise`
- `file_cochange`
- `module_cochange`

## Notes

- `commits` and `commit_files` are the backbone for most analytics.
- `modules` should be derived deterministically from file paths.
- Derived tables should be updated only by workers.
- V1 module analytics are rebuilt after each successful sync inside `sync-service`.
- V1 formulas are intentionally simple and explainable:
  - ownership = contributor module churn / total module churn
  - expertise raw score = `commit_count * 5 + files_touched_count * 3 + recent_commit_count * 7`
  - expertise score = normalized `0-100` inside each module
  - bus factor = minimum contributors needed to reach `50%` cumulative ownership
  - risk = `high` for bus factor `1`, `medium` for `2`, `low` for `3+`

## V1 Module Analytics Tables

### `module_metrics`

Purpose:

- stores per-module bus factor and concentration summary

Suggested columns:

- `module_id`
- `bus_factor`
- `active_contributors`
- `top_owner_percent`
- `risk`
- `created_at`
- `updated_at`

### `module_ownership`

Purpose:

- stores ranked ownership entries per module

Suggested columns:

- `module_id`
- `github_user_id`
- `username`
- `ownership_percent`
- `commit_count`
- `changes_count`
- `files_touched_count`
- `rank`
- `created_at`
- `updated_at`

### `module_expertise`

Purpose:

- stores ranked expertise entries per module

Suggested columns:

- `module_id`
- `github_user_id`
- `username`
- `score`
- `raw_score`
- `commit_count`
- `files_touched_count`
- `recent_commit_count`
- `last_commit_at`
- `rank`
- `created_at`
- `updated_at`

### `file_cochange`

Purpose:

- stores ranked file pairs that frequently change together inside one repository
- powers hidden dependency and coupling insights without recomputing pairs on every API call

Suggested columns:

- `repository_id`
- `left_file_id`
- `left_path`
- `right_file_id`
- `right_path`
- `cochange_count`
- `last_cochanged_at`
- `created_at`
- `updated_at`

### `module_cochange`

Purpose:

- stores ranked module pairs that frequently change together inside one repository
- powers higher-level coupling insights without recomputing module pairs on every API call

Suggested columns:

- `repository_id`
- `left_module_id`
- `left_module_name`
- `left_path_prefix`
- `right_module_id`
- `right_module_name`
- `right_path_prefix`
- `cochange_count`
- `last_cochanged_at`
- `created_at`
- `updated_at`

## Repository Service Schema

### `repositories`

Purpose:

- stores the canonical GitHub repository record inside CodeAtlas
- tracks whether initial import has started or completed
- stores later GitHub App installation and webhook identifiers

Suggested columns:

- `id`
- `github_repo_id`
- `owner`
- `name`
- `full_name`
- `default_branch`
- `is_private`
- `installation_id`
- `webhook_id`
- `sync_status`
- `last_synced_at`
- `created_at`
- `updated_at`

Recommended `sync_status` values:

- `pending`
- `importing`
- `ready`
- `failed`

### `user_repositories`

Purpose:

- maps which CodeAtlas users connected or can view which repositories
- gives us a simple access-control anchor without hard-coding ownership into `repositories`

Suggested columns:

- `user_id`
- `repository_id`
- `role`
- `connected_at`

Recommended `role` values:

- `owner`
- `viewer`

### `repository_sync_runs`

Purpose:

- tracks onboarding and later re-sync attempts
- useful for status pages, retries, and debugging

Suggested columns:

- `id`
- `repository_id`
- `sync_type`
- `status`
- `error_message`
- `started_at`
- `completed_at`
- `created_at`

Recommended `sync_type` values:

- `initial`
- `incremental`
- `manual`

Recommended `status` values:

- `queued`
- `running`
- `succeeded`
- `failed`
