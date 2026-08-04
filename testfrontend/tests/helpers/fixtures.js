// Fixed IDs from cmd/seed/main.go's deterministic demo data.
//
// PINNED_TEST_DATE is the date (safe for both real products' draw-day
// calendar - not the 1-year product's 16th, not the 2-year product's 1st/2nd)
// that FIXED_CLOCK_RFC3339 pins the whole testfrontend server's business
// clock to (see ../../playwright.config.js). Without this, every buy-salak
// spec would become flaky on the real calendar's draw days once
// cmd/seed populates salak.draw_dates for the real products. The one spec
// that DOES want to hit a draw day (draw-day-guard.spec.js) inserts its own
// one-off row for this exact date instead of relying on the real calendar.
module.exports = {
  DEMO_USERNAME: "demo",
  DEMO_PASSWORD: "demopass123",
  SAVINGS_ACCOUNT_ID: "22222222-2222-2222-2222-222222222222",
  SAVINGS_ACCOUNT_NUMBER: "1234009012",
  SALAK_ACCOUNT_ID: "33333333-3333-3333-3333-333333333333",
  SALAK_ACCOUNT_NUMBER: "4001000111",
  KAPOOK_ACCOUNT_ID: "44444444-4444-4444-4444-444444444444",
  KAPOOK_ACCOUNT_NUMBER: "5001000111",
  PINNED_TEST_DATE: "2026-03-10",
  FIXED_CLOCK_RFC3339: "2026-03-10T00:00:00Z",
};
