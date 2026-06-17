# Kafka Topics

## Proposed V1 Topics

- `repo.lifecycle`
- `github.events`
- `code.events`

## Example Event Types

- `repository.connected`
- `repository.import.requested`
- `repository.import.completed`
- `github.push.received`
- `github.pull_request.received`
- `commit.ingested`
- `file.changed`
- `contributor.discovered`

## Partitioning

- Partition key: `repository_id`
- Goal: preserve per-repository event ordering

