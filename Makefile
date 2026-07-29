.PHONY: run migrate-up migrate-down migrate-version seed docker-up docker-down build test

build:
	go build -o bin/api ./cmd/api
	go build -o bin/migrate ./cmd/migrate
	go build -o bin/seed ./cmd/seed

run:
	go run ./cmd/api

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

migrate-version:
	go run ./cmd/migrate version

seed:
	go run ./cmd/seed

docker-up:
	docker compose up -d

docker-down:
	docker compose down

test:
	go test ./...
