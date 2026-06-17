.PHONY: help up down logs migrate

help:
	@echo "Available targets: up down logs migrate"

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

migrate:
	go run ./cmd/migrate
