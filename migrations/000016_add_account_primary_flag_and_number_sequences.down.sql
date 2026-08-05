DROP SEQUENCE account.kapook_account_number_seq;
DROP SEQUENCE account.salak_account_number_seq;
DROP SEQUENCE account.savings_account_number_seq;

DROP INDEX account.idx_accounts_primary_per_user;
ALTER TABLE account.accounts DROP CONSTRAINT accounts_primary_only_savings;
ALTER TABLE account.accounts DROP COLUMN is_primary_account;
