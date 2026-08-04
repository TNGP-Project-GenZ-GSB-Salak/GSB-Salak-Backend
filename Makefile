.PHONY: run worker dev migrate-up migrate-down migrate-version seed docker-up docker-down build test test-integration swagger frontend-install frontend-run frontend-test frontend-report

build:
	go build -o bin/api ./cmd/api
	go build -o bin/migrate ./cmd/migrate
	go build -o bin/seed ./cmd/seed
	go build -o bin/worker ./cmd/worker

run:
	go run ./cmd/api

worker:
	go run ./cmd/worker

# Runs the API and the Kapook auto-purchase worker together - the worker
# fires nothing on its own; omitting it here is the exact silent failure
# the worker package's docs warn about (a countdown that never expires,
# with no error anywhere).
dev:
	@trap 'kill 0' EXIT; \
	$(MAKE) run & \
	$(MAKE) worker & \
	wait

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

test-integration: docker-up migrate-up
	go test -tags=integration ./test/integration/...

frontend-install:
	cd testfrontend && npm install && npx playwright install chromium

frontend-run:
	cd testfrontend && node server.js

frontend-test:
	cd testfrontend && npm test

frontend-report:
	cd testfrontend && npm run report
