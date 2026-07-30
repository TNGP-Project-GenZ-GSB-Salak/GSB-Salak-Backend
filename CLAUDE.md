# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Go backend (Gin + GORM + Postgres) that mimics the "Digital Salak" feature of GSB's MyMo mobile banking app: a user has multiple bank accounts (savings + a premium digital lottery-savings-bond product called "Salak"), and can buy Salak by transferring funds from a savings account into a Salak account, which mints a lottery holding with a sequential ticket-number range.

It is a **modular monolith**: one Go module, one Postgres database, but each business domain owns its own Postgres schema (`"user"`, `account`, `salak`, `transaction`) and its own `internal/<domain>` package tree. Cross-domain calls happen in-process through Go interfaces, never HTTP.

## Commands

```sh
docker compose up -d          # start Postgres (schema/env baked into docker-compose.yml)
go run ./cmd/migrate up       # apply migrations (golang-migrate, embedded via embed.FS)
go run ./cmd/migrate down     # roll back one migration
go run ./cmd/migrate version  # print current schema_migrations version
go run ./cmd/migrate force <n>
SEED_DEMO_DATA=true go run ./cmd/seed   # seed salak products + a demo user/accounts (idempotent, safe to re-run)
go run ./cmd/api              # start the HTTP API on :8080 (see .env.example for config)

go build ./...
go vet ./...
gofmt -l .                    # list files needing formatting; `gofmt -w .` to fix
go test ./...                 # (test suite not yet written — see test/integration/)
```

A `Makefile` wraps all of the above (`make run`, `make migrate-up`, `make seed`, `make build`, `make test`, `make docker-up`/`docker-down`).

Config is env-var driven (`internal/platform/config/config.go`), with sane local defaults — `go run ./cmd/api` works with zero env vars set against the docker-compose Postgres. Copy `.env.example` to `.env` to override.

## Architecture

### Domain boundaries and the `ports.go` rule

Each domain lives at `internal/<domain>/` with this internal shape:
```
internal/<domain>/
├── ports.go            # package <domain> — Repository + Service interfaces (the ONLY public surface)
├── domain/             # package domain — plain structs, GORM TableName()
├── repository/         # package repository — GORM implementations of ports.go's Repository interface
├── service/            # package service — implements ports.go's Service interface
└── http/                # package http — Gin handlers + DTOs, depends only on the Service interface
```

The domains are `user`, `account`, `salak`, `transaction`. **`account` and `salak` must never import `transaction`** — this is the load-bearing rule in this codebase; if you're adding a feature that seems to require `account` or `salak` to know about `transaction`, the orchestration belongs in `transaction` instead. Dependencies otherwise flow one way: `account` is a leaf (imports no sibling domain); `salak` depends on `account.Service` (e.g. `ListHoldingsByAccount` verifies account ownership via `accounts.GetByID` before returning holdings); `transaction` depends on both `account.Service` and `salak.Service`. All of these are `ports.go` interfaces injected at the composition root, never concrete repos/services from another domain's package.

The composition root — `cmd/api/main.go` — is the only place concrete repos/services from different domains are wired together. It builds each domain's repository → service → http.Handler bottom-up, then registers routes.

### Why `Account` is a single table with a `type` discriminator

`account.accounts` has a `type` column (`savings` | `salak`) rather than separate tables per account kind. This is deliberate: MyMo's account-list screen is a flat list of all account types with a balance each, and this design keeps that screen a single `SELECT ... WHERE user_id = ?` with no joins. A Salak-type account's `balance` is the aggregate THB value of its `salak.holdings`, kept in sync on every mint — it is not derived on read.

### Cross-domain atomicity: the `tx *gorm.DB` parameter pattern

The "buy Salak" flow (`internal/transaction/service/buy_salak_service.go`) must atomically: debit a savings account, mint a `salak.holdings` row (reserving a contiguous ticket-number range from the `salak.ticket_sequence` singleton under `SELECT ... FOR UPDATE`), credit the salak account, and write a paired debit/credit `transaction.ledger_entries` row (sharing one `reference_id`).

Because this spans four Postgres schemas but one physical database, all of it happens inside one `db.Transaction(func(tx *gorm.DB) error {...})` call. Every **mutating** method on `account.Service` and `salak.Service` takes an explicit `tx *gorm.DB` parameter instead of using the service's own ambient DB handle — this is what lets `transaction`'s orchestration call across domain boundaries while still participating in one Postgres transaction. Read-only methods (`ListByUser`, `GetProduct`, etc.) use the service's ambient handle since they don't need transactional consistency with the write. If you add a new mutating cross-domain operation, follow this same `tx`-parameter convention rather than having the callee open its own transaction.

Lock ordering in `BuySalak` is fixed: validate ownership/product rules (no locks) → debit funding account → mint holding (locks `ticket_sequence`) → credit salak account → write ledger entries. Keep new orchestration flows consistent with "debit before credit" to avoid introducing lock-order inversions.

### Migrations

`/migrations` is a single flat, globally-numbered directory (not per-domain subfolders), because golang-migrate tracks one `schema_migrations` version for the whole database and cross-schema foreign keys (e.g. `salak.holdings.account_id -> account.accounts.id`) require a deterministic single ordering across domains. `migrations/embed.go` embeds the `*.sql` files via `embed.FS` so `cmd/migrate` ships as a single static binary with no runtime dependency on the `migrate` CLI — the same SQL files are still fully compatible with the real CLI for ad hoc ops.

Migration and seeding are separate binaries (`cmd/migrate`, `cmd/seed`) and neither is invoked from `cmd/api` — the API server never touches schema or seed data on startup.

### Money and ticket numbers

- All money fields use `github.com/shopspring/decimal`, never `float64`.
- Salak product limits (unit price, min/max purchase, step amount) are rows in `salak.products`, not code constants — `ValidatePurchase` reads them from the fetched product.
- Ticket numbers are `bigint` in the database; zero-padded display formatting (e.g. `0007530`) is a presentation concern for callers, not stored.

### Errors

`internal/platform/apperror` defines a small set of typed errors (`Validation`, `NotFound`, `Unauthorized`, `Forbidden`, `Conflict`, `Internal`) mapped to HTTP status codes by `apperror.HTTPStatus`. Service-layer code should return these (not raw `gorm.ErrRecordNotFound` etc.) so `httpserver.Fail(c, err)` in HTTP handlers can render a correct status/message without each handler needing domain-specific error handling.
