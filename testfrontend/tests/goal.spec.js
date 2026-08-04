const { test, expect } = require("@playwright/test");
const { createShooter } = require("./helpers/screenshot");
const { loginAsDemo } = require("./helpers/auth");

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

    // Reloading re-fetches from the API rather than trusting client state,
    // proving the goal actually persisted server-side.
    await page.reload();
    await expect(page.getByTestId("goal-view")).toBeVisible();
    await expect(page.getByTestId("goal-form-section")).toBeHidden();
    await shoot(page, "still-active-after-reload");
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
});
