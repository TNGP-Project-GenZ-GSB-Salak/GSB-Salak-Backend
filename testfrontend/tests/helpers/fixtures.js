// Fixed IDs from cmd/seed/main.go's deterministic demo data.
//
// There is no pinned business-clock date here on purpose: the server
// always runs on the real wall clock (internal/platform/clock.Real).
// globalSetup.js instead clears salak.draw_dates before the suite runs, so
// no spec other than draw-day-guard.spec.js ever sees a draw day - that
// one spec inserts its own one-off row for the real "today" and cleans it
// up immediately after. A settable business clock was tried and reverted:
// it pinned draw-day testing correctly, but the same clock also drives
// ledger timestamps, maturity dates, and (once the Kapook countdown
// existed) the withdrawal fee window and GoalReachedAt - one env var was
// moving all of those at once, which is exactly the failure mode
// .scratch/kapook-goal-saving/spec.md's "The scheduler" section rejected a
// settable debug clock for.
module.exports = {
  DEMO_USERNAME: "demo",
  DEMO_PASSWORD: "demopass123",
  SAVINGS_ACCOUNT_ID: "22222222-2222-2222-2222-222222222222",
  SAVINGS_ACCOUNT_NUMBER: "1234009012",
  SALAK_ACCOUNT_ID: "33333333-3333-3333-3333-333333333333",
  SALAK_ACCOUNT_NUMBER: "4001000111",
  KAPOOK_ACCOUNT_ID: "44444444-4444-4444-4444-444444444444",
  KAPOOK_ACCOUNT_NUMBER: "5001000111",
  // Short enough to observe within a test, long enough to assert the
  // "counting down, not yet purchased" state before it expires. The
  // worker's own poll tick is a fixed ~1 minute (not configurable - see
  // cmd/worker), so a countdown spec's worst-case wait is this plus one
  // tick, not just this alone.
  KAPOOK_COUNTDOWN_DURATION: "10s",
};
