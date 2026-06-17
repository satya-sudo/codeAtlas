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

## Notes

- `commits` and `commit_files` are the backbone for most analytics.
- `modules` should be derived deterministically from file paths.
- Derived tables should be updated only by workers.

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
