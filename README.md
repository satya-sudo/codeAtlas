# CodeAtlas

CodeAtlas is a backend-first, event-driven GitHub intelligence platform.

## V1 Focus

- GitHub OAuth and repository onboarding
- Commit and contributor backfill
- GitHub webhook ingestion
- Kafka-based event distribution
- PostgreSQL analytics read models
- Neo4j knowledge graph projection

## Repository Layout

```text
apps/
  auth-service/
  repo-service/
  sync-service/
  webhook-service/
  analytics-worker/
  graph-worker/
  frontend/

packages/
  config/
  logger/
  database/
  kafka/
  github/
  events/
  models/
  utils/

infra/
  docker/
  kafka/
  postgres/
  neo4j/

docs/
  architecture/
  api/
  events/
  schemas/

scripts/
  dev/
  setup/
```

## Service Responsibilities

- `auth-service`: GitHub OAuth, session/JWT, user profile APIs
- `repo-service`: repository connect flow, repo metadata, read APIs for frontend
- `sync-service`: initial backfill for commits, files, contributors, webhook registration
- `webhook-service`: webhook signature validation, normalization, Kafka publish
- `analytics-worker`: PostgreSQL analytics projections and derived metrics
- `graph-worker`: Neo4j graph projections and relationship updates
- `frontend`: plain JavaScript analytics shell and auth-aware UI

## Shared Packages

- `config`: environment and runtime configuration helpers
- `logger`: structured logging helpers
- `database`: PostgreSQL connection and migration helpers
- `kafka`: producer/consumer abstractions
- `github`: GitHub client and webhook helpers
- `events`: internal event contracts and serializers
- `models`: shared domain types
- `utils`: generic cross-service utilities

## Next Steps

1. Choose the backend language and framework for all services.
2. Create the root workspace manifest.
3. Define PostgreSQL schemas and migration strategy.
4. Define Kafka topics and internal event payloads.
5. Implement `auth-service` and `repo-service` first.
