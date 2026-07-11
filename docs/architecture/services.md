# Services

## External-Facing Services

### `auth-service`

- Handles GitHub OAuth login
- Creates and reads user records
- Issues JWT or session tokens

### `repo-service`

- Stores repository metadata
- Starts repository onboarding
- Exposes repository and analytics read APIs

### `webhook-service`

- Receives GitHub webhook traffic
- Validates webhook signature
- Normalizes payloads into internal events
- Publishes to Kafka
- Currently accepts `ping` and `push` events, and publishes normalized `push` events

## Internal Processing Services

### `sync-service`

- Imports repository metadata
- Backfills commit history
- Backfills contributors and files
- Registers repository webhooks in GitHub
- Publishes import events to Kafka

### `analytics-worker`

- Consumes internal events
- Updates PostgreSQL analytics tables
- Builds hotspots, ownership, expertise, bus factor, and overview metrics

### `graph-worker`

- Consumes internal events
- Updates Neo4j nodes and edges
- Maintains developer, module, file, and co-change graph relationships

## Why This Split

- Request path stays thin
- Analytics are computed asynchronously
- Graph projection is decoupled from relational analytics
- GitHub payload normalization happens once
