# `analytics-worker`

## Responsibility

- Consume internal events
- Update analytics read models in PostgreSQL

## Entrypoint

- `cmd/worker/main.go`

## Suggested Internal Modules

- `internal/consumer`
- `internal/metrics`
- `internal/ownership`
- `internal/hotspots`
- `internal/busfactor`
