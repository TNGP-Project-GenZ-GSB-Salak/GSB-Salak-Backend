.PHONY: run migrate-up migrate-down migrate-version seed docker-up docker-down build test swagger frontend-install frontend-run frontend-test frontend-report

build:
	go build -o bin/api ./cmd/api
	go build -o bin/migrate ./cmd/migrate
	go build -o bin/seed ./cmd/seed

run:
	go run ./cmd/api

swagger:
	swag init -g cmd/api/main.go -o docs --parseInternal

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

frontend-install:
	cd testfrontend && npm install && npx playwright install chromium

frontend-run:
	cd testfrontend && node server.js

frontend-test:
	cd testfrontend && npm test

frontend-report:
	cd testfrontend && npm run report
