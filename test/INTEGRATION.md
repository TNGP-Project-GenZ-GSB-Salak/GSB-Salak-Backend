# Integration tests

This documents the Go integration test suite in `test/integration/`. Where the unit
suite (`test/UNIT.md`) covers the service layer against hand-rolled fakes, this suite
exercises the real GORM repository layer against a **real Postgres database** — the one
boundary fakes structurally cannot verify: row-locking (`SELECT ... FOR UPDATE`),
unique/check/FK constraint enforcement, `ON CONFLICT` upsert semantics, and whether
`BuySalakService`'s cross-domain `db.Transaction(...)` actually commits/rolls back
against real SQL (the existing unit test for it uses `sqlmock`, which only confirms
`Rollback()` was *called*, not that Postgres actually undid real row changes).

## Scope

**In scope**: `internal/<domain>/repository/gorm_*.go` — the five GORM repository
implementations — plus one test proving the cross-domain transaction mechanics in
`internal/transaction/service/buy_salak_service.go` actually work against real Postgres.

**Out of scope** (already covered elsewhere, not re-tested here):
- HTTP handlers/DTOs — Playwright E2E (`testfrontend`) covers the full request/response
  cycle through real handlers.
- Validation rules, ownership checks, arithmetic, orchestration branching — the
  service-layer unit tests (`test/UNIT.md`) already cover every edge case there with
  fakes; the one cross-domain test in this suite exists solely to prove the *transaction
  mechanics* work, not to re-verify `ValidatePurchase` or account-type checks.

## Running

```sh
make test-integration        # docker-up + migrate-up, then go test -tags=integration ./test/integration/...
```

Or manually:

```sh
make docker-up
make migrate-up
go test -tags=integration ./test/integration/...
```

**Not wired into `make test` or the Claude Code Stop hook** — deliberately manual-only.
Every file is gated behind `//go:build integration`, so plain `go test ./...` (what
`make test` and the Stop hook already run) continues to see zero integration tests,
unchanged. If the DB isn't reachable, every test in the suite calls `t.Skip` individually
(via `newTestTx`/direct `sharedDB == nil` checks) rather than the binary failing outright
— confirmed by running `make docker-down` and re-running `go test -tags=integration
./test/integration/...`, which produces all `SKIP`, zero `FAIL`.

## Isolation strategy

`testdb_test.go`'s `TestMain` opens one shared `*gorm.DB` (via
`internal/platform/config.Load()` + `internal/platform/db.Open()` — same env vars/
defaults as `cmd/api`) and pings it once. Every ordinary test then calls `newTestTx(t)`,
which:

1. Skips the test gracefully if the shared DB is unreachable.
2. Opens its own transaction off the shared `*gorm.DB`.
3. Registers `tx.Rollback()` via `t.Cleanup`.

Repos are constructed directly on that per-test `tx` (e.g.
`accountrepo.NewGormAccountRepository(tx)`), and the same `tx` is passed again wherever a
method takes an explicit `tx *gorm.DB` parameter — so every read and write in a test runs
inside the one transaction that gets rolled back at the end. Nothing a test does ever
persists, including row locks (released the instant `Rollback()` runs) and — critically —
the `salak.ticket_sequence` singleton row that already exists post-migration, which
would otherwise leak mutations between tests.

Tests do **not** call `t.Parallel()`. Two reasons: row-locking tests only make sense
against one active transaction at a time, and the two exceptions below deliberately break
the rollback pattern in ways that would race against any other test touching the same
singleton row.

## The two exceptions to the rollback pattern

### Concurrent `ReserveTicketRange` (`holding_repo_test.go`)

`TestReserveTicketRange_ConcurrentCallersGetDisjointContiguousRanges` is the single most
valuable test in this suite — proving the row lock in `ReserveTicketRange` actually
serializes access under real contention. This *cannot* use the rollback-per-test pattern:
a `*gorm.DB` bound to one `*sql.Tx` isn't safe for concurrent goroutines (same rule as
`database/sql`'s `*sql.Tx`), so each "concurrent caller" must be a genuinely separate
`Begin()`+`Commit()` against the shared `*gorm.DB` — which means it **permanently
advances** the real `salak.ticket_sequence` singleton. The test snapshots the counter
before and after and asserts on **deltas** (each reservation spans exactly the requested
width, reservations are contiguous, the counter advances by exactly what was requested),
never absolutes. Verified non-flaky by running it repeatedly (`go test -tags=integration
./test/integration/... -run TestReserveTicketRange_Concurrent -count=5`).

