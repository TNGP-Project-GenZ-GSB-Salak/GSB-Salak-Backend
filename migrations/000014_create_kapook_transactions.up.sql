-- Five values, not six: an exempt-window bail-out withdrawal was considered
-- and dropped, since it carries no fee or quota exemption - it's an
-- ordinary 'withdraw'/'withdraw_with_fee' row.
--
-- kapook_account_id/savings_account_id are qualified names (not a bare
-- account_id), since a row can reference two distinct Account roles at
-- once - unlike ledger_entries/holdings/kapook_goals, which each reference
-- exactly one. Both savings_account_id and holding_id are nullable since no
-- single transaction type uses both: deposit/withdraw/withdraw_with_fee use
-- savings_account_id only, buy_salak/salak_expiration use holding_id only.
CREATE TABLE kapook.kapook_transactions (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    type                varchar(20) NOT NULL CHECK (type IN ('deposit', 'withdraw', 'withdraw_with_fee', 'buy_salak', 'salak_expiration')),
    amount              numeric(18,2) NOT NULL CHECK (amount > 0),
    kapook_account_id   uuid NOT NULL REFERENCES account.accounts(id),
    savings_account_id  uuid REFERENCES account.accounts(id),
    holding_id          uuid REFERENCES salak.holdings(id),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_kapook_transactions_kapook_account_id ON kapook.kapook_transactions (kapook_account_id);
