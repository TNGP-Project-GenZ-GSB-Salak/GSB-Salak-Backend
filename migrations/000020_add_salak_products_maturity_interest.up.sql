-- Per-unit baht payout at maturity (D11: per-product column, not a code
-- constant, matching the precedent of unit_price/min/max/step_amount
-- already living on this row). Two-step idiom (same one migration 000007
-- used for ticket_letter): add nullable, backfill the two seeded products,
-- then lock down NOT NULL + a positivity check.
ALTER TABLE salak.products ADD COLUMN maturity_interest_per_unit numeric(10,4);

UPDATE salak.products SET maturity_interest_per_unit = 0.15 WHERE code = 'SALAK_1Y';
UPDATE salak.products SET maturity_interest_per_unit = 0.50 WHERE code = 'SALAK_2Y';

ALTER TABLE salak.products ALTER COLUMN maturity_interest_per_unit SET NOT NULL;
ALTER TABLE salak.products ADD CONSTRAINT maturity_interest_per_unit_positive CHECK (maturity_interest_per_unit > 0);
