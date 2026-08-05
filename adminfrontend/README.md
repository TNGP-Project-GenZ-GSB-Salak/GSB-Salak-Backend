# adminfrontend

A barebones vanilla HTML/CSS/JS admin panel for the GSB Salak API — a login page plus a
"Salak Actions" page that can force-settle a matured holding immediately (see
`internal/admin`). CSS is layout-only — no colors, fonts, or other decoration. Mirrors
`../testfrontend`'s shape (same zero-dependency static server, same `api.js` fetch-helper
convention), since this is the same kind of thing: an internal-only client, not a real
product surface.

Deliberately minimal today (one action, one page) but structured — one `<section>` per
concern on `dashboard.html` — so it can grow into an observability dashboard later without
redoing the login/auth plumbing.

It's served by its own static file server on a different origin/port than the Go API, so
the API's CORS middleware (`internal/platform/middleware/cors.go`) is what allows it.

## Setup

No `npm install` — zero dependencies, no build step.

The API's Postgres must already be running, migrated, and seeded with an admin
credential (from the repo root):

```sh
docker compose up -d
go run ./cmd/migrate up
ADMIN_USERNAME=admin ADMIN_PASSWORD=<a real password> go run ./cmd/seed
```

`cmd/seed` skips creating an admin credential entirely if `ADMIN_USERNAME`/
`ADMIN_PASSWORD` aren't set — see `internal/platform/config/config.go`.

## Running

```sh
node server.js        # serves adminfrontend/ on http://localhost:5175 (PORT env var to override)
```

In another terminal, from the repo root: `go run ./cmd/api` (serves the API on `:8080`).
Then open `http://localhost:5175` and log in with the admin credential seeded above.

Or from `GSB-Salak-Backend/`: `make admin-run`.

## Using the Salak Actions page

Paste a real holding ID (visible via `GET /salak/holdings?account_id=...`, or from
`testfrontend`/`GSB-Salak-Frontend`'s own UI) into the "Holding ID" field and submit.
Settling an already-settled holding returns a 409, shown as an error message rather than
a crash.
