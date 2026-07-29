CREATE TABLE salak.holdings (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id          uuid NOT NULL REFERENCES account.accounts(id),
    product_id          uuid NOT NULL REFERENCES salak.products(id),
    units               bigint NOT NULL CHECK (units > 0),
    ticket_start        bigint NOT NULL CHECK (ticket_start > 0),
    ticket_end          bigint NOT NULL CHECK (ticket_end > ticket_start),
    purchase_amount     numeric(18,2) NOT NULL CHECK (purchase_amount > 0),
    purchase_date       date NOT NULL DEFAULT CURRENT_DATE,
    maturity_date       date NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    CHECK (ticket_end - ticket_start + 1 = units)
);

CREATE INDEX idx_holdings_account_id ON salak.holdings (account_id);

CREATE TABLE salak.ticket_sequence (
    id                  int PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    next_ticket_number  bigint NOT NULL DEFAULT 1,
    updated_at          timestamptz NOT NULL DEFAULT now()
);

INSERT INTO salak.ticket_sequence (id, next_ticket_number) VALUES (1, 1);
