const { test, expect } = require("@playwright/test");
const { createShooter } = require("./helpers/screenshot");
const { loginAsDemo } = require("./helpers/auth");
const { SAVINGS_ACCOUNT_NUMBER, SALAK_ACCOUNT_NUMBER } = require("./helpers/fixtures");

test.describe("dashboard", () => {
  test("renders accounts and salak products after login", async ({ page }) => {
    const shoot = createShooter("dashboard", "accounts-and-products-render");

    await loginAsDemo(page);
    await shoot(page, "dashboard-loaded");

    await expect(page.getByTestId("account-row")).toHaveCount(2);
    const accountNumbers = await page.getByTestId("account-number").allTextContents();
    expect(accountNumbers).toContain(SAVINGS_ACCOUNT_NUMBER);
    expect(accountNumbers).toContain(SALAK_ACCOUNT_NUMBER);

    await expect(page.getByTestId("product-row")).toHaveCount(2);
    const productNames = await page.getByTestId("product-name").allTextContents();
    expect(productNames).toContain("Digital Salak 1-Year");
    expect(productNames).toContain("Digital Salak 2-Year");

    await shoot(page, "verified");
  });

  test("view history link navigates to that account's transaction page", async ({ page }) => {
    const shoot = createShooter("dashboard", "view-history-navigation");

    await loginAsDemo(page);

    const savingsRow = page.getByTestId("account-row").filter({ hasText: SAVINGS_ACCOUNT_NUMBER });
    await shoot(page, "dashboard-loaded");

    await savingsRow.getByTestId("account-history-link").click();
    await page.waitForURL(/transactions\.html\?account_id=/);
    await shoot(page, "transactions-page-loaded");

    expect(page.url()).toContain("account_id=");
  });

  test("unauthenticated visitor is redirected to login", async ({ page }) => {
    const shoot = createShooter("dashboard", "unauthenticated-redirect");

    await page.goto("/dashboard.html");
    await page.waitForURL("**/login.html");
    await shoot(page, "redirected-to-login");
  });

  test("logout clears the session and redirects to login", async ({ page }) => {
    const shoot = createShooter("dashboard", "logout");

    await loginAsDemo(page);
    await shoot(page, "dashboard-loaded");

    await page.getByTestId("logout-button").click();
    await page.waitForURL("**/login.html");
    await shoot(page, "redirected-to-login");

    expect(await page.evaluate(() => localStorage.getItem("token"))).toBeNull();
  });
});
