ALTER TABLE account.accounts DROP CONSTRAINT accounts_type_check;
ALTER TABLE account.accounts ADD CONSTRAINT accounts_type_check CHECK (type IN ('savings', 'salak', 'kapook'));
