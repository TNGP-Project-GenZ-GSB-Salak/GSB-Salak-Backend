const { test, expect } = require("@playwright/test");
const { createShooter } = require("./helpers/screenshot");
const { loginAsDemo } = require("./helpers/auth");
const { readBalance } = require("./helpers/dashboard");
const {
  SAVINGS_ACCOUNT_ID,
  SAVINGS_ACCOUNT_NUMBER,
  SALAK_ACCOUNT_ID,
  SALAK_ACCOUNT_NUMBER,
} = require("./helpers/fixtures");

// Balances are read from the UI before each case (rather than hardcoded)
// so assertions hold regardless of what earlier tests/files already spent.
// Fills the form using the "specify amount" option, since the presets
// (1000/5000/10000/50000/100000/500000) don't cover the arbitrary/edge-case
// amounts most of these tests exercise. See "preset amount" test below for
// the dropdown-preset path.
async function fillBuyForm(page, { amount, productLabel }) {
  await page.getByTestId("funding-account-select").selectOption(SAVINGS_ACCOUNT_ID);
  await page.getByTestId("salak-account-select").selectOption(SALAK_ACCOUNT_ID);
  await page.getByTestId("product-select").selectOption({ label: productLabel });
  await page.getByTestId("amount-select").selectOption("custom");
  await page.getByTestId("custom-amount-input").fill(String(amount));
}

