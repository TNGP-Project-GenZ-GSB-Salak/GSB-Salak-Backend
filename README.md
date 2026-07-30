# GSB Salak Backend

A Go backend (chi + GORM + PostgreSQL) that mimics the "Digital Salak" feature of GSB's MyMo mobile banking app: a user holds multiple bank accounts — a savings account and a premium digital lottery-savings-bond account ("Salak") — and can buy Salak by transferring funds from savings into a Salak account, which mints a lottery holding with a sequential ticket-number range.

Built as a **modular monolith**: one Go module and one Postgres database, but each business domain (`user`, `account`, `salak`, `transaction`) owns its own Postgres schema and its own package tree, communicating in-process through narrow Go interfaces rather than HTTP.

## Requirements

- Go 1.26+
- Docker (for Postgres via `docker-compose.yml`)

## Quickstart

```sh
docker compose up -d                    # start Postgres
go run ./cmd/migrate up                 # create schemas + tables
SEED_DEMO_DATA=true go run ./cmd/seed   # seed salak products + a demo user/accounts
go run ./cmd/api                        # start the API on :8080
```

Or via the Makefile: `make docker-up`, `make migrate-up`, `make seed`, `make run`.

Config is env-var driven with local defaults matching the docker-compose Postgres, so the above works with no `.env` file. Copy `.env.example` to `.env` to override (DB connection, `JWT_SECRET`, `HTTP_PORT`, etc.).

The seed script creates a demo login: `demo` / `demopass123`, with a savings account (50,000 THB) and an empty Salak account.

## API

Base path `/api/v1`. All routes except `/auth/*` require a `Authorization: Bearer <token>` header (obtained from `/auth/login`).

```
POST   /auth/register
POST   /auth/login

GET    /accounts                    # list my accounts
GET    /accounts/:id

GET    /salak/products               # list available Salak products (1yr/2yr terms)
GET    /salak/products/:id

POST   /transactions/buy-salak       # buy salak: debit funding account, mint a holding, credit salak account
GET    /transactions?account_id=...  # transaction/ledger history for an account
```

Example — buy 30,000 THB of the 1-year Salak product:

```sh
curl -X POST localhost:8080/api/v1/transactions/buy-salak \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"funding_account_id":"...","salak_account_id":"...","product_id":"...","amount":30000}'
```

returns a receipt with the unit count, ticket-number range, updated balances, and maturity date.

## Migrations & seeding

Migrations are plain SQL files in `/migrations`, managed with [golang-migrate](https://github.com/golang-migrate/migrate) — `cmd/migrate` embeds them into a single static binary, but the same files also work with the standalone `migrate` CLI for ad hoc ops. Migrating and seeding are deliberately separate binaries (`cmd/migrate`, `cmd/seed`) and neither runs automatically from `cmd/api`.

## Documentation

See [`CLAUDE.md`](./CLAUDE.md) for architecture notes (domain boundaries, the cross-domain transaction pattern, why accounts use a type discriminator, etc.) aimed at anyone — human or AI — making changes to this codebase.
