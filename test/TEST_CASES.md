# Detailed unit test case reference

This is the granular companion to `test/UNIT.md`: **151 rows**, one per individual test
scenario (every table-driven `t.Run` case counted separately, not just its parent
function), in a `Prerequisites / Expected / Actual` format. See `test/UNIT.md` for the high-level
narrative — what's covered and why, per-package coverage percentages, and what's
deliberately out of scope. This document only covers the **unit** suite
(`internal/**/*_test.go`); `test/integration/` has its own reference,
`test/INTEGRATION.md`.

Every row below was transcribed directly from the corresponding `_test.go` file's actual
fixture setup and assertions — "Actual" is not a copy of "Expected"; it confirms what the
test currently observes, including any documented-behavior nuance called out in the test
itself or in `UNIT.md`. All rows below are currently passing (`go test ./...`).

## 1. `internal/platform/apperror/errors_test.go`

| ID | Test Case | Prerequisites | Expected | Actual |
|---|---|---|---|---|
| APPERR-01 | `Error()` message without a wrapped cause | `*apperror.Error` built via `apperror.New(KindValidation, "bad input")`, no wrapped err | `.Error()` returns exactly `"bad input"` | ✅ Returns `"bad input"` |
| APPERR-02 | `Error()` message appends the wrapped cause | `apperror.Wrap(KindInternal, "failed to save", cause)` where `cause = errors.New("db exploded")` | `.Error()` returns `"failed to save: db exploded"` | ✅ Returns `"failed to save: db exploded"` |
| APPERR-03 | `Unwrap` returns the wrapped error | `apperror.Wrap(KindInternal, "wrapper", cause)`, `cause = errors.New("root cause")` | `errors.Unwrap(e)` is the exact same `cause` value | ✅ Returns the same `cause` instance |
| APPERR-04 | `Unwrap` is nil when nothing was wrapped | `apperror.New(KindValidation, "bad input")`, no wrapped err | `errors.Unwrap(e)` is `nil` | ✅ Returns `nil` |
| APPERR-05 | `errors.Is` sees through to the wrapped cause | `apperror.Wrap(KindInternal, "wrapper", cause)`, `cause = errors.New("sentinel")` | `errors.Is(e, cause)` is `true` | ✅ Returns `true` |
| APPERR-06 | `Validation()` sets Kind/Message, no wrapped error | none (pure constructor call) | Returns `{Kind: KindValidation, Message: "v", Err: nil}` | ✅ Matches expected |
| APPERR-07 | `NotFound()` sets Kind/Message, no wrapped error | none | Returns `{Kind: KindNotFound, Message: "nf", Err: nil}` | ✅ Matches expected |
| APPERR-08 | `Unauthorized()` sets Kind/Message, no wrapped error | none | Returns `{Kind: KindUnauthorized, Message: "u", Err: nil}` | ✅ Matches expected |
| APPERR-09 | `Forbidden()` sets Kind/Message, no wrapped error | none | Returns `{Kind: KindForbidden, Message: "f", Err: nil}` | ✅ Matches expected |
| APPERR-10 | `Conflict()` sets Kind/Message, no wrapped error | none | Returns `{Kind: KindConflict, Message: "c", Err: nil}` | ✅ Matches expected |
| APPERR-11 | `Internal()` sets Kind/Message and preserves the wrapped error | `cause = errors.New("boom")` | Returns `{Kind: KindInternal, Message: "i", Err: cause}` | ✅ Matches expected |
| APPERR-12 | Validation kind maps to HTTP 400 | `apperror.Validation("x")` | `HTTPStatus(err)` returns `400` | ✅ Returns 400 |
| APPERR-13 | NotFound kind maps to HTTP 404 | `apperror.NotFound("x")` | `HTTPStatus(err)` returns `404` | ✅ Returns 404 |
| APPERR-14 | Unauthorized kind maps to HTTP 401 | `apperror.Unauthorized("x")` | `HTTPStatus(err)` returns `401` | ✅ Returns 401 |
| APPERR-15 | Forbidden kind maps to HTTP 403 | `apperror.Forbidden("x")` | `HTTPStatus(err)` returns `403` | ✅ Returns 403 |
| APPERR-16 | Conflict kind maps to HTTP 409 | `apperror.Conflict("x")` | `HTTPStatus(err)` returns `409` | ✅ Returns 409 |
| APPERR-17 | Internal kind maps to HTTP 500 | `apperror.Internal("x", errors.New("y"))` | `HTTPStatus(err)` returns `500` | ✅ Returns 500 |
| APPERR-18 | A plain non-`*apperror.Error` defaults to 500 | `errors.New("plain")` | `HTTPStatus(err)` returns `500` | ✅ Returns 500 |
| APPERR-19 | A `nil` error defaults to 500 | `err = nil` | `HTTPStatus(nil)` returns `500` | ✅ Returns 500 |
| APPERR-20 | An unrecognized `Kind` string still defaults to 500 | `apperror.New(Kind("something_new"), "mystery")` | `HTTPStatus(err)` returns `500` | ✅ Returns 500 — unknown kinds don't crash, they fail safe |

