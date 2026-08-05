-- Null = not yet settled. Doubles as the idempotency guard for
-- SettleMaturedHolding (can't settle the same holding twice) and, later,
-- an automated worker's claim predicate (maturity_date <= today AND
-- settled_at IS NULL) - nothing sets it yet except the new settlement
-- service.
ALTER TABLE salak.holdings ADD COLUMN settled_at timestamptz;