test.describe("buy salak", () => {
  test("success: buying 30000 THB of the 1-year product", async ({ page }) => {
    const shoot = createShooter("buy-salak", "success");

    await loginAsDemo(page);
    const fundingBefore = await readBalance(page, SAVINGS_ACCOUNT_NUMBER);
    const salakBefore = await readBalance(page, SALAK_ACCOUNT_NUMBER);
    const holdingsBefore = await page.getByTestId("holding-row").count();

    await expect(page.getByTestId("custom-amount-input")).toHaveAttribute("step", "1000");
    await expect(page.getByTestId("amount-hint")).toHaveText(/multiple of 1000 THB/i);

    await fillBuyForm(page, { amount: 30000, productLabel: "สลากดิจิทัล 1 ปี" });
    await shoot(page, "form-filled");

    await page.getByTestId("buy-submit").click();

    const receipt = page.getByTestId("receipt");
    await expect(receipt).toBeVisible();
    await expect(page.getByTestId("receipt-product-name")).toHaveText("สลากดิจิทัล 1 ปี");
    await expect(page.getByTestId("receipt-units")).toHaveText("300");
    await expect(page.getByTestId("receipt-funding-balance")).toHaveText(String(fundingBefore - 30000));
    await expect(page.getByTestId("receipt-salak-balance")).toHaveText(String(salakBefore + 30000));

    // Ticket IDs are "[ก-ฮ][0-9]{7}" - one randomized Thai consonant
    // (shared by the whole holding) followed by a sequential 7-digit number.
    const ticketStartText = await page.getByTestId("receipt-ticket-start").textContent();
    const ticketEndText = await page.getByTestId("receipt-ticket-end").textContent();

    const ticketLetter = ticketStartText[0];
    expect(ticketEndText[0]).toBe(ticketLetter);
    const letterCode = ticketLetter.codePointAt(0);
    expect(letterCode).toBeGreaterThanOrEqual(0x0e01); // ก
    expect(letterCode).toBeLessThanOrEqual(0x0e2e); // ฮ

    const ticketStart = Number(ticketStartText.slice(1));
    const ticketEnd = Number(ticketEndText.slice(1));
    expect(ticketEnd - ticketStart + 1).toBe(300);
    await shoot(page, "receipt-shown");

    expect(await readBalance(page, SAVINGS_ACCOUNT_NUMBER)).toBe(fundingBefore - 30000);
    expect(await readBalance(page, SALAK_ACCOUNT_NUMBER)).toBe(salakBefore + 30000);

    await expect(page.getByTestId("holding-row")).toHaveCount(holdingsBefore + 1);
    const newHolding = page.getByTestId("holding-row").filter({ hasText: "สลากดิจิทัล 1 ปี" }).last();
    await expect(newHolding.getByTestId("holding-units")).toHaveText("300");
    await expect(newHolding.getByTestId("holding-ticket-range")).toHaveText(`${ticketStartText} - ${ticketEndText}`);
    await shoot(page, "accounts-updated");
  });

  test("selecting \"Specify amount\" reveals the custom amount input", async ({ page }) => {
    const shoot = createShooter("buy-salak", "specify-amount-reveals-input");

    await loginAsDemo(page);
    await expect(page.getByTestId("custom-amount-input")).toBeHidden();
    await shoot(page, "presets-only");

    await page.getByTestId("amount-select").selectOption("custom");
    await expect(page.getByTestId("custom-amount-input")).toBeVisible();
    await shoot(page, "custom-input-visible");

    await page.getByTestId("amount-select").selectOption("10000");
    await expect(page.getByTestId("custom-amount-input")).toBeHidden();
    await shoot(page, "hidden-again");
  });

  test("success: buying via a preset amount option (10000 THB)", async ({ page }) => {
    const shoot = createShooter("buy-salak", "preset-amount");

    await loginAsDemo(page);
    const fundingBefore = await readBalance(page, SAVINGS_ACCOUNT_NUMBER);
    const salakBefore = await readBalance(page, SALAK_ACCOUNT_NUMBER);

    await page.getByTestId("funding-account-select").selectOption(SAVINGS_ACCOUNT_ID);
    await page.getByTestId("salak-account-select").selectOption(SALAK_ACCOUNT_ID);
    await page.getByTestId("product-select").selectOption({ label: "สลากดิจิทัล 1 ปี" });
    await page.getByTestId("amount-select").selectOption("10000");
    await shoot(page, "form-filled");

    await page.getByTestId("buy-submit").click();

    const receipt = page.getByTestId("receipt");
    await expect(receipt).toBeVisible();
    await expect(page.getByTestId("receipt-units")).toHaveText("100");
    await expect(page.getByTestId("receipt-funding-balance")).toHaveText(String(fundingBefore - 10000));
    await expect(page.getByTestId("receipt-salak-balance")).toHaveText(String(salakBefore + 10000));
    await shoot(page, "receipt-shown");

    expect(await readBalance(page, SAVINGS_ACCOUNT_NUMBER)).toBe(fundingBefore - 10000);
    expect(await readBalance(page, SALAK_ACCOUNT_NUMBER)).toBe(salakBefore + 10000);
  });

  test("amount not a multiple of 1000 is rejected", async ({ page }) => {
    const shoot = createShooter("buy-salak", "amount-not-multiple-of-1000");

    await loginAsDemo(page);
    const fundingBefore = await readBalance(page, SAVINGS_ACCOUNT_NUMBER);

    await fillBuyForm(page, { amount: 1500, productLabel: "สลากดิจิทัล 1 ปี" });
    await shoot(page, "form-filled");

    await page.getByTestId("buy-submit").click();

    const error = page.getByTestId("buy-error");
    await expect(error).toHaveText(/multiple of the step amount/i);
    await expect(page.getByTestId("receipt")).toBeHidden();
    await shoot(page, "error-shown");

    expect(await readBalance(page, SAVINGS_ACCOUNT_NUMBER)).toBe(fundingBefore);
  });

  test("amount exceeding the product maximum is rejected", async ({ page }) => {
    const shoot = createShooter("buy-salak", "amount-exceeds-max");

    await loginAsDemo(page);
    const fundingBefore = await readBalance(page, SAVINGS_ACCOUNT_NUMBER);

    await fillBuyForm(page, { amount: 20_000_000, productLabel: "สลากดิจิทัล 1 ปี" });
    await shoot(page, "form-filled");

    await page.getByTestId("buy-submit").click();

    const error = page.getByTestId("buy-error");
    await expect(error).toHaveText(/exceeds the maximum/i);
    await shoot(page, "error-shown");

    expect(await readBalance(page, SAVINGS_ACCOUNT_NUMBER)).toBe(fundingBefore);
  });

  test("insufficient funds is rejected without changing balances", async ({ page }) => {
    const shoot = createShooter("buy-salak", "insufficient-funds");

    await loginAsDemo(page);
    const fundingBefore = await readBalance(page, SAVINGS_ACCOUNT_NUMBER);
    const tooMuch = fundingBefore + 1000; // still a multiple of 1000, still under max, but more than available

    await fillBuyForm(page, { amount: tooMuch, productLabel: "สลากดิจิทัล 1 ปี" });
    await shoot(page, "form-filled");

    await page.getByTestId("buy-submit").click();

    const error = page.getByTestId("buy-error");
    await expect(error).toHaveText(/insufficient funds/i);
    await shoot(page, "error-shown");

    expect(await readBalance(page, SAVINGS_ACCOUNT_NUMBER)).toBe(fundingBefore);
  });
});
