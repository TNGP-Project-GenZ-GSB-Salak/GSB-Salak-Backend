// Resets the seeded demo user's data to a known state before the suite runs,
// so tests are deterministic and repeatable regardless of prior runs
// (buy-salak mutates balances/ticket numbers, which would otherwise drift).
// Assumes docker-compose Postgres is already up, migrated, and seeded once
// (see testfrontend/README.md).
const { execSync } = require("child_process");

module.exports = async function globalSetup() {
  const container = process.env.DB_CONTAINER || "gsb-salak-backend-db-1";
  const dbName = process.env.DB_NAME || "gsb_salak";

  const statements = [
    "UPDATE account.accounts SET balance = 50000 WHERE account_number = '1234009012'",
    "UPDATE account.accounts SET balance = 0 WHERE account_number = '4001000111'",
    "DELETE FROM transaction.ledger_entries",
    "DELETE FROM salak.holdings",
    "UPDATE salak.ticket_sequence SET next_ticket_number = 1 WHERE id = 1",
    "DELETE FROM kapook.terms_acceptances WHERE user_id = '11111111-1111-1111-1111-111111111111'",
    "DELETE FROM kapook.kapook_goals WHERE account_id = '44444444-4444-4444-4444-444444444444'",
  ].join("; ");

  try {
    execSync(`docker exec ${container} psql -U postgres -d ${dbName} -c "${statements}"`, {
      stdio: "inherit",
    });
  } catch (err) {
    console.error(
      "\nFailed to reset demo data before the test run.\n" +
        "Make sure docker-compose Postgres is running, migrated (`go run ./cmd/migrate up`), " +
        "and seeded once (`SEED_DEMO_DATA=true go run ./cmd/seed`).\n"
    );
    throw err;
  }
};
