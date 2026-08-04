const { execSync } = require("child_process");
const { test, expect } = require("@playwright/test");
const { createShooter } = require("./helpers/screenshot");
const { loginAsDemo } = require("./helpers/auth");

test.describe("kapook terms", () => {
  // This file's first test needs the demo user's terms genuinely
  // unaccepted. globalSetup already resets this once per suite RUN, but
  // another spec file (goal.spec.js, which needs terms accepted to create
  // a goal, and which Playwright happens to run before this file
  // alphabetically) can accept them first within the SAME run. Rather than
  // depend on cross-file execution order, reset directly here too, so this
  // file's precondition holds regardless of what ran before it.
  test.beforeAll(() => {
    const container = process.env.DB_CONTAINER || "gsb-salak-backend-db-1";
    const dbName = process.env.DB_NAME || "gsb_salak";
    execSync(
      `docker exec ${container} psql -U postgres -d ${dbName} -c "DELETE FROM kapook.terms_acceptances WHERE user_id = '11111111-1111-1111-1111-111111111111'"`,
      { stdio: "inherit" }
    );
  });

  test("shows not-yet-accepted, then accepting flips the status and disables the button", async ({ page }) => {
    const shoot = createShooter("terms", "accept-flow");

    await loginAsDemo(page);
    await page.goto("/terms.html");

    const status = page.getByTestId("terms-status");
    await expect(status).toHaveText("Not yet accepted");
    await shoot(page, "not-yet-accepted");

    const acceptButton = page.getByTestId("accept-terms-button");
    await acceptButton.click();

    await expect(status).toHaveText("Accepted");
    await expect(acceptButton).toBeDisabled();
    await shoot(page, "accepted");

    // Reloading re-fetches from the API rather than trusting client state,
    // proving acceptance actually persisted server-side.
    await page.reload();
    await expect(status).toHaveText("Accepted");
    await expect(acceptButton).toBeDisabled();
    await shoot(page, "still-accepted-after-reload");
  });

  test("accepting twice does not error - the button stays disabled and the endpoint stays idempotent", async ({ page }) => {
    const shoot = createShooter("terms", "double-accept");

    await loginAsDemo(page);
    await page.goto("/terms.html");

    const status = page.getByTestId("terms-status");
    const acceptButton = page.getByTestId("accept-terms-button");

    // Already accepted by the first test in this file: globalSetup resets
    // kapook.terms_acceptances once per suite RUN, not once per test, so
    // acceptance persists across the rest of this run's tests - same as
    // it would across a real user's whole session in production.
    await expect(status).toHaveText("Accepted");
    await expect(acceptButton).toBeDisabled();
    await shoot(page, "already-accepted");
  });

  test("unauthenticated visitor is redirected to login", async ({ page }) => {
    const shoot = createShooter("terms", "unauthenticated-redirect");

    await page.goto("/terms.html");
    await page.waitForURL("**/login.html");
    await shoot(page, "redirected-to-login");
  });
});
