# Unit tests

This documents the Go unit test suite added under `internal/`. Before this,
`go test ./...` had no test files anywhere in the module (see the root
`CLAUDE.md`'s note: "test suite not yet written — see test/integration/").
Real-Postgres integration tests now live in `test/integration/` (see
`test/INTEGRATION.md`); end-to-end/UI coverage runs separately against the real
`GSB-Salak-Frontend` app via Playwright (see root `CLAUDE.md`).

This file is the high-level narrative — what's covered, why, and per-package coverage
percentages. For a granular, one-row-per-scenario breakdown (`Prerequisites / Expected /
Actual` for every individual test case, including each table-driven subtest counted
separately), see **`test/TEST_CASES.md`**.

## Scope

Unit tests target the **service layer** (business logic — validation,
authorization, arithmetic, orchestration) and small **platform packages**
(`apperror`, `jwtutil`, `middleware`) that are pure logic with no live
Postgres dependency. Every test is a real `go test`, runs in milliseconds,
and needs no `docker compose up`.

**Out of scope** (0% unit coverage by design, not by oversight):

| Layer | Why it's out of scope here |
|---|---|
| `internal/*/repository` (GORM repos) | Thin wrappers around real SQL/GORM calls (locking clauses, upserts, joins) — correctness depends on an actual Postgres schema, so these belong in `test/integration/`, not mocked unit tests. |
| `internal/*/http` (chi handlers + DTOs) | Wiring/serialization over the already-tested service layer; better exercised end-to-end via `GSB-Salak-Frontend`'s Playwright suite, which drives real HTTP requests through real handlers. |
| `internal/platform/config`, `internal/platform/db`, `internal/platform/httpserver/docs.go` | Environment/bootstrap glue (env var parsing, `sql.Open`, Swagger doc generation) with no branching logic worth unit-testing in isolation. |
| `cmd/*`, `docs/`, `migrations/` | Entry points and generated/SQL artifacts, not unit-testable logic. |

## Approach

- **No mocking framework.** Each test file hand-rolls small fakes for the
  `ports.go` interfaces it depends on (e.g. `fakeAccountRepo`, `fakeSalakService`).
  The interfaces are small (2-5 methods), so a real mocking library would add
  indirection without saving much code.
- **`github.com/stretchr/testify`** (`assert`/`require`) for assertions —
  already an indirect dependency via `swaggo`; promoted to direct via `go mod
  tidy` once tests started importing it.
- **`github.com/DATA-DOG/go-sqlmock`** (added as a new dependency) backs the
  one test file that needs a real `*gorm.DB`:
  `transaction/service/buy_salak_service_test.go`. `BuySalakService.BuySalak`
  calls `s.db.Transaction(func(tx *gorm.DB) error {...})` directly (not
  through an interface), so it needs *something* that can `Begin`/`Commit`/
  `Rollback`. `gorm.io/driver/postgres`'s `Dialector.Initialize` sets
  `db.ConnPool` straight to whatever `*sql.DB` you hand it via
  `postgres.Config{Conn: ...}`, issuing no setup queries — so a
  `sqlmock`-backed `*sql.DB` works with zero fuss, and `mock.ExpectBegin()` /
  `ExpectCommit()` / `ExpectRollback()` verify the transaction actually
  commits on success and rolls back on every failure branch. Everything
  *inside* the transaction callback (accounts, salak, ledger) is still a hand
  fake, so no real SQL is ever executed — only the transaction envelope
  itself is real.
- Table-driven subtests (`t.Run`) are used throughout for the enumerable
  edge-case matrices (validation boundaries, error-kind mapping, limit
  clamping, etc).

## Test files and what they cover

### `internal/platform/apperror/errors_test.go`
The typed-error package every service returns instead of raw `gorm`/stdlib errors.
- `Error()` message formatting with and without a wrapped cause.
- `Unwrap()` / `errors.Is` see through to the wrapped cause; `nil` when there's no cause.
- All six constructors (`Validation`, `NotFound`, `Unauthorized`, `Forbidden`, `Conflict`, `Internal`) set the right `Kind`/`Message`/`Err`.
- `HTTPStatus` maps every `Kind` to its HTTP status, **plus edge cases**: a plain non-`*Error`, a `nil` error, and an unrecognized `Kind` string all fall back to 500.

### `internal/platform/jwtutil/jwt_test.go`
JWT signing/parsing used by login and the auth middleware.
- Sign → Parse round trip recovers the same user ID.
- **Edge cases**: expired token (negative expiry), wrong secret, malformed tokens (empty string, non-JWT string, wrong segment count), a token signed with `alg: none` (must be rejected even though `token.Valid` would otherwise pass — the keyfunc explicitly checks for `*jwt.SigningMethodHMAC`), and confirming a failed parse always returns `uuid.Nil`.

### `internal/platform/middleware/auth_test.go`, `cors_test.go`, `recover_test.go`, `requestlog_test.go`
The HTTP middleware chain (plain `net/http.Handler`, composed by chi).
- **Auth**: missing header, non-Bearer schemes (including a case-sensitivity check and a missing-space malformed header), invalid token, expired token, and the success path (context carries the parsed user ID through to the next handler). `UserIDFromContext` absent/present, plus a documented guarantee that a colliding-but-different context key type can't spoof a user ID (the key type is unexported).
- **CORS**: headers are set on normal requests; an `OPTIONS` preflight short-circuits with `204` and never reaches the wrapped handler.
- **Recover**: a panic (string or `error` value) is converted to a `500` JSON body instead of crashing the process; a non-panicking request passes through unchanged.
- **RequestLog**: status code and body pass through untouched; a handler that never explicitly writes a status still reports `200` (the chi `WrapResponseWriter` default).

### `internal/user/service/auth_service_test.go`
`AuthService` — registration, login, lookup.
- **Register**: success (password gets bcrypt-hashed, not stored in plaintext); every blank-field combination; the 8-character password minimum, including the **exact boundary** (8 chars passes, 7 fails); username-already-taken → `Conflict`; an unexpected repo error during the uniqueness check → `Internal` (distinguished from the expected "not found" case); repo `Create` failure → `Internal`. A documented-behavior test pins down that whitespace-only fields are **not** treated as blank (no `strings.TrimSpace` today), so a future accidental trim doesn't silently change behavior.
- **Login**: success returns both the user and a token that actually parses back to the same ID; unknown username and wrong password both return the *same* `Unauthorized` kind (not a distinguishable 404, to avoid username enumeration); an unexpected repo error → `Internal`; empty-string password never matches any real hash.
- **GetByID**: success, not-found, unexpected repo error.

### `internal/account/service/account_service_test.go`
`AccountService` — the money-movement primitives every purchase is built on.
- **ListByUser**: only that user's accounts come back; empty list is not an error; repo error → `Internal`.
- **GetByID**: success; not-found; **an account that exists but belongs to a different user returns `NotFound`, not `Forbidden`** (documents the deliberate anti-enumeration choice already in the code); repo error → `Internal`.
- **Debit**: success reduces balance correctly; **debiting the exact balance to zero succeeds** (boundary — zero is not "negative"); insufficient funds is rejected; account-not-found; lock/lookup error → `Internal`; `UpdateBalance` failure → `Internal`.
- **Credit**: success increases balance; account-not-found; lock/lookup error; update failure; crediting zero is a no-op success.

### `internal/salak/domain/holding_test.go`, `product_test.go`
Pure formatting/struct logic, no service involved.
- `TicketStartID`/`TicketEndID` zero-padding: a typical mid-range number, `0` (fully padded), **exactly 7 digits** (fills the field with no padding), **more than 7 digits** (not truncated — a real edge case once the ticket sequence grows past 9,999,999), and a single digit.
- Start and end IDs share the same ticket letter.
- `TableName()` for `Holding`, `TicketSequence`, and `Product`.

### `internal/salak/service/salak_service_test.go`
`SalakService` — product catalog, purchase validation, and ticket minting.
- **ListProducts / GetProduct**: success; not-found; repo error → `Internal`; **an inactive product is rejected even though it exists** (`Validation`, not `NotFound`).
- **ValidatePurchase**: zero and negative amounts; below minimum; above maximum; not a multiple of the step amount; and the two **inclusive boundary cases** (exactly at minimum, exactly at maximum — both must be valid, since the code uses `LessThan`/`GreaterThan`, not `<=`/`>=`).
- **MintHolding**: success (verifies `Units` is computed as `amount / unitPrice` truncated, the reserved ticket range flows through unchanged into the holding, `MaturityDate` is `PurchaseDate + TermMonths`, and the ticket letter is a valid single Thai consonant rune in the exact `[0x0E01, 0x0E2E]` range the production code draws from — checked numerically rather than by hand-copying all 46 Thai characters into the test, which would be fragile); an amount that isn't an exact multiple of the unit price **truncates down** rather than erroring (documents current behavior — full-amount validation happens earlier in `ValidatePurchase`, not here); an amount below one whole unit price is rejected; product not found; product lookup error; ticket-range reservation failure; holding create failure.
- **ListHoldingsByAccount**: success only after the ownership check passes (and confirms the correct `userID`/`accountID` were forwarded to `account.Service.GetByID`); an ownership-check failure is propagated **verbatim** (not rewrapped); holdings-repo error → `Internal`; no holdings is an empty slice, not an error.

### `internal/transaction/service/buy_salak_service_test.go`
`BuySalakService` — the cross-domain orchestration that debits, mints, credits, and ledgers atomically. This is the most complex unit under test since it's the one place a real `*gorm.DB` transaction envelope is involved (see sqlmock note above).
- **Pre-transaction validation** (no DB transaction opened at all for these): funding account == salak account is rejected; a funding/salak account lookup failure is propagated verbatim; the funding account must be `savings`-type and the salak account must be `salak`-type (each checked with the *other* type on purpose); a product lookup failure and a `ValidatePurchase` failure both propagate verbatim.
- **Success path**: `mock.ExpectBegin()` → `ExpectCommit()`; asserts the full receipt (product name, units, amount, both post-transaction balances, a non-nil reference ID), that exactly two ledger entries were written (one `debit` on the funding account, one `credit` on the salak account), that **both entries share one `reference_id`** (the pairing invariant the whole ledger design depends on), that the holding ID is attached to both entries, and that the fake `badge.Service` was never called since no badge was supplied.
- **Badge-ownership gate** (`badgeID *uuid.UUID`, optional): when supplied and owned, the purchase proceeds exactly as the no-badge case, and `badge.Service.UserOwnsBadge` is confirmed called; when supplied but not owned, the purchase is rejected with `apperror.KindForbidden` **before any DB transaction opens** (no `sqlmock` expectations needed — the fake `db` is `nil`); an error from the ownership check itself (not a "not owned" result) maps to `apperror.KindInternal`. When `badgeID` is `nil`, behavior is byte-for-byte identical to before this gate existed.
- **Rollback paths**, one per failure point inside the transaction, each asserting `ExpectBegin()` → `ExpectRollback()` and that the original error is propagated: debit failure (insufficient funds), mint-holding failure (ticket reservation lock timeout), credit failure, and ledger-write failure (the last op in the transaction — confirms a late failure still rolls back the earlier debit/mint/credit, not just fails silently).
- **ListHistory**: ownership-check-failure propagation; non-positive limit (`0`, `-1`, `-100`) defaults to 20; limit above 100 clamps to 100; **limit exactly at the 100 boundary is not clamped** (off-by-one guard); negative offset clamps to 0; repo error → `Internal`.

### `internal/chooser/chooser_test.go`
`Chooser` — a generic weighted-random-index picker (`math/rand/v2`-based) that both `WeightedRandomBadgeService` (below) and, in principle, any future weighted-pick need can share.
- **Construction validation**: an empty weights slice, a negative weight, and all-zero weights are all rejected (a mix of zero and positive weights is valid, since the total is still > 0).
- **`Pick`**: a single-weight chooser always returns index 0; an entry with zero weight is never picked even when mixed with a positive-weight entry (1,000 draws); and a 3-way weight split (0.5/0.3/0.2) produces a 10,000-draw distribution within ±5% of each expected share — a statistical check, not an exact one, since the picker is genuinely randomized.

### `internal/badge/service/random_badge_service_test.go`
Two services live in this package: `WeightedRandomBadgeService` (fully unit-tested here) and the thin `BadgeService` ownership-check pass-through consumed by the transaction badge gate above (**not** directly unit-tested — see the coverage gap noted below; its behavior is exercised indirectly through `buy_salak_service_test.go`'s fake and the real integration test).
- **Construction validation**: mirrors `chooser`'s rules through `NewWeightedRandomBadgeService` — empty badges slice, all-zero-weight badges, and a negative-weight badge are all rejected.
- **`GetRandomBadge`**: returns one of the configured badges; a 3-badge/weight (0.5/0.3/0.2) distribution check over 10,000 draws stays within ±5% of each badge's expected share, same statistical style as the chooser test.

## Coverage summary

Measured with `go test ./... -cover`. Percentages are statement coverage of
the packages that have tests; packages with `[no test files]` or `0.0%` are
out-of-scope per the table above, not gaps.

| Domain | Package | Coverage | Notes |
|---|---|---:|---|
| Platform | `internal/platform/apperror` | 100.0% | |
| Platform | `internal/platform/jwtutil` | 100.0% | |
| Platform | `internal/platform/middleware` | 100.0% | |
| Platform | `internal/chooser` | 100.0% | |
| User | `internal/user/service` | 93.9% | Gaps are `bcrypt.GenerateFromPassword` and `signer.Sign` error branches — practically unreachable without fault-injecting those libraries. |
| Account | `internal/account/service` | 100.0% | |
| Salak | `internal/salak/domain` | 100.0% | |
| Salak | `internal/salak/service` | 96.2% | Gap is the `crypto/rand.Int` failure branch inside `randomTicketLetter` — unreachable without a fake entropy source. |
| Transaction | `internal/transaction/service` | 98.4% | Same `crypto/rand` branch, reached transitively through `MintHolding`. |
| Badge | `internal/badge/service` | 84.6% | `WeightedRandomBadgeService` is 100% covered; the gap is entirely `BadgeService.NewBadgeService`/`UserOwnsBadge` (the thin ownership-check pass-through used by the transaction badge gate) — it has no direct unit test yet, only indirect exercise via `buy_salak_service_test.go`'s fake and `test/integration/buy_salak_flow_test.go`'s real-Postgres path. |

**Per-domain business-logic average** (the five `internal/<domain>/service`
packages, i.e. the actual thing this task asked to test): **(100.0 + 93.9 +
96.2 + 98.4 + 84.6) / 5 ≈ 94.6%**.

**Overall `go test ./... -cover` statement coverage across the entire
module** (every package, including the intentionally-untested repository/
http/config/db/cmd/docs/migrations layers): **43.5%** — low only because the
denominator includes ~2,500 lines of GORM/chi/bootstrap code this task
deliberately left to integration/E2E testing (see Scope above), not because
service-layer logic is under-tested.

## Running

```sh
go test ./...                    # whole suite
go test ./... -cover             # with per-package coverage
go test ./internal/... -v        # verbose, every sub-test name
go build ./... && go vet ./... && gofmt -l .   # sanity-check alongside tests
```

All of the above are clean as of this writing: `go build`, `go vet`, and
`gofmt -l .` report nothing, and every test passes (174 passing test cases —
top-level tests plus table-driven subtests — across 14 `_test.go` files).
