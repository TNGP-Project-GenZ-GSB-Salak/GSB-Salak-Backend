const { test, expect } = require("@playwright/test");
const { createShooter } = require("./helpers/screenshot");
const { loginAsDemo } = require("./helpers/auth");

test.describe("kapook terms", () => {
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
