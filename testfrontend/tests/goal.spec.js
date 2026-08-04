const { test, expect } = require("@playwright/test");
const { createShooter } = require("./helpers/screenshot");
const { loginAsDemo } = require("./helpers/auth");
const { SAVINGS_ACCOUNT_ID } = require("./helpers/fixtures");

test.describe("kapook goal", () => {
  test("creating a goal shows it with progress, and it persists across reload", async ({ page }) => {
    const shoot = createShooter("goal", "create-and-progress");

    await loginAsDemo(page);

    // Goal creation is gated on terms acceptance. Accept via the real UI
    // flow rather than assuming terms.spec.js already ran - Playwright runs
    // spec files in their own order (alphabetically, "goal" comes before
    // "terms"), so that can't be relied on.
    await page.goto("/terms.html");
    const acceptButton = page.getByTestId("accept-terms-button");
    if (await acceptButton.isEnabled()) {
      await acceptButton.click();
      await expect(page.getByTestId("terms-status")).toHaveText("Accepted");
    }

    await page.goto("/goal.html");
    await expect(page.getByTestId("goal-form-section")).toBeVisible();
    await expect(page.getByTestId("goal-view")).toBeHidden();
    await shoot(page, "no-goal-yet");

    await page.getByTestId("product-select").selectOption({ label: "Digital Salak 1-Year" });
    await page.getByTestId("goal-amount-input").fill("5000");
    await shoot(page, "form-filled");

    await page.getByTestId("create-goal-submit").click();

    const goalView = page.getByTestId("goal-view");
    await expect(goalView).toBeVisible();
    await expect(page.getByTestId("goal-target")).toHaveText("5000");
    await expect(page.getByTestId("goal-saved")).toHaveText("0");
    await expect(page.getByTestId("goal-converted")).toHaveText("0");
    await expect(page.getByTestId("goal-progress")).toHaveText("0");
    await shoot(page, "goal-created");

    // Pay in from the demo user's savings account and see progress move.
    await page.getByTestId("savings-account-select").selectOption(SAVINGS_ACCOUNT_ID);
    await page.getByTestId("deposit-amount-input").fill("1500");
    await shoot(page, "deposit-form-filled");

    await page.getByTestId("deposit-submit").click();

    await expect(page.getByTestId("goal-saved")).toHaveText("1500");
    await expect(page.getByTestId("goal-progress")).toHaveText("30");
    await shoot(page, "progress-after-deposit");

    // Reloading re-fetches from the API rather than trusting client state,
    // proving the goal AND the deposit actually persisted server-side.
    await page.reload();
    await expect(page.getByTestId("goal-view")).toBeVisible();
    await expect(page.getByTestId("goal-form-section")).toBeHidden();
    await expect(page.getByTestId("goal-saved")).toHaveText("1500");
    await shoot(page, "still-active-after-reload");
  });

  test("a deposit that would exceed the target is rejected without changing the balance", async ({ page }) => {
    const shoot = createShooter("goal", "deposit-exceeds-target");

    await loginAsDemo(page);
    await page.goto("/goal.html");

    // Continues from the previous test: goal target 5000, already saved 1500.
    await expect(page.getByTestId("goal-view")).toBeVisible();
    const savingsBefore = await page.evaluate(() =>
      apiFetch("/accounts").then((accounts) => accounts.find((a) => a.type === "savings").balance)
    );

    await page.getByTestId("savings-account-select").selectOption(SAVINGS_ACCOUNT_ID);
    await page.getByTestId("deposit-amount-input").fill("4000"); // 1500 + 4000 = 5500 > 5000 target
    await shoot(page, "form-filled");

    await page.getByTestId("deposit-submit").click();

    const error = page.getByTestId("message");
    await expect(error).toHaveText(/exceed the goal's target/i);
    // The rejected deposit must not have moved progress.
    await expect(page.getByTestId("goal-saved")).toHaveText("1500");
    await shoot(page, "rejected");

    const savingsAfter = await page.evaluate(() =>
      apiFetch("/accounts").then((accounts) => accounts.find((a) => a.type === "savings").balance)
    );
    expect(savingsAfter).toBe(savingsBefore);
  });

  test("a second active goal is rejected by the API even though the UI hides the form", async ({ page }) => {
    const shoot = createShooter("goal", "second-goal-rejected");

    await loginAsDemo(page);
    await page.goto("/goal.html");

    // Already has an active goal from the previous test in this file (same
    // demo user/account; globalSetup resets kapook.kapook_goals once per
    // suite RUN, not once per test) - the form is gone, exactly the UI
    // behaviour ticket 05 asks for. To prove the rejection is real and not
    // just "the button is missing", call the endpoint directly.
    await expect(page.getByTestId("goal-view")).toBeVisible();
    await shoot(page, "already-has-goal");

    const accounts = await page.evaluate(() => apiFetch("/accounts"));
    const kapookAccount = accounts.find((a) => a.type === "kapook");
    const products = await page.evaluate(() => apiFetch("/salak/products"));

    const errorMessage = await page.evaluate(
      async ({ accountId, productId }) => {
        try {
          await apiFetch("/kapook/goals", {
            method: "POST",
            body: JSON.stringify({ account_id: accountId, product_id: productId, goal_amount: 5000 }),
          });
          return null;
        } catch (err) {
          return err.message;
        }
      },
      { accountId: kapookAccount.id, productId: products[0].id }
    );

    expect(errorMessage).toMatch(/active goal already exists/i);
    await shoot(page, "second-attempt-rejected");
  });

  test("withdrawing shows whether it's free or charged, and the fee kicks in after two free withdrawals", async ({ page }) => {
    const shoot = createShooter("goal", "withdraw");

    await loginAsDemo(page);
    await page.goto("/goal.html");

    // Continues from the earlier tests in this file: goal target 5000,
    // already saved 1500 - two free withdrawals available in the window.
    await expect(page.getByTestId("goal-view")).toBeVisible();
    await expect(page.getByTestId("withdrawal-status")).toHaveText(/free/i);
    await shoot(page, "before-any-withdrawal");

    // 1st withdrawal: free.
    await page.getByTestId("withdraw-savings-account-select").selectOption(SAVINGS_ACCOUNT_ID);
    await page.getByTestId("withdraw-amount-input").fill("500");
    await page.getByTestId("withdraw-submit").click();

    await expect(page.getByTestId("goal-saved")).toHaveText("1000");
    await expect(page.getByTestId("withdraw-result")).toHaveText(/no fee/i);
    await expect(page.getByTestId("withdrawal-status")).toHaveText(/free/i);
    await shoot(page, "first-withdrawal-free");

    // 2nd withdrawal: still free, but it's the last one - status should now
    // warn the next one will be charged.
    await page.getByTestId("withdraw-amount-input").fill("500");
    await page.getByTestId("withdraw-submit").click();

    await expect(page.getByTestId("goal-saved")).toHaveText("500");
    await expect(page.getByTestId("withdraw-result")).toHaveText(/no fee/i);
    await expect(page.getByTestId("withdrawal-status")).toHaveText(/2% fee/i);
    await shoot(page, "second-withdrawal-free-allowance-exhausted");

    // 3rd withdrawal in the same window: charged the 2% fee.
    await page.getByTestId("withdraw-amount-input").fill("100");
    await page.getByTestId("withdraw-submit").click();

    await expect(page.getByTestId("goal-saved")).toHaveText("400");
    await expect(page.getByTestId("withdraw-result")).toHaveText(/2% fee applied - 98 THB reached savings/i);
    await shoot(page, "third-withdrawal-charged");

    // The goal survives every withdrawal, including one that would empty it.
    await page.getByTestId("withdraw-amount-input").fill("400");
    await page.getByTestId("withdraw-submit").click();

    await expect(page.getByTestId("goal-saved")).toHaveText("0");
    await expect(page.getByTestId("goal-view")).toBeVisible();
    await shoot(page, "emptied-but-goal-still-active");

    await page.reload();
    await expect(page.getByTestId("goal-view")).toBeVisible();
    await expect(page.getByTestId("goal-form-section")).toBeHidden();
    await expect(page.getByTestId("goal-saved")).toHaveText("0");
  });
});
