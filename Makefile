.PHONY: help up down logs migrate repo-service sync-service frontend

help:
	@echo "Available targets: up down logs migrate repo-service sync-service frontend"

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

migrate:
	go run ./cmd/migrate

repo-service:
	./scripts/dev/run-with-log.sh repo-service go run ./apps/repo-service/cmd/server

sync-service:
	./scripts/dev/run-with-log.sh sync-service go run ./apps/sync-service/cmd/worker/main.go

frontend:
	./scripts/dev/run-with-log.sh frontend npm start --prefix ./apps/frontend
