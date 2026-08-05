-- The primary account (บัญชีคู่โอน) - MVP#1 ticket 11's recorded shape.
-- "At least one" isn't declaratively expressible, so this only prevents
-- *two*; every registered user gets exactly one via application code
-- (internal/user/service/auth_service.go), backfilled below for existing
-- users. account.Service.GetPrimaryAccount is the only reader; the
-- NotFound path stays for the zero case.
ALTER TABLE account.accounts ADD COLUMN is_primary_account boolean NOT NULL DEFAULT false;

ALTER TABLE account.accounts
    ADD CONSTRAINT accounts_primary_only_savings CHECK (NOT is_primary_account OR type = 'savings');

CREATE UNIQUE INDEX idx_accounts_primary_per_user ON account.accounts (user_id) WHERE is_primary_account;

-- Backfill: each existing user's earliest savings account becomes primary.
UPDATE account.accounts a
SET is_primary_account = true
FROM (
    SELECT DISTINCT ON (user_id) id
    FROM account.accounts
    WHERE type = 'savings'
    ORDER BY user_id, created_at ASC
) earliest
WHERE a.id = earliest.id;

-- Per-type account-number sequences for registration
-- (internal/account/repository/gorm_account_repo.go's NextAccountNumber),
-- mirroring salak.ticket_sequence's "reserve deterministically, never
-- retry" idiom. Each type's 2-digit prefix (61/62/63) is disjoint from every
-- seeded account number's leading digits (12340..., 40010..., 50010...), so
-- collision is impossible by construction regardless of how far a sequence
-- has advanced - not merely "started above" the three seeded values.
CREATE SEQUENCE account.savings_account_number_seq START WITH 1;
CREATE SEQUENCE account.salak_account_number_seq START WITH 1;
CREATE SEQUENCE account.kapook_account_number_seq START WITH 1;