## 2. `internal/platform/jwtutil/jwt_test.go`

| ID | Test Case | Prerequisites | Expected | Actual |
|---|---|---|---|---|
| JWT-01 | Sign → Parse round trip recovers the same user ID | Signer with secret `"test-secret"`, 60-min expiry; a random `userID` | Parsing the signed token returns the exact same `userID`, no error | ✅ Returns the same `userID` |
| JWT-02 | Expired token is rejected | Signer with expiry = -1 minutes (already expired at signing time) | `Parse` on the resulting token returns an error | ✅ Returns an error |
| JWT-03 | Token signed with a different secret is rejected | Two signers with different secrets (`"secret-a"`/`"secret-b"`); token signed by the first | Parsing with the second signer returns an error | ✅ Returns an error |
| JWT-04 | Empty string is rejected as a token | Input token string = `""` | `Parse("")` returns an error | ✅ Returns an error |
| JWT-05 | Non-JWT string is rejected as a token | Input token string = `"not-a-jwt"` | `Parse(...)` returns an error | ✅ Returns an error |
| JWT-06 | Token with the wrong segment count is rejected | Input token string = `"a.b.c"` | `Parse(...)` returns an error | ✅ Returns an error |
| JWT-07 | Token signed with the "none" algorithm is rejected | A token crafted with `jwt.SigningMethodNone` + `jwt.UnsafeAllowNoneSignatureType`, otherwise unexpired | `Parse` returns an error even though `token.Valid` would otherwise pass | ✅ Returns an error — the keyfunc explicitly requires `*jwt.SigningMethodHMAC` |
| JWT-08 | Failed parse always returns `uuid.Nil` | Input token string = `"garbage"` | Returns an error AND the returned UUID is `uuid.Nil` | ✅ Returns error with `uuid.Nil` |

## 3. `internal/platform/middleware/{auth,cors,recover,requestlog}_test.go`

### `auth_test.go`

