# `webhook-service`

## Responsibility

- Receive GitHub webhook events
- Validate signatures
- Normalize event payloads
- Publish internal events to Kafka

## Entrypoint

- `cmd/server/main.go`

## Suggested Internal Modules

- `internal/http`
- `internal/verification`
- `internal/normalizer`
- `internal/publisher`
