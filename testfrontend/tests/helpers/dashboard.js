// Reads the balance currently rendered for a given account number on the
// dashboard's accounts table. Used to compute expected deltas rather than
// asserting on hardcoded absolute balances, so tests stay order-independent.
async function readBalance(page, accountNumber) {
  const rows = page.getByTestId("account-row");
  const count = await rows.count();

  for (let i = 0; i < count; i++) {
    const row = rows.nth(i);
    const number = await row.getByTestId("account-number").textContent();
    if (number === accountNumber) {
      const balanceText = await row.getByTestId("account-balance").textContent();
      return Number(balanceText);
    }
  }
  throw new Error(`account ${accountNumber} not found on dashboard`);
}

module.exports = { readBalance };
