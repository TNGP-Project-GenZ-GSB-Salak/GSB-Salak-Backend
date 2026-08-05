const { test, expect } = require("@playwright/test");
const { createShooter } = require("./helpers/screenshot");
const { loginAsDemo } = require("./helpers/auth");
const { readBalance } = require("./helpers/dashboard");
const { SAVINGS_ACCOUNT_ID, SAVINGS_ACCOUNT_NUMBER, SALAK_ACCOUNT_ID } = require("./helpers/fixtures");

test.describe("transaction history", () => {
  test("shows the debit entry for a completed purchase", async ({ page }) => {
    const shoot = createShooter("transactions", "shows-purchase-entry");

    await loginAsDemo(page);
    const before = await readBalance(page, SAVINGS_ACCOUNT_NUMBER);

    await page.getByTestId("funding-account-select").selectOption(SAVINGS_ACCOUNT_ID);
    await page.getByTestId("salak-account-select").selectOption(SALAK_ACCOUNT_ID);
    await page.getByTestId("product-select").selectOption({ label: "สลากดิจิทัล 2 ปี" });
    await page.getByTestId("amount-select").selectOption("1000");
    await page.getByTestId("buy-submit").click();
    await expect(page.getByTestId("receipt")).toBeVisible();
    await shoot(page, "purchase-made");

    await page.goto(`/transactions.html?account_id=${SAVINGS_ACCOUNT_ID}`);
    await shoot(page, "history-loaded");

    const rows = page.getByTestId("transaction-row");
    await expect(rows.first()).toBeVisible();

    const types = await page.getByTestId("transaction-type").allTextContents();
    const amounts = await page.getByTestId("transaction-amount").allTextContents();
    const balancesAfter = await page.getByTestId("transaction-balance-after").allTextContents();

    expect(types).toContain("debit");
    expect(amounts).toContain("1000");
    expect(balancesAfter).toContain(String(before - 1000));

    await shoot(page, "verified");
  });

  test("missing account_id query param shows an error instead of a blank page", async ({ page }) => {
    const shoot = createShooter("transactions", "missing-account-id");

    await loginAsDemo(page);
    await page.goto("/transactions.html");
    await shoot(page, "page-loaded");

    const message = page.getByTestId("message");
    await expect(message).toHaveText(/missing account_id/i);
    await shoot(page, "error-shown");
  });
});
