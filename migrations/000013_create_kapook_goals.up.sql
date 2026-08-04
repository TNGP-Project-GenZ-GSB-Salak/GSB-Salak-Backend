CREATE TABLE kapook.kapook_goals (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id      uuid NOT NULL REFERENCES account.accounts(id),
    product_id      uuid NOT NULL REFERENCES salak.products(id),
    goal_amount     numeric(18,2) NOT NULL CHECK (goal_amount > 0),
    saving_amount   numeric(18,2) NOT NULL DEFAULT 0 CHECK (saving_amount >= 0),
    salak_amount    numeric(18,2) NOT NULL DEFAULT 0 CHECK (salak_amount >= 0),
    is_active       boolean NOT NULL DEFAULT true,
    goal_reached_at timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    -- Can't have saved/converted more than the target itself, or converted
    -- more than was ever saved. Both are static, product-independent
    -- invariants (unlike goal_amount's step/max rule, which needs the
    -- referenced product row and so can't be a CHECK constraint).
    CHECK (saving_amount <= goal_amount),
    CHECK (salak_amount <= saving_amount)
);

-- AccountID is deliberately NOT unique - one kapook account hosts many goals
-- over its life (one per goal set/reached/replaced). At most one may be
-- ACTIVE at a time, which a plain UNIQUE can't express - hence a partial
-- index instead.
CREATE UNIQUE INDEX idx_kapook_goals_account_active ON kapook.kapook_goals (account_id) WHERE is_active;