Same reasoning applies to `TestAccountRepo_Debit_NoLostUpdateAcrossSequentialTransactions`
in `account_repo_test.go`: it needs its fixture account to persist across two separately
committed transactions, so it commits its own fixture directly against the shared DB and
cleans up with a raw `DELETE` in `t.Cleanup`, rather than using `newTestTx`.

### Cross-domain transaction mechanics (`buy_salak_flow_test.go`)

Constructs the *real* `account.Service`, `salak.Service`, `transaction.LedgerRepository`,
and `BuySalakService` — wired the same way `cmd/api/main.go` does — pointed at one
`newTestTx` transaction. `BuySalakService.BuySalak` calls `s.db.Transaction(...)`
internally; GORM detects the connection is already inside an open transaction and
transparently uses a `SAVEPOINT` instead of a real nested `BEGIN`, so the full
debit→mint→credit→ledger flow still runs as real SQL and still rolls back cleanly via the
outer `t.Cleanup` — no exception to the pattern needed here, unlike the two tests above.
Two cases:

- **Happy path**: real debit, real mint (with a real reserved ticket range), real credit,
  two real ledger entries sharing one `reference_id`.
- **Rollback path**: deletes the `salak.ticket_sequence` row *inside the test's own
  transaction* before calling `BuySalak`, so `MintHolding`'s first query (a `SELECT ...
  FOR UPDATE` against a row that's no longer there) fails with a genuine
  `gorm.ErrRecordNotFound` — strictly after the debit already ran. Asserts the funding
  account's balance is completely unchanged and no holding/ledger rows exist, proving
  Postgres actually discarded the real row changes (not just that `Rollback()` was
  called, which is all the `sqlmock`-based unit test for this service can confirm).

## What's tested, per file

- **`account_repo_test.go`**: create/find round-trip (decimal precision), duplicate
  `account_number` (unique violation), invalid `type` and negative `balance` (check
  violations), unknown `user_id` (FK violation), no-lost-update across two sequential
  committed transactions.
- **`user_repo_test.go`**: create/find round-trip, duplicate `username` (unique
  violation), not-found returns the real `gorm.ErrRecordNotFound` (not a fake's
  hand-returned sentinel).
- **`product_repo_test.go`**: `Upsert` on conflicting `code` updates the existing row
  in place (keeps its original `id`) rather than erroring or duplicating; invalid
  `term_months` and `max_purchase < min_purchase` (check violations); `ListActive`
  filters `is_active` and orders by `term_months ASC`.
- **`holding_repo_test.go`**: all three holding check constraints
  (`ticket_end > ticket_start`, `units > 0`, `ticket_end - ticket_start + 1 = units`);
  FK violations on unknown `account_id`/`product_id`; `FindByAccountID` ordering;
  sequential `ReserveTicketRange` contiguity; the concurrent `ReserveTicketRange` test
  (see above).
- **`ledger_repo_test.go`**: invalid `type` and non-positive `amount` (check
  violations); unknown `account_id` (FK violation); `FindByAccountID` limit/offset/
  ordering; `HoldingID` round-trips both `nil` and a real holding reference.
- **`buy_salak_flow_test.go`**: the cross-domain transaction mechanics test (see above).

## A note on one debugging detour

The connection/rollback helper file was initially named `testdb.go` (and the fixture
helper file `fixtures.go`) instead of `testdb_test.go`/`fixtures_test.go`. Go only treats
`TestMain` (and any `TestXxx` function) as special when it's declared in a file whose
name ends in `_test.go` — in a plain `.go` file it's just an ordinary exported function
that Go's test runner never calls, even with a matching signature and no compiler error
to flag it. The symptom was every test skipping with "integration DB unreachable" despite
Postgres being up, migrated, and directly reachable — because `TestMain` was silently
never invoked, so `sharedDB` stayed `nil`. Worth remembering if a future `TestMain`-based
helper file in this repo ever needs renaming.
