CREATE TABLE salak.draw_dates (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  uuid NOT NULL REFERENCES salak.products(id),
    draw_date   date NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    -- The unique index this constraint creates already has (product_id,
    -- draw_date) as its leading columns, so it doubles as the lookup index
    -- the guard needs ("is product X closed on date Y") - no separate index.
    UNIQUE (product_id, draw_date)
);
