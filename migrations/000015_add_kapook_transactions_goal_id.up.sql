-- Every kapook_transaction happens in the context of exactly one goal (the
-- account's active goal at the time), but the account_id-only shape from
-- 000014 can't distinguish which goal once an account has hosted more than
-- one over its life. That's needed starting with ticket 07: the free-
-- withdrawal allowance is counted per-goal, not per-account.
ALTER TABLE kapook.kapook_transactions
    ADD COLUMN goal_id uuid NOT NULL REFERENCES kapook.kapook_goals(id);

CREATE INDEX idx_kapook_transactions_goal_id ON kapook.kapook_transactions (goal_id);
