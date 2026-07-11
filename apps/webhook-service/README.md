# `webhook-service`

## Responsibility

- Receive GitHub webhook events
- Validate signatures
- Normalize event payloads
- Publish internal events to Kafka

## Current behavior

- Exposes `POST /webhooks/github`
- Accepts GitHub `ping` and `push`
- Verifies `X-Hub-Signature-256` using `GITHUB_WEBHOOK_SECRET`
- Publishes normalized `push` payloads to Kafka topic `github.push`
- Returns `202 Accepted` for unsupported event types without failing the delivery

## Entrypoint

- `cmd/server/main.go`

## Suggested Internal Modules

- `internal/http`
- `internal/verification`
- `internal/normalizer`
- `internal/publisher`
