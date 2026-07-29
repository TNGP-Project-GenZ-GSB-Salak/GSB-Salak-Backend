const { test, expect } = require("@playwright/test");
const { createShooter } = require("./helpers/screenshot");
const { DEMO_USERNAME, DEMO_PASSWORD } = require("./helpers/fixtures");

test.describe("login", () => {
  test("success: demo user logs in and reaches the dashboard", async ({ page }) => {
    const shoot = createShooter("login", "success");

    await page.goto("/login.html");
    await shoot(page, "form-empty");

    await page.getByTestId("username-input").fill(DEMO_USERNAME);
    await page.getByTestId("password-input").fill(DEMO_PASSWORD);
    await shoot(page, "form-filled");

    await page.getByTestId("submit-button").click();
    await page.waitForURL("**/dashboard.html");
    await shoot(page, "dashboard-loaded");

    expect(await page.evaluate(() => localStorage.getItem("token"))).toBeTruthy();
  });

  test("wrong password is rejected", async ({ page }) => {
    const shoot = createShooter("login", "wrong-password");

    await page.goto("/login.html");
    await page.getByTestId("username-input").fill(DEMO_USERNAME);
    await page.getByTestId("password-input").fill("totally-wrong-password");
    await shoot(page, "form-filled");

    await page.getByTestId("submit-button").click();

    const message = page.getByTestId("message");
    await expect(message).toHaveText(/invalid username or password/i);
    await shoot(page, "error-message");

    expect(page.url()).toContain("login.html");
  });

  test("unknown username gives the same generic error (no user enumeration)", async ({ page }) => {
    const shoot = createShooter("login", "unknown-user");

    await page.goto("/login.html");
    await page.getByTestId("username-input").fill("does-not-exist-user");
    await page.getByTestId("password-input").fill("whatever123");
    await shoot(page, "form-filled");

    await page.getByTestId("submit-button").click();

    const message = page.getByTestId("message");
    await expect(message).toHaveText(/invalid username or password/i);
    await shoot(page, "error-message");
  });
});
