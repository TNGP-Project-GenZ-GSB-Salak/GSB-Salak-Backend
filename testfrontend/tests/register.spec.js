const { test, expect } = require("@playwright/test");
const { createShooter } = require("./helpers/screenshot");

function uniqueUsername() {
  return `user_${Date.now()}_${Math.floor(Math.random() * 100000)}`;
}

test.describe("register", () => {
  test("success: a new user can register", async ({ page }) => {
    const shoot = createShooter("register", "success");
    const username = uniqueUsername();

    await page.goto("/register.html");
    await shoot(page, "form-empty");

    await page.getByTestId("username-input").fill(username);
    await page.getByTestId("password-input").fill("a-strong-password");
    await page.getByTestId("full-name-input").fill("Test User");
    await shoot(page, "form-filled");

    await page.getByTestId("submit-button").click();

    const message = page.getByTestId("message");
    await expect(message).toHaveText(/Registration successful/i);
    await shoot(page, "success-message");
  });

  test("duplicate username is rejected", async ({ page }) => {
    const shoot = createShooter("register", "duplicate-username");

    await page.goto("/register.html");
    await page.getByTestId("username-input").fill("demo");
    await page.getByTestId("password-input").fill("whatever123");
    await page.getByTestId("full-name-input").fill("Duplicate Demo");
    await shoot(page, "form-filled");

    await page.getByTestId("submit-button").click();

    const message = page.getByTestId("message");
    await expect(message).toHaveText(/already taken/i);
    await shoot(page, "error-message");
  });

  test("password shorter than 8 characters is rejected", async ({ page }) => {
    const shoot = createShooter("register", "short-password");
    const username = uniqueUsername();

    await page.goto("/register.html");
    await page.getByTestId("username-input").fill(username);
    await page.getByTestId("password-input").fill("short");
    await page.getByTestId("full-name-input").fill("Short Password");
    await shoot(page, "form-filled");

    await page.getByTestId("submit-button").click();

    const message = page.getByTestId("message");
    await expect(message).toHaveText(/at least 8 characters/i);
    await shoot(page, "error-message");
  });
});
