const { defineConfig, devices } = require("@playwright/test");
const { KAPOOK_COUNTDOWN_DURATION } = require("./tests/helpers/fixtures");

module.exports = defineConfig({
  testDir: "./tests",
  globalSetup: require.resolve("./globalSetup.js"),
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [["html", { outputFolder: "playwright-report", open: "never" }], ["list"]],
  use: {
    baseURL: "http://localhost:5173",
    trace: "retain-on-failure",
  },
  webServer: [
    {
      command: "node server.js",
      port: 5173,
      reuseExistingServer: !process.env.CI,
    },
    {
      command: "go run ./cmd/api",
      cwd: "..",
      port: 8080,
      // The server runs on the real wall clock - buy-salak specs are kept
      // safe from a real-calendar draw day by globalSetup.js clearing
      // salak.draw_dates before the suite runs, not by pinning the clock.
      // See fixtures.js for why a settable clock isn't used here. Note:
      // with reuseExistingServer, an already-running `go run ./cmd/api`
      // from outside this suite won't pick up KAPOOK_COUNTDOWN_DURATION.
      env: { ...process.env, KAPOOK_COUNTDOWN_DURATION },
      reuseExistingServer: !process.env.CI,
      timeout: 30_000,
    },
    {
      // The Kapook auto-purchase worker. It has no HTTP port of its own,
      // so there is no port/url for Playwright to wait on - omitting both
      // is deliberate, not an oversight; Playwright just spawns it and
      // moves on. Omitting this entry entirely is the exact silent
      // failure the worker package's own docs warn about: the countdown
      // spec's goal would simply never get bought, with every assertion
      // up to that point passing.
      command: "go run ./cmd/worker",
      cwd: "..",
      env: { ...process.env, KAPOOK_COUNTDOWN_DURATION },
      reuseExistingServer: !process.env.CI,
      timeout: 30_000,
    },
  ],
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
