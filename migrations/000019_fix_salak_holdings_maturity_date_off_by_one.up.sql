-- MaturityDate was computed as purchase_date + term_months, one day later
-- than the official rule (purchase + term - 1 day - see the 1-year sheet's
-- own worked example: deposit 3 ก.ค. matures 2 ก.ค. the following year).
-- The bug is a constant one-day offset regardless of product, so the fix
-- for already-minted rows is this one-line shift - no join to products
-- needed, since MintHolding itself no longer computes the wrong date.
UPDATE salak.holdings SET maturity_date = maturity_date - interval '1 day';
