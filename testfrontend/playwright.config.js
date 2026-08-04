const { defineConfig, devices } = require("@playwright/test");
const { FIXED_CLOCK_RFC3339 } = require("./tests/helpers/fixtures");

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
      // Pins the server's business clock (purchase/maturity dates, the
      // draw-day guard) away from the real calendar, so buy-salak specs
      // never flake on the 16th/1st/2nd once cmd/seed populates real
      // draw dates for the demo products. See fixtures.js for the date's
      // rationale. Note: with reuseExistingServer, an already-running
      // `go run ./cmd/api` from outside this suite won't pick this up.
      env: { ...process.env, FIXED_CLOCK_RFC3339 },
      reuseExistingServer: !process.env.CI,
      timeout: 30_000,
    },
  ],
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
