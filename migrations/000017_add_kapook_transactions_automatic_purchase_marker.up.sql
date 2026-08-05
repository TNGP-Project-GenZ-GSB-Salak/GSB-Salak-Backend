-- Ticket 09's expand step: the column exists and is exposed over the API,
-- nullable, before anything sets it - ticket 10's worker populates it on
-- buy_salak rows it performs unattended, so the countdown/history features
-- don't have a circular dependency on each other. NULL for every row until
-- then, including every customer-initiated purchase.
ALTER TABLE kapook.kapook_transactions
    ADD COLUMN is_automatic_purchase boolean;
