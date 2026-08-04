// Direct Postgres access for test setup/teardown the app's own API has no
// endpoint for (resetting demo data, seeding a one-off draw_dates row).
// Shared by globalSetup.js and any spec that needs it.
const { execSync } = require("child_process");

const CONTAINER = process.env.DB_CONTAINER || "gsb-salak-backend-db-1";
const DB_NAME = process.env.DB_NAME || "gsb_salak";

function execSql(sql) {
  execSync(`docker exec ${CONTAINER} psql -U postgres -d ${DB_NAME} -c "${sql}"`, {
    stdio: "inherit",
  });
}

// queryOne runs sql and returns the first column of the first row as a
// trimmed string (psql's -tAc gives unaligned, tuples-only output).
function queryOne(sql) {
  return execSync(`docker exec ${CONTAINER} psql -U postgres -d ${DB_NAME} -tAc "${sql}"`)
    .toString()
    .trim();
}

module.exports = { execSql, queryOne };
