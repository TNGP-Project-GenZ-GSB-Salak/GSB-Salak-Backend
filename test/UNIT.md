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

### `internal/platform/jwtutil/jwt_test.go`, `admin_test.go`
JWT signing/parsing used by login and the auth middleware — customer (`Signer`) and admin (`AdminSigner`) each get an identical test shape, since they're deliberately parallel, separately-secreted types.
- Sign → Parse round trip recovers the same user/admin ID.
- **Edge cases**, both signers: expired token (negative expiry), wrong secret, malformed tokens (empty string, non-JWT string, wrong segment count), a token signed with `alg: none` (must be rejected even though `token.Valid` would otherwise pass — the keyfunc explicitly checks for `*jwt.SigningMethodHMAC`), and confirming a failed parse always returns `uuid.Nil`.
- **Admin-only**: a validly-signed customer token, even under the same secret string, decodes to `uuid.Nil` (not a real admin) against `AdminSigner.Parse` — `AdminClaims` simply has no `user_id` field to read, so the JSON payload's `admin_id` stays zero-valued. Pins down the exact reason a customer token can never masquerade as an admin one.

### `internal/platform/middleware/auth_test.go`, `admin_auth_test.go`, `cors_test.go`, `recover_test.go`, `requestlog_test.go`
The HTTP middleware chain (plain `net/http.Handler`, composed by chi).
- **Auth / AdminAuth**: missing header, non-Bearer schemes (including a case-sensitivity check and a missing-space malformed header), invalid token, expired token, and the success path (context carries the parsed user/admin ID through to the next handler). `UserIDFromContext`/`AdminIDFromContext` absent/present, plus a documented guarantee that a colliding-but-different context key type can't spoof an ID (the key type is unexported, and admin's is its own distinct type from the customer one). **AdminAuth-only**: a real, validly-signed customer JWT is rejected outright (signed with a different secret than `ADMIN_JWT_SECRET`, so signature verification fails before `AdminClaims` decoding is ever reached) — the actual security property `AdminAuth` exists for, not just "some role check".
- **CORS**: headers are set on normal requests; an `OPTIONS` preflight short-circuits with `204` and never reaches the wrapped handler.
- **Recover**: a panic (string or `error` value) is converted to a `500` JSON body instead of crashing the process; a non-panicking request passes through unchanged.
- **RequestLog**: status code and body pass through untouched; a handler that never explicitly writes a status still reports `200` (the chi `WrapResponseWriter` default).

### `internal/admin/service/admin_service_test.go`
`AdminService` — login for the minimal internal-ops identity (username + bcrypt hash, no roles). Structurally identical to `AuthService.Login` below, since it's the same anti-enumeration shape applied to a second, unrelated identity.
- **Login**: success returns the admin and a token that parses back to the same ID via `AdminSigner`; unknown username and wrong password both return the *same* `Unauthorized` kind (no distinguishable 404, same anti-enumeration reasoning as customer login); an unexpected repo error → `Internal`; empty-string password never matches any real hash.

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

