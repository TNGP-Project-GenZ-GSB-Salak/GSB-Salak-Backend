const { test, expect } = require("@playwright/test");
const { createShooter } = require("./helpers/screenshot");
const { loginAsDemo } = require("./helpers/auth");
const { SAVINGS_ACCOUNT_ID, SALAK_ACCOUNT_ID } = require("./helpers/fixtures");

// This spec exercises the real worker process Playwright's webServer
// config starts alongside the API (see ../playwright.config.js) - not a
// direct RunOnce call, which is what the Go integration/unit suites use
// instead of ever sleeping for a tick. Here, waiting for a real tick is
// the point: it is this ticket's silent-failure risk made visible - if the
// worker were ever left out of the webServer config, this test would hang
// until timeout with the countdown status stuck at "pending", not fail
// fast with a clear error.
test.describe("kapook countdown auto-purchase", () => {
  test("reaching the target starts a purchase-pending countdown, and the worker buys it without any further action", async ({ page }) => {
    // KAPOOK_COUNTDOWN_DURATION=10s (see fixtures.js) plus up to one ~1
    // minute worker tick - generous headroom over the default 30s.
    test.setTimeout(120_000);
    const shoot = createShooter("countdown", "auto-purchase");

    await loginAsDemo(page);

    // Runs alphabetically before goal.spec.js and terms.spec.js in this
    // suite, so neither the demo user's terms acceptance nor any active
    // goal can be assumed - accept via the real UI flow, same defensive
    // pattern goal.spec.js's first test uses.
    await page.goto("/terms.html");
    const acceptButton = page.getByTestId("accept-terms-button");
    if (await acceptButton.isEnabled()) {
      await acceptButton.click();
      await expect(page.getByTestId("terms-status")).toHaveText("Accepted");
    }

    await page.goto("/goal.html");
    await expect(page.getByTestId("goal-form-section")).toBeVisible();

    // purchase_date is a date, not a timestamp, and this whole suite runs
    // within one real calendar day - so every holding it creates ties on
    // the same purchase_date. FindByAccountID's "ORDER BY purchase_date
    // DESC" is therefore not a stable order among them: the array's last
    // element is not reliably "the one this test just bought." Snapshot
    // ids first and diff afterward instead.
    const baselineHoldings = await page.evaluate((accountId) => apiFetch(`/salak/holdings?account_id=${accountId}`), SALAK_ACCOUNT_ID);
    const baselineHoldingIds = new Set(baselineHoldings.map((h) => h.id));

    // The smallest valid target for SALAK_1Y (min purchase = step = 1000),
    // so a single deposit reaches it immediately.
    await page.getByTestId("product-select").selectOption({ label: "Digital Salak 1-Year" });
    await page.getByTestId("goal-amount-input").fill("1000");
    await page.getByTestId("create-goal-submit").click();
    await expect(page.getByTestId("goal-view")).toBeVisible();
    await shoot(page, "goal-created");

    await page.getByTestId("savings-account-select").selectOption(SAVINGS_ACCOUNT_ID);
    await page.getByTestId("deposit-amount-input").fill("1000");
    await page.getByTestId("deposit-submit").click();

    await expect(page.getByTestId("goal-saved")).toHaveText("1000");
    await expect(page.getByTestId("countdown-status")).toHaveText(/pending/i);
    await shoot(page, "target-reached-purchase-pending");

    // Wait for the real worker process to notice and act - no sleeping on
    // a fixed duration; poll until the pending state resolves one way or
    // the other, exactly what a customer watching this screen would see.
    await expect(page.getByTestId("countdown-status")).toHaveText(/completed/i, { timeout: 100_000 });
    await expect(page.getByTestId("auto-purchase-result")).toHaveText(/Holding: \d+ units, tickets .+ to .+\./i);
    await shoot(page, "auto-purchase-completed");

    // The goal is done - the create form is back, and reloading confirms
    // this is real server state, not a client-side illusion.
    await expect(page.getByTestId("goal-form-section")).toBeVisible();
    await page.reload();
    await expect(page.getByTestId("goal-form-section")).toBeVisible();
    await expect(page.getByTestId("goal-view")).toBeHidden();

    const holdingsAfter = await page.evaluate((accountId) => apiFetch(`/salak/holdings?account_id=${accountId}`), SALAK_ACCOUNT_ID);
    const newHoldings = holdingsAfter.filter((h) => !baselineHoldingIds.has(h.id));
    expect(newHoldings).toHaveLength(1);
    expect(newHoldings[0].purchase_amount).toBe("1000");
  });
});
