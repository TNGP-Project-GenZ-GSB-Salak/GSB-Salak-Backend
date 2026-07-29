CREATE TABLE salak.products (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code            varchar(20) NOT NULL UNIQUE,
    name            varchar(100) NOT NULL,
    term_months     int NOT NULL CHECK (term_months IN (12, 24)),
    unit_price      numeric(10,2) NOT NULL CHECK (unit_price > 0),
    min_purchase    numeric(18,2) NOT NULL CHECK (min_purchase > 0),
    max_purchase    numeric(18,2) NOT NULL CHECK (max_purchase > 0),
    step_amount     numeric(18,2) NOT NULL DEFAULT 1000,
    is_active       boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CHECK (max_purchase >= min_purchase)
);
