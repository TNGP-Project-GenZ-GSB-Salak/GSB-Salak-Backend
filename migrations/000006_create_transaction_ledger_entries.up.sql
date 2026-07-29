CREATE TABLE transaction.ledger_entries (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id      uuid NOT NULL REFERENCES account.accounts(id),
    holding_id      uuid REFERENCES salak.holdings(id),
    type            varchar(20) NOT NULL CHECK (type IN ('debit', 'credit')),
    amount          numeric(18,2) NOT NULL CHECK (amount > 0),
    balance_after   numeric(18,2) NOT NULL,
    reference_type  varchar(30) NOT NULL DEFAULT 'buy_salak',
    reference_id    uuid NOT NULL,
    description     varchar(255) NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_ledger_account_id_created_at ON transaction.ledger_entries (account_id, created_at DESC);
CREATE INDEX idx_ledger_reference_id ON transaction.ledger_entries (reference_id);
