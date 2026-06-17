# `sync-service`

## Responsibility

- Repository import
- Commit history backfill
- Contributor backfill
- Webhook registration

## Entrypoint

- `cmd/worker/main.go`

## Suggested Internal Modules

- `internal/github`
- `internal/importer`
- `internal/publisher`
- `internal/webhooks`
