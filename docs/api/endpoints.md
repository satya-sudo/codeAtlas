# API Surface

## Auth Service (`http://localhost:8061`)

- `GET /auth/github/login`
- `GET /auth/github/callback`
- `GET /auth/me`

## Repo Service (`http://localhost:8062`)

- `GET /integrations/github/install`
- `GET /integrations/github/setup`
- `GET /integrations/github/installations`
- `POST /integrations/github/installations/claim`
- `GET /integrations/github/installations/{installationId}/repositories`
- `POST /integrations/github/installations/{installationId}/repositories/connect`
- `GET /repos`
- `POST /repos`
- `GET /repos/{id}`

For request and response schemas, use [openapi.yaml](./openapi.yaml).
