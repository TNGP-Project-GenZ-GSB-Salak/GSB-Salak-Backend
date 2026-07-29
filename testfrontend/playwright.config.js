const { defineConfig, devices } = require("@playwright/test");

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
      reuseExistingServer: !process.env.CI,
      timeout: 30_000,
    },
  ],
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
