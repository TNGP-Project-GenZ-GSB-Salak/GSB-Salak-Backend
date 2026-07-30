const { expect } = require("@playwright/test");

// Reads the balance currently rendered for a given account number on the
// dashboard's accounts table. Used to compute expected deltas rather than
// asserting on hardcoded absolute balances, so tests stay order-independent.
//
// Uses an auto-retrying locator rather than a one-shot count()+loop:
// dashboard.html's loadAccounts() re-fetches "/accounts" and then clears
// and rebuilds the whole table (innerHTML = "" followed by re-appending
// rows), which buy-salak's submit handler triggers right after showing the
// receipt. A single synchronous scan can land in that momentarily-empty
// window and wrongly report the account as missing; expect(...).toHaveCount
// polls until the row reappears (already reflecting the post-refresh
// balance, since the row is only rebuilt after the new data has loaded).
async function readBalance(page, accountNumber) {
  const row = page.getByTestId("account-row").filter({
    has: page.getByTestId("account-number").getByText(accountNumber, { exact: true }),
  });
  await expect(row).toHaveCount(1);
  return Number(await row.getByTestId("account-balance").textContent());
}

module.exports = { readBalance };
