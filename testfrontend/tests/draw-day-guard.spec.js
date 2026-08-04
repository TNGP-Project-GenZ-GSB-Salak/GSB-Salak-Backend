const { test, expect } = require("@playwright/test");
const { createShooter } = require("./helpers/screenshot");
const { loginAsDemo } = require("./helpers/auth");
const { SAVINGS_ACCOUNT_ID, SALAK_ACCOUNT_ID, PINNED_TEST_DATE } = require("./helpers/fixtures");
const { execSql, queryOne } = require("./helpers/db");

// The whole testfrontend server's business clock is pinned to
// PINNED_TEST_DATE (see playwright.config.js) precisely so every OTHER spec
// never lands on a real-calendar draw day by accident. That means
// demonstrating a REJECTED draw-day purchase needs its own one-off
// draw_dates row for that exact pinned date, added just for this test and
// removed immediately after so it can't affect any other spec's purchase of
// the same product.
test.describe("draw-day purchase guard", () => {
  test("a purchase on the product's draw day is rejected", async ({ page }) => {
    const shoot = createShooter("draw-day-guard", "rejected");

    const productId = queryOne(`SELECT id FROM salak.products WHERE code = 'SALAK_2Y'`);
    execSql(
      `INSERT INTO salak.draw_dates (product_id, draw_date) VALUES ('${productId}', '${PINNED_TEST_DATE}') ON CONFLICT DO NOTHING`
    );

    try {
      await loginAsDemo(page);
      await page.getByTestId("funding-account-select").selectOption(SAVINGS_ACCOUNT_ID);
      await page.getByTestId("salak-account-select").selectOption(SALAK_ACCOUNT_ID);
      await page.getByTestId("product-select").selectOption({ label: "Digital Salak 2-Year" });
      await page.getByTestId("amount-select").selectOption("1000");
      await shoot(page, "form-filled");

      await page.getByTestId("buy-submit").click();

      const error = page.getByTestId("buy-error");
      await expect(error).toHaveText(/draw day/i);
      await expect(page.getByTestId("receipt")).toBeHidden();
      await shoot(page, "error-shown");
    } finally {
      execSql(
        `DELETE FROM salak.draw_dates WHERE product_id = '${productId}' AND draw_date = '${PINNED_TEST_DATE}'`
      );
    }
  });
});
