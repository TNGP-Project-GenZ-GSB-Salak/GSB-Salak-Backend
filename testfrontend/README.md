# testfrontend

A barebones vanilla HTML/CSS/JS frontend for the GSB Salak API, built to exercise the full user flow (register, login, view accounts, view Salak products, buy Salak, view transaction history) and to drive a Playwright test suite that screenshots each step and looks for bugs. CSS is layout-only — no colors, fonts, or other decoration.

**Note:** this predates the real `GSB-Salak-Frontend` (a Vite + React SPA, which now has its own equivalent Playwright suite in `GSB-Salak-Frontend/tests/`). This still works and is still useful for backend-only smoke testing without a Node/Vite toolchain, but don't add new user-flow test cases here — add them against the real frontend instead.

It's served by its own static file server on a different origin/port than the Go API, so the API has CORS middleware enabled (`internal/platform/middleware/cors.go`) to allow it.

## Setup

```sh
npm install
npx playwright install chromium
```

The API's Postgres must already be running, migrated, and seeded once (from the repo root):

```sh
docker compose up -d
go run ./cmd/migrate up
SEED_DEMO_DATA=true go run ./cmd/seed
```

## Running the frontend manually

```sh
node server.js        # serves testfrontend/ on http://localhost:5173
```

In another terminal, from the repo root: `go run ./cmd/api` (serves the API on `:8080`). Then open `http://localhost:5173` and log in with `demo` / `demopass123`.

## Running the Playwright suite

```sh
npm test               # headless
npm run test:headed    # watch it click through in a real browser window
npm run report          # open the last HTML report
```

`playwright.config.js`'s `webServer` array starts both the static frontend server and `go run ./cmd/api` automatically (reusing them if already running), so you don't need to start either by hand before `npm test` — just make sure Postgres is up/migrated/seeded first.

A `globalSetup.js` resets the seeded demo user's account balances, ticket sequence, and transaction/holdings history to a known baseline via `docker exec ... psql` before the suite runs, so tests are repeatable across runs regardless of how much the demo accounts were drawn down previously. It assumes the Postgres container is named `gsb-salak-backend-db-1` (override with the `DB_CONTAINER` env var if different).

## Screenshots

Every test captures numbered screenshots at each meaningful step into `screenshots/<flow>/<case>/`, e.g. `screenshots/buy-salak/success/02-receipt-shown.png`. This directory is gitignored (regenerated on each run) — see `tests/helpers/screenshot.js`.

## Test data assumptions

Tests use the fixed demo user/account IDs seeded by `cmd/seed` (see `tests/helpers/fixtures.js`). Monetary assertions read the currently-displayed balance from the UI and assert on deltas rather than hardcoded absolutes, so the suite tolerates running multiple times without a full DB reset (`globalSetup` still resets it for a clean baseline, but individual tests don't depend on that exact baseline holding).
