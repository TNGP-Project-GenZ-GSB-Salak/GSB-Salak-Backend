-- Product names and ledger descriptions were left in English from the
-- original scaffold, but every other piece of customer-facing copy in this
-- product is Thai - the frontend renders both fields verbatim with no
-- translation layer (there's no code-based i18n for these, unlike error
-- messages). Backfills already-persisted rows to match the Go-side fix
-- (cmd/seed's product literals, and the Description string builders in
-- internal/transaction/service and internal/kapook/service).
UPDATE salak.products SET name = 'สลากดิจิทัล 1 ปี' WHERE code = 'SALAK_1Y';
UPDATE salak.products SET name = 'สลากดิจิทัล 2 ปี' WHERE code = 'SALAK_2Y';

UPDATE transaction.ledger_entries SET description = 'ซื้อสลากดิจิทัล 1 ปี' WHERE description = 'Buy Digital Salak 1-Year';
UPDATE transaction.ledger_entries SET description = 'ซื้อสลากดิจิทัล 2 ปี' WHERE description = 'Buy Digital Salak 2-Year';
UPDATE transaction.ledger_entries SET description = 'สลากครบกำหนด - เงินต้น' WHERE description = 'Salak maturity - principal';
UPDATE transaction.ledger_entries SET description = 'สลากครบกำหนด - ดอกเบี้ย' WHERE description = 'Salak maturity - interest';
UPDATE transaction.ledger_entries SET description = 'ฝากเงินเข้ากระปุกออมสลาก' WHERE description = 'Kapook deposit';
UPDATE transaction.ledger_entries SET description = 'ถอนเงินจากกระปุกออมสลาก' WHERE description = 'Kapook withdrawal';
UPDATE transaction.ledger_entries SET description = 'ถอนเงินจากกระปุกออมสลาก (หักค่าธรรมเนียม 2%)' WHERE description = 'Kapook withdrawal (2% fee)';
