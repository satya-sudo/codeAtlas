# Development Scripts

Put local developer scripts here, such as:

- bootstrap environment
- create Kafka topics
- seed local data
- run all services in dev mode

## Current scripts

- `init-kafka-topics.sh`
  Creates the local Kafka topics needed by CodeAtlas.

- `run-with-log.sh`
  Runs a local command and mirrors stdout/stderr into `logs/<service>-<timestamp>.log`.

## Examples

```bash
./scripts/dev/run-with-log.sh repo-service go run ./apps/repo-service/cmd/server
./scripts/dev/run-with-log.sh sync-service go run ./apps/sync-service/cmd/worker/main.go
./scripts/dev/run-with-log.sh frontend npm start --prefix ./apps/frontend
```
