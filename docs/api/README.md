# CodeAtlas API Docs

## Files

- `openapi.yaml`: current Swagger / OpenAPI source of truth for the auth-service and repo-service routes.
- `endpoints.md`: lightweight quick reference.

## Current onboarding flow

1. `GET /auth/github/login`
2. `GET /auth/github/callback`
3. `GET /integrations/github/install`
4. `GET /integrations/github/setup`
5. `POST /integrations/github/installations/claim`
6. `GET /integrations/github/installations`
7. `GET /integrations/github/installations/{installationId}/repositories`
8. `POST /integrations/github/installations/{installationId}/repositories/connect`

## How to view

Open `openapi.yaml` in Swagger Editor or any OpenAPI-compatible viewer.
