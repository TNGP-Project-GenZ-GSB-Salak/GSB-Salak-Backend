ALTER TABLE account.accounts DROP CONSTRAINT accounts_type_check;
-- NOT VALID: re-adding the stricter constraint would otherwise fail this
-- migration immediately against any 'kapook' rows already written. NOT
-- VALID skips that one-time validation of existing rows, so the rollback
-- itself always succeeds and doesn't touch/delete any 'kapook' data - but
-- Postgres re-validates a row's CHECK constraints in full on every
-- subsequent UPDATE, not just changed columns, so an existing 'kapook'
-- account would start failing on its very next balance update (or any
-- other column) after this rollback. That's an accepted consequence of
-- rolling back, not a bug in this migration - the constraint is doing
-- exactly what a rollback should: block 'kapook' going forward.
ALTER TABLE account.accounts ADD CONSTRAINT accounts_type_check CHECK (type IN ('savings', 'salak')) NOT VALID;
