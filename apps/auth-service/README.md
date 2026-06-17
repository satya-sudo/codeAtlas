# `auth-service`

## Responsibility

- GitHub OAuth
- User creation and lookup
- Session or JWT issuance

## Entrypoint

- `cmd/server/main.go`

## V1 Endpoints

- `GET /auth/github/login`
- `GET /auth/github/callback`
- `GET /auth/me`

## Suggested Internal Modules

- `internal/http`
- `internal/oauth`
- `internal/users`
- `internal/tokens`
