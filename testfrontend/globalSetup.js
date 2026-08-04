// Resets the seeded demo user's data to a known state before the suite runs,
// so tests are deterministic and repeatable regardless of prior runs
// (buy-salak mutates balances/ticket numbers, which would otherwise drift).
// Assumes docker-compose Postgres is already up, migrated, and seeded once
// (see testfrontend/README.md).
const { execSql } = require("./tests/helpers/db");

module.exports = async function globalSetup() {
  const statements = [
    "UPDATE account.accounts SET balance = 50000 WHERE account_number = '1234009012'",
    "UPDATE account.accounts SET balance = 0 WHERE account_number = '4001000111'",
    "DELETE FROM transaction.ledger_entries",
    "DELETE FROM salak.holdings",
    "UPDATE salak.ticket_sequence SET next_ticket_number = 1 WHERE id = 1",
  ].join("; ");

  try {
    execSql(statements);
  } catch (err) {
    console.error(
      "\nFailed to reset demo data before the test run.\n" +
        "Make sure docker-compose Postgres is running, migrated (`go run ./cmd/migrate up`), " +
        "and seeded once (`SEED_DEMO_DATA=true go run ./cmd/seed`).\n"
    );
    throw err;
  }
};