| ID | Test Case | Prerequisites | Expected | Actual |
|---|---|---|---|---|
| MWAUTH-01 | Missing `Authorization` header is rejected | No `Authorization` header on the request | Next handler never called; response status 401 | ✅ `called=false`, status 401 |
| MWAUTH-02 | Basic-scheme header is rejected | Header = `"Basic dXNlcjpwYXNz"` | Next handler never called; response status 401 | ✅ `called=false`, status 401 |
| MWAUTH-03 | "Bearer" prefix with no space is rejected | Header = `"Bearertoken-with-no-space"` | Next handler never called; response status 401 | ✅ `called=false`, status 401 |
| MWAUTH-04 | Lowercase "bearer" scheme is rejected (case-sensitive check) | Header = `"bearer lowercase-scheme"` | Next handler never called; response status 401 | ✅ `called=false`, status 401 |
| MWAUTH-05 | Syntactically invalid bearer token is rejected | Header = `"Bearer not-a-real-token"` | Next handler never called; response status 401 | ✅ `called=false`, status 401 |
| MWAUTH-06 | Expired bearer token is rejected | Signer with -1 min expiry signs a token, sent as `"Bearer <token>"` | Next handler never called; response status 401 | ✅ `called=false`, status 401 |
| MWAUTH-07 | Valid bearer token sets context and calls next | Signer signs a token for a random `userID`, sent as `"Bearer <token>"` | Next handler called; `UserIDFromContext` on its request context returns the same `userID`; status 200 | ✅ `called=true`, context carries `userID`, status 200 |
| MWAUTH-08 | `UserIDFromContext` on a bare context returns absent | `context.Background()` (no value ever set) | Returns `(uuid.Nil, false)` | ✅ Returns `(uuid.Nil, false)` |
| MWAUTH-09 | `UserIDFromContext` returns the ID set by a real `Auth()` flow | Request context produced by `middleware.Auth` after a valid token | Returns `(userID, true)` matching the signed token's user | ✅ Returns `(userID, true)` |
| MWAUTH-10 | A colliding-but-different context key type is not exposed | Context value set under an unrelated `struct{k string}{"userIDKey"}` key (not the package's real unexported key type) | Returns `(uuid.Nil, false)` | ✅ Returns `(uuid.Nil, false)` — a forged key can't spoof a user ID |

### `cors_test.go`

| ID | Test Case | Prerequisites | Expected | Actual |
|---|---|---|---|---|
| MWCORS-01 | CORS sets headers and calls next on a normal request | Plain GET request, no special headers | `Access-Control-Allow-Origin: *`, `-Methods: GET, POST, PUT, PATCH, DELETE, OPTIONS`, `-Headers: Content-Type, Authorization` all set; next called; status 200 | ✅ All headers set, `called=true`, status 200 |
| MWCORS-02 | OPTIONS preflight short-circuits before reaching next | OPTIONS request to any path | Next handler never called; response status 204; CORS headers still present | ✅ `called=false`, status 204, `Access-Control-Allow-Origin: *` present |

### `recover_test.go`

| ID | Test Case | Prerequisites | Expected | Actual |
|---|---|---|---|---|
| MWRECOVER-01 | A string panic is converted to a 500 JSON error | Next handler panics with `"boom"` (a string value) | `ServeHTTP` does not panic; status 500; JSON body `{"error":"internal server error"}` | ✅ No panic escapes, status 500, correct JSON body |
| MWRECOVER-02 | A panic with an `error` value is also converted to a 500 | Next handler panics with `assert.AnError` (an `error` value) | `ServeHTTP` does not panic; status 500 | ✅ No panic escapes, status 500 |
| MWRECOVER-03 | No panic passes the response through unchanged | Next handler writes status 418 (Teapot), no panic | Response status is exactly 418 | ✅ Status 418 passed through untouched |

### `requestlog_test.go`

| ID | Test Case | Prerequisites | Expected | Actual |
|---|---|---|---|---|
| MWLOG-01 | Status code and body pass through untouched | Next handler writes status 201 and body `"ok"` | Response status 201, body `"ok"`; next handler called | ✅ Status 201, body `"ok"`, `called=true` |
| MWLOG-02 | Handler that never writes a status still reports 200 | Next handler only calls `Write` (body `"implicit 200"`), never `WriteHeader` | Response status defaults to 200 | ✅ Status 200 (chi's `WrapResponseWriter` default) |

## 4. `internal/user/service/auth_service_test.go`

| ID | Test Case | Prerequisites | Expected | Actual |
|---|---|---|---|---|
| USER-01 | Register success hashes password and persists user | Empty fake user repo | Returns a user with non-nil ID, matching username/full name, a `PasswordHash` that differs from the plaintext but verifies via bcrypt; repo's `Create` called with that user | ✅ Returns hashed, persisted user; `bcrypt.CompareHashAndPassword` succeeds |
| USER-02 | Password exactly at the 8-character minimum boundary succeeds | Empty fake repo, password = `"12345678"` (8 chars) | `Register` succeeds, no error | ✅ No error — boundary is inclusive |
| USER-03 | Blank username is rejected | username="", password="password123", fullName="Full Name" | Returns `apperror.KindValidation` | ✅ Returns Validation error |
| USER-04 | Blank password is rejected | username="alice", password="", fullName="Full Name" | Returns `apperror.KindValidation` | ✅ Returns Validation error |
| USER-05 | Blank full name is rejected | username="alice", password="password123", fullName="" | Returns `apperror.KindValidation` | ✅ Returns Validation error |
| USER-06 | All fields blank is rejected | username="", password="", fullName="" | Returns `apperror.KindValidation` | ✅ Returns Validation error |
| USER-07 | Password below the 8-character minimum is rejected | password = `"1234567"` (7 chars) | Returns `apperror.KindValidation` | ✅ Returns Validation error |
| USER-08 | Username already taken returns conflict | Fake repo pre-seeded with an existing `"alice"` user | `Register("alice", ...)` returns `apperror.KindConflict` | ✅ Returns Conflict error |
| USER-09 | Unexpected error checking username availability returns internal error | Fake repo's `FindByUsername` returns `errors.New("connection reset")` (not `gorm.ErrRecordNotFound`) | Returns `apperror.KindInternal`, distinguished from the "available" case | ✅ Returns Internal error |
| USER-10 | Repo `Create` failure returns internal error | Fake repo's `Create` returns `errors.New("write failed")` | Returns `apperror.KindInternal` | ✅ Returns Internal error |
| USER-11 | Whitespace-only username is NOT treated as blank (documented behavior) | username = `"   "` (three spaces), valid password/fullName | `Register` succeeds, no error | ✅ Succeeds — the `== ""` check doesn't catch whitespace-only strings; documents there's no `strings.TrimSpace` today, guarding against a future silent behavior change |
| USER-12 | Login success returns user and a valid token | A user already registered via `Register` | Returns the same user ID and a token that itself parses back to that user ID | ✅ Returns matching user + valid, parseable token |
| USER-13 | Unknown username returns unauthorized, not a distinguishable not-found | Empty fake repo (no such user) | Returns `apperror.KindUnauthorized` | ✅ Returns Unauthorized — same kind as wrong password, avoiding username enumeration |
| USER-14 | Wrong password returns unauthorized | A registered user; login attempted with a different password | Returns `apperror.KindUnauthorized` | ✅ Returns Unauthorized |
| USER-15 | Unexpected repo error during login returns internal error | Fake repo's `FindByUsername` returns `errors.New("connection reset")` | Returns `apperror.KindInternal` | ✅ Returns Internal error |
| USER-16 | Empty-string password never matches any real hash | A registered user; login attempted with password = `""` | Returns `apperror.KindUnauthorized` | ✅ Returns Unauthorized |
| USER-17 | GetByID success | A registered user, looked up by their own ID | Returns that user, no error | ✅ Returns the correct user |
| USER-18 | GetByID not found | A random UUID never registered | Returns `apperror.KindNotFound` | ✅ Returns NotFound |
| USER-19 | GetByID unexpected repo error returns internal error | Fake repo's `FindByID` returns `errors.New("connection reset")` | Returns `apperror.KindInternal` | ✅ Returns Internal error |

## 5. `internal/account/service/account_service_test.go`

| ID | Test Case | Prerequisites | Expected | Actual |
|---|---|---|---|---|
| ACC-01 | ListByUser: success returns only that user's accounts | Two accounts in the fake repo: one for `userID`, one for a different user | Returns exactly the one account belonging to `userID` | ✅ Returns only that user's account |
| ACC-02 | ListByUser: empty result when the user has no accounts | Empty fake repo | Returns an empty slice, no error | ✅ Returns empty slice, no error |
| ACC-03 | ListByUser: repo error returns internal error | Fake repo's `FindByUserID` returns `errors.New("db down")` | Returns `apperror.KindInternal` | ✅ Returns Internal error |
| ACC-04 | GetByID: success | An account belonging to `userID` | Returns that account, no error | ✅ Returns the account |
| ACC-05 | GetByID: not found | Empty fake repo | Returns `apperror.KindNotFound` | ✅ Returns NotFound |
| ACC-06 | GetByID: account belongs to a different user returns not found, not forbidden | An account that exists but is owned by a different user than the requester | Returns `apperror.KindNotFound` (not `Forbidden`) | ✅ Returns NotFound — deliberate anti-enumeration choice |
| ACC-07 | GetByID: repo error returns internal error | Fake repo's `FindByID` returns `errors.New("db down")` | Returns `apperror.KindInternal` | ✅ Returns Internal error |
| ACC-08 | Debit: success reduces balance | Account with balance 100.00 THB | Debiting 40.00 THB returns new balance 60.00 THB, no error | ✅ Returns 60.00 THB, no error |
| ACC-09 | Debit: debiting the exact balance leaves zero, not an error | Account with balance 50.00 THB | Debiting 50.00 THB returns new balance 0.00 THB, no error | ✅ Returns 0.00 THB, no error — zero is not negative |
| ACC-10 | Debit: insufficient funds is rejected | Account with balance 10.00 THB | Debiting 10.01 THB returns `apperror.KindValidation`, balance untouched | ✅ Returns Validation error |
| ACC-11 | Debit: account not found | Empty fake repo | Returns `apperror.KindNotFound` | ✅ Returns NotFound |
| ACC-12 | Debit: lock/lookup error returns internal error | Fake repo's `FindForUpdate` returns `errors.New("lock timeout")` | Returns `apperror.KindInternal` | ✅ Returns Internal error |
| ACC-13 | Debit: update failure returns internal error | Account with balance 100.00 THB; fake repo's `UpdateBalance` returns `errors.New("write failed")` | Returns `apperror.KindInternal` | ✅ Returns Internal error |
| ACC-14 | Credit: success increases balance | Account with balance 100.00 THB | Crediting 25.50 THB returns new balance 125.50 THB, no error | ✅ Returns 125.50 THB, no error |
| ACC-15 | Credit: account not found | Empty fake repo | Returns `apperror.KindNotFound` | ✅ Returns NotFound |
| ACC-16 | Credit: lock/lookup error returns internal error | Fake repo's `FindForUpdate` returns `errors.New("lock timeout")` | Returns `apperror.KindInternal` | ✅ Returns Internal error |
| ACC-17 | Credit: update failure returns internal error | Account with balance 100.00 THB; fake repo's `UpdateBalance` returns `errors.New("write failed")` | Returns `apperror.KindInternal` | ✅ Returns Internal error |
| ACC-18 | Credit: crediting zero amount is a no-op success | Account with balance 100.00 THB | Crediting 0 THB returns unchanged balance 100.00 THB, no error | ✅ Returns 100.00 THB, no error |

## 6. `internal/salak/domain/{holding,product}_test.go`

Pure formatting/struct logic — no service or repo involved.

| ID | Test Case | Prerequisites | Expected | Actual |
|---|---|---|---|---|
| HOLDDOM-01 | TicketStartID: typical mid-range number | `Holding{TicketLetter:"ก", TicketStart:7530}` | `TicketStartID()` returns `"ก0007530"` | ✅ Returns `"ก0007530"` |
| HOLDDOM-02 | TicketStartID: zero pads to full width | `Holding{TicketLetter:"ก", TicketStart:0}` | Returns `"ก0000000"` | ✅ Returns `"ก0000000"` |
| HOLDDOM-03 | TicketStartID: exactly 7 digits fills the field with no padding | `Holding{TicketLetter:"ข", TicketStart:1234567}` | Returns `"ข1234567"` | ✅ Returns `"ข1234567"` |
| HOLDDOM-04 | TicketStartID: more than 7 digits is not truncated | `Holding{TicketLetter:"ฮ", TicketStart:123456789}` | Returns `"ฮ123456789"` (9 digits, not clipped to 7) | ✅ Returns `"ฮ123456789"` — real edge case once the ticket sequence grows past 9,999,999 |
| HOLDDOM-05 | TicketStartID: single digit | `Holding{TicketLetter:"ค", TicketStart:5}` | Returns `"ค0000005"` | ✅ Returns `"ค0000005"` |
| HOLDDOM-06 | TicketEndID formats the same way as TicketStartID | `Holding{TicketLetter:"ก", TicketEnd:42}` | `TicketEndID()` returns `"ก0000042"` | ✅ Returns `"ก0000042"` |
| HOLDDOM-07 | Start and end IDs share the same ticket letter | `Holding{TicketLetter:"ง", TicketStart:100, TicketEnd:105}` | `TicketStartID()`="ง0000100", `TicketEndID()`="ง0000105" | ✅ Both match, same letter |
| HOLDDOM-08 | Holding.TableName() | `Holding{}` (zero value) | Returns `"salak.holdings"` | ✅ Returns `"salak.holdings"` |
| HOLDDOM-09 | TicketSequence.TableName() | `TicketSequence{}` (zero value) | Returns `"salak.ticket_sequence"` | ✅ Returns `"salak.ticket_sequence"` |
| PRODDOM-01 | Product.TableName() | `Product{}` (zero value) | Returns `"salak.products"` | ✅ Returns `"salak.products"` |

## 7. `internal/salak/service/salak_service_test.go`

| ID | Test Case | Prerequisites | Expected | Actual |
|---|---|---|---|---|
| SALAK-01 | ListProducts: success | Fake product repo seeded with one active product | Returns a slice containing that one product | ✅ Returns the product |
| SALAK-02 | ListProducts: repo error returns internal error | Fake repo's `ListActive` returns `errors.New("db down")` | Returns `apperror.KindInternal` | ✅ Returns Internal error |
| SALAK-03 | GetProduct: success | Fake repo seeded with one active product | Returns that product, no error | ✅ Returns the product |
| SALAK-04 | GetProduct: not found | Empty fake repo | Returns `apperror.KindNotFound` | ✅ Returns NotFound |
| SALAK-05 | GetProduct: repo error returns internal error | Fake repo's `FindByID` returns `errors.New("db down")` | Returns `apperror.KindInternal` | ✅ Returns Internal error |
| SALAK-06 | GetProduct: inactive product is not purchasable | A product that exists but has `IsActive=false` | Returns `apperror.KindValidation` (not NotFound) | ✅ Returns Validation error even though the product exists |
| SALAK-07 | ValidatePurchase: zero amount rejected | Product with min=100, max=1,000,000, step=100 | `ValidatePurchase(product, 0)` returns `apperror.KindValidation` | ✅ Returns Validation error |
| SALAK-08 | ValidatePurchase: negative amount rejected | Same product | Amount = -100 returns `apperror.KindValidation` | ✅ Returns Validation error |
| SALAK-09 | ValidatePurchase: below minimum rejected | Same product | Amount = 50 (< min 100) returns `apperror.KindValidation` | ✅ Returns Validation error |
| SALAK-10 | ValidatePurchase: above maximum rejected | Same product | Amount = 1,000,001 (> max 1,000,000) returns `apperror.KindValidation` | ✅ Returns Validation error |
| SALAK-11 | ValidatePurchase: not a multiple of step rejected | Same product (step=100) | Amount = 150 returns `apperror.KindValidation` | ✅ Returns Validation error |
| SALAK-12 | ValidatePurchase: exactly at minimum boundary is valid | Same product | Amount = 100 (== min) returns no error | ✅ No error — boundary is inclusive (`LessThan`, not `<=`) |
| SALAK-13 | ValidatePurchase: exactly at maximum boundary is valid | Same product | Amount = 1,000,000 (== max) returns no error | ✅ No error — boundary is inclusive (`GreaterThan`, not `>=`) |
| SALAK-14 | ValidatePurchase: valid multiple of step in range | Same product | Amount = 500 returns no error | ✅ No error |
| SALAK-15 | MintHolding: success computes units, ticket range, and maturity date | Product with unit price 100, term 3 months; fake holding repo configured to reserve range [1000,1004] | Amount 500 → `Units=5`, reserved range flows through unchanged into the holding, `MaturityDate = PurchaseDate + 3 months`, `TicketLetter` is a single valid Thai consonant rune in `[0x0E01,0x0E2E]` | ✅ All fields match; ticket letter verified numerically against the exact rune range the production code draws from (not by hand-copying all 46 characters) |
| SALAK-16 | MintHolding: amount truncates to whole units when not an exact multiple | Product with unit price 100 | Amount 250 → `Units=2` (250/100 truncated down, not rounded or rejected) | ✅ Returns `Units=2` — documents current behavior; full-amount validation happens earlier in `ValidatePurchase`, not here |
| SALAK-17 | MintHolding: amount below one unit price is rejected | Product with unit price 100 | Amount 50 → `apperror.KindValidation` (0 whole units) | ✅ Returns Validation error |
| SALAK-18 | MintHolding: zero amount is rejected | Same product | Amount 0 → `apperror.KindValidation` | ✅ Returns Validation error |
| SALAK-19 | MintHolding: product not found | Empty fake product repo | Returns `apperror.KindNotFound` | ✅ Returns NotFound |
| SALAK-20 | MintHolding: product lookup error returns internal error | Fake repo's `FindByID` returns `errors.New("db down")` | Returns `apperror.KindInternal` | ✅ Returns Internal error |
| SALAK-21 | MintHolding: ticket range reservation failure returns internal error | Fake holding repo's `ReserveTicketRange` returns `errors.New("lock timeout")` | Returns `apperror.KindInternal` | ✅ Returns Internal error |
| SALAK-22 | MintHolding: holding create failure returns internal error | Fake holding repo's `Create` returns `errors.New("write failed")` | Returns `apperror.KindInternal` | ✅ Returns Internal error |
| SALAK-23 | ListHoldingsByAccount: success after ownership check passes | Fake `account.Service.GetByID` returns an account owned by `userID`; fake holding repo has one holding for that account | Returns that one holding; confirms the correct `userID`/`accountID` were forwarded to `account.Service.GetByID` | ✅ Returns the holding, correct args forwarded |
| SALAK-24 | ListHoldingsByAccount: ownership check failure is propagated verbatim | Fake `account.Service.GetByID` returns `apperror.NotFound("account not found")` | Returns that exact `apperror.KindNotFound`, not rewrapped | ✅ Returns NotFound, propagated verbatim |
| SALAK-25 | ListHoldingsByAccount: holding repo error returns internal error | Fake holding repo's `FindByAccountID` returns `errors.New("db down")` | Returns `apperror.KindInternal` | ✅ Returns Internal error |
| SALAK-26 | ListHoldingsByAccount: no holdings returns an empty slice, not an error | Ownership check passes; fake holding repo has no holdings for the account | Returns an empty slice, no error | ✅ Returns empty slice, no error |

## 8. `internal/transaction/service/buy_salak_service_test.go`

| ID | Test Case | Prerequisites | Expected | Actual |
|---|---|---|---|---|
| TX-01 | BuySalak: funding and salak account must differ | `fundingAccountID == salakAccountID` (same ID passed for both) | Returns `apperror.KindValidation`, no DB transaction ever opened | ✅ Returns Validation error |
| TX-02 | BuySalak: funding account lookup failure is propagated verbatim | Fake `account.Service.GetByID` for the funding account returns `apperror.NotFound(...)` | Returns that exact NotFound error, not rewrapped | ✅ Returns NotFound, propagated verbatim |
| TX-03 | BuySalak: funding account must be savings-type | Funding account exists but has `Type=salak` (wrong type on purpose) | Returns `apperror.KindValidation` | ✅ Returns Validation error |
| TX-04 | BuySalak: salak account lookup failure is propagated verbatim | Funding account valid; fake `GetByID` for the salak account returns `apperror.NotFound(...)` | Returns that exact NotFound error | ✅ Returns NotFound, propagated verbatim |
| TX-05 | BuySalak: salak account must be salak-type | Both accounts exist, but the "salak" account has `Type=savings` (wrong type on purpose) | Returns `apperror.KindValidation` | ✅ Returns Validation error |
| TX-06 | BuySalak: product lookup failure is propagated verbatim | Both accounts valid; fake `salak.Service.GetProduct` returns `apperror.NotFound(...)` | Returns that exact NotFound error | ✅ Returns NotFound, propagated verbatim |
| TX-07 | BuySalak: purchase validation failure is propagated verbatim | Accounts + product valid; fake `salak.Service.ValidatePurchase` returns `apperror.Validation("amount must be a multiple of the step amount")` | Returns that exact Validation error | ✅ Returns Validation error, propagated verbatim |
| TX-08 | BuySalak: success commits the transaction and returns a full receipt | Valid funding/salak accounts, valid product, no badge supplied (`badgeID=nil`); `sqlmock`-backed `*gorm.DB` with `ExpectBegin()`/`ExpectCommit()` | Returns a receipt with correct product name/units/amount/both post-transaction balances/non-nil reference ID; exactly 2 ledger entries (debit+credit) sharing one `reference_id`, both carrying the minted holding's ID; fake `badge.Service.UserOwnsBadge` is never called since no badge was supplied; `mock.ExpectationsWereMet()` passes | ✅ All receipt fields correct; ledger pairing verified; `badgeSvc.called` confirmed false; transaction genuinely committed (mock expectations met) |
| TX-09 | BuySalak: badge supplied and owned succeeds | Funding/salak accounts + product valid; a `badgeID` is supplied; fake `badge.Service.UserOwnsBadge` returns `owns: true`; `sqlmock`-backed `*gorm.DB` with `ExpectBegin()`/`ExpectCommit()` | Purchase succeeds exactly like the no-badge case; fake `badge.Service.UserOwnsBadge` was called; `mock.ExpectationsWereMet()` passes | ✅ Succeeds, `badgeSvc.called` is true, `ExpectationsWereMet()` confirms commit |
| TX-10 | BuySalak: badge supplied but not owned is rejected before any transaction opens | A `badgeID` is supplied; fake `badge.Service.UserOwnsBadge` returns `owns: false`; `db` passed as `nil` (no `sqlmock` expectations needed) | Returns `apperror.KindForbidden`; no DB transaction is ever opened | ✅ Returns Forbidden error |
| TX-11 | BuySalak: badge ownership check error returns internal error | A `badgeID` is supplied; fake `badge.Service.UserOwnsBadge` returns `errors.New("db down")` | Returns `apperror.KindInternal` | ✅ Returns Internal error |
| TX-12 | BuySalak: debit failure rolls back and is propagated verbatim | Fake `account.Service.Debit` returns `apperror.Validation("insufficient funds")`; `sqlmock` expects `Begin()`→`Rollback()` | Returns that exact Validation error; mock confirms rollback occurred | ✅ Returns Validation error; `ExpectationsWereMet()` confirms rollback |
| TX-13 | BuySalak: mint holding failure rolls back and is propagated verbatim | Fake `salak.Service.MintHolding` returns an Internal error (e.g. simulated ticket-lock timeout); `sqlmock` expects `Begin()`→`Rollback()` | Returns that Internal error; mock confirms rollback | ✅ Returns Internal error; rollback confirmed |
| TX-14 | BuySalak: credit failure rolls back and is propagated verbatim | Fake `account.Service.Credit` returns a plain error; `sqlmock` expects `Begin()`→`Rollback()` | Returns an error; mock confirms rollback | ✅ Returns error; rollback confirmed |
| TX-15 | BuySalak: ledger write failure rolls back the whole transaction | Fake `LedgerRepository.Create` returns a plain error (disk full); `sqlmock` expects `Begin()`→`Rollback()` | Returns an error; mock confirms the whole debit→mint→credit chain rolled back, not just the ledger write | ✅ Returns error; rollback confirmed even though 3 prior steps had already "succeeded" against the fakes |
| TX-16 | ListHistory: success | Fake `account.Service.GetByID` succeeds; fake ledger repo returns a fixed list of entries | Returns that exact list of entries | ✅ Returns the entries |
| TX-17 | ListHistory: ownership check failure is propagated verbatim | Fake `account.Service.GetByID` returns `apperror.NotFound(...)` | Returns that exact NotFound error | ✅ Returns NotFound, propagated verbatim |
| TX-18 | ListHistory: non-positive limit defaults to 20 | `limit` passed as 0, -1, or -100 | Ledger repo is called with `limit=20` in every case | ✅ Repo called with `limit=20` for all three inputs |
| TX-19 | ListHistory: limit above 100 is clamped to 100 | `limit=500` | Ledger repo is called with `limit=100` | ✅ Repo called with `limit=100` |
| TX-20 | ListHistory: limit exactly at the 100 boundary is not clamped | `limit=100` | Ledger repo is called with `limit=100` (unchanged) | ✅ Repo called with `limit=100` — off-by-one guard confirmed |
| TX-21 | ListHistory: negative offset is clamped to 0 | `offset=-5` | Ledger repo is called with `offset=0` | ✅ Repo called with `offset=0` |
| TX-22 | ListHistory: repo error returns internal error | Fake ledger repo's `FindByAccountID` returns `errors.New("db down")` | Returns `apperror.KindInternal` | ✅ Returns Internal error |

## 9. `internal/chooser/chooser_test.go`

| ID | Test Case | Prerequisites | Expected | Actual |
|---|---|---|---|---|
| CHOOSER-01 | `NewChooser`: empty weights slice is rejected | `chooser.NewChooser(nil)` | Returns an error | ✅ Returns error |
| CHOOSER-02 | `NewChooser`: a negative weight is rejected | `chooser.NewChooser([]float64{1, -0.5, 2})` | Returns an error | ✅ Returns error |
| CHOOSER-03 | `NewChooser`: all-zero weights are rejected | `chooser.NewChooser([]float64{0, 0, 0})` | Returns an error | ✅ Returns error |
| CHOOSER-04 | `Pick`: a single-weight chooser always picks index 0 | One weight (`{5}`); `rand.New(rand.NewPCG(1, 1))`; 100 draws | Every draw returns index `0` | ✅ All 100 draws return index 0 |
| CHOOSER-05 | `Pick`: a zero-weight entry is never picked | Weights `{0, 1}` (valid since total > 0); `rand.New(rand.NewPCG(2, 2))`; 1000 draws | Every draw returns index `1`, never index `0` | ✅ All 1000 draws return index 1 |
| CHOOSER-06 | `Pick`: distribution matches weights within tolerance | Weights `{0.5, 0.3, 0.2}`; `rand.New(rand.NewPCG(3, 3))`; 10,000 draws | Each index's draw count falls within ±5% of its weight's expected share of 10,000 | ✅ All three counts fall within tolerance |

## 10. `internal/badge/service/random_badge_service_test.go`

| ID | Test Case | Prerequisites | Expected | Actual |
|---|---|---|---|---|
| BADGESVC-01 | `NewWeightedRandomBadgeService`: empty badges slice is rejected | `service.NewWeightedRandomBadgeService(rand.New(rand.NewPCG(0, 0)), nil)` | Returns an error | ✅ Returns error |
| BADGESVC-02 | `NewWeightedRandomBadgeService`: all-zero-weight badges are rejected | Two badges, both `Weight: 0` | Returns an error | ✅ Returns error |
| BADGESVC-03 | `NewWeightedRandomBadgeService`: a negative-weight badge is rejected | Two badges, one with `Weight: -1` | Returns an error | ✅ Returns error |
| BADGESVC-04 | `GetRandomBadge`: valid badge | Service constructed with 3 fixed badges (weights 0.5/0.3/0.2) | Returns one of the 3 configured badges, no error | ✅ Returns a badge contained in the configured set |
| BADGESVC-05 | `GetRandomBadge`: distribution matches weights within tolerance | Same 3-badge/weight setup; 10,000 draws | Each badge's draw count falls within ±5% of its weight's expected share of 10,000 | ✅ All three counts fall within tolerance |