### `internal/salak/domain/holding_test.go`, `product_test.go`, `ticket_letters_test.go`
Pure formatting/struct logic, no service involved.
- `TicketStartID`/`TicketEndID` zero-padding: a typical mid-range number, `0` (fully padded), **exactly 7 digits** (fills the field with no padding), **more than 7 digits** (not truncated — a real edge case once the ticket sequence grows past 9,999,999), and a single digit.
- Start and end IDs share the same ticket letter.
- `TableName()` for `Holding`, `TicketSequence`, and `Product`.
- `NextLetter` (the per-product ticket cursor's letter-advance rule): a normal advance (ก→ข); **skips ฤ and ฦ** (ร→ล, ล→ว — advancing is a skip, not a naive `+1`, since those two code points are vowels, not consonants); errors past the last letter (ฮ) and on any input that isn't one of the 44 real consonants.

### `internal/salak/service/salak_service_test.go`
`SalakService` — product catalog, purchase validation, and ticket minting.
- **ListProducts / GetProduct**: success; not-found; repo error → `Internal`; **an inactive product is rejected even though it exists** (`Validation`, not `NotFound`).
- **ValidatePurchase**: zero and negative amounts; below minimum; above maximum; not a multiple of the step amount; and the two **inclusive boundary cases** (exactly at minimum, exactly at maximum — both must be valid, since the code uses `LessThan`/`GreaterThan`, not `<=`/`>=`).
- **MintHolding**: success (verifies `Units` is computed as `amount / unitPrice` truncated, `productID` is forwarded to `ReserveTicketRange`, the reserved letter+range flow through unchanged into the holding — asserted against the exact value the fake returned, not just "some valid consonant", since a random letter is exactly the bug this now guards against — and `MaturityDate` is `PurchaseDate + TermMonths`); an amount that isn't an exact multiple of the unit price **truncates down** rather than erroring (documents current behavior — full-amount validation happens earlier in `ValidatePurchase`, not here); an amount below one whole unit price is rejected; product not found; product lookup error; ticket-range reservation failure (both the generic-error → `Internal` path and the `ErrUnitsExceedLetterCapacity` → `Validation` path); holding create failure.
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

### `internal/kapook/domain/goal_test.go`, `transaction_test.go`, `terms_acceptance_test.go`, `withdrawal_window_test.go`
Pure struct/formatting logic for the Kapook (กระปุกออม) goal-saving feature, no service involved.
- `Goal.AvailableBalance` (`SavingAmount - SalakAmount`).
- `WithdrawalWindow`'s rolling-12-month free-withdrawal window computation.
- `TableName()` for `Transaction` and `TermsAcceptance`.

### `internal/kapook/service/kapook_service_test.go`
`KapookService` — terms acceptance, goal lifecycle, deposits/withdrawals, and the goal-buy/settlement paths, the largest test file in the suite.
- **Accept / HasAccepted**: idempotent acceptance; the has-accepted read path.
- **CreateGoal**: ownership/account-type checks, terms-accepted gate, the "at most one active goal per account" rule, and goal-amount validation against the product's step/max.
- **GetActiveGoal / Snapshot**: the no-active-goal empty state; the derived read model (`AvailableBalance`, `TargetReached`, `CountdownRemainingSeconds`, `BuyEligible`, and the auto-purchase failure-tracking fields `AutoPurchaseAttempts`/`AutoPurchaseLastError` surfaced for the worker-observability admin panel).
- **Deposit / Withdraw**: balance/target-exceeded rejections, the goal-reached countdown stamping on the deposit that first hits the target, the free-withdrawal-count/fee computation, and the all-or-nothing full-withdrawal-during-a-live-countdown rule.
- **BuyFromGoal / BuyFromGoalInTx**: partial vs full purchases, goal deactivation once fully bought, and the tx-supplied variant the worker uses.
- **GetGoalHistory / SettleMaturedHolding**: ownership-scoped history, and the settlement wrapper's Kapook-specific bookkeeping (decrementing `SalakAmount`, recording a `salak_expiration` row) only when a holding traces back to a goal.

### `internal/kapook/worker/worker_test.go`
`Worker` — the unattended auto-purchase poller (`ClaimDueGoals` → buy → mark done/failed/deferred).
- **RunOnce**: the happy path (claims a due goal, buys its full available balance, deactivates it); a goal not yet due is left untouched; a draw-day rejection defers the goal and persists the retry date; any other failure is recorded via `RecordAutoPurchaseFailure` (attempts incremented, last error/timestamp stamped) and the goal is left active for the next tick — never a give-up/dead-letter state.

## Coverage summary

Moved to `docs/tests/unit/COVERAGE.md` in the **monorepo root** (i.e.
outside this submodule — `../../docs/tests/unit/COVERAGE.md` from here),
so the numbers can be regenerated/tracked independently of this narrative
file. Regenerate it (`go test ./... -cover`) whenever a package's test
coverage changes meaningfully, the same way this file itself should be
updated when a new test file or domain is added.

## Running

```sh
go test ./...                    # whole suite
go test ./... -cover             # with per-package coverage
go test ./internal/... -v        # verbose, every sub-test name
go build ./... && go vet ./... && gofmt -l .   # sanity-check alongside tests
```

All of the above are clean as of this writing: `go build`, `go vet`, and
`gofmt -l .` report nothing, and every test passes (384 passing test cases —
top-level tests plus table-driven subtests — across 28 `_test.go` files
under `internal/`).
