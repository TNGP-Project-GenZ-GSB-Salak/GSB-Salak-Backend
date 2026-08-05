UPDATE salak.products SET name = 'Digital Salak 1-Year' WHERE code = 'SALAK_1Y';
UPDATE salak.products SET name = 'Digital Salak 2-Year' WHERE code = 'SALAK_2Y';

UPDATE transaction.ledger_entries SET description = 'Buy Digital Salak 1-Year' WHERE description = 'ซื้อสลากดิจิทัล 1 ปี';
UPDATE transaction.ledger_entries SET description = 'Buy Digital Salak 2-Year' WHERE description = 'ซื้อสลากดิจิทัล 2 ปี';
UPDATE transaction.ledger_entries SET description = 'Salak maturity - principal' WHERE description = 'สลากครบกำหนด - เงินต้น';
UPDATE transaction.ledger_entries SET description = 'Salak maturity - interest' WHERE description = 'สลากครบกำหนด - ดอกเบี้ย';
UPDATE transaction.ledger_entries SET description = 'Kapook deposit' WHERE description = 'ฝากเงินเข้ากระปุกออมสลาก';
UPDATE transaction.ledger_entries SET description = 'Kapook withdrawal' WHERE description = 'ถอนเงินจากกระปุกออมสลาก';
UPDATE transaction.ledger_entries SET description = 'Kapook withdrawal (2% fee)' WHERE description = 'ถอนเงินจากกระปุกออมสลาก (หักค่าธรรมเนียม 2%)';
