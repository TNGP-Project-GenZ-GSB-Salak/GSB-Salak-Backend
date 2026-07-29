CREATE TABLE account.accounts (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid NOT NULL REFERENCES "user".users(id),
    account_number  varchar(20) NOT NULL UNIQUE,
    type            varchar(20) NOT NULL CHECK (type IN ('savings', 'salak')),
    balance         numeric(18,2) NOT NULL DEFAULT 0 CHECK (balance >= 0),
    currency        varchar(3) NOT NULL DEFAULT 'THB',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_accounts_user_id ON account.accounts (user_id);
