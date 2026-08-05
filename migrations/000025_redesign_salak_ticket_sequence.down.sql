ALTER TABLE salak.holdings
    DROP CONSTRAINT IF EXISTS holdings_no_overlapping_ranges;

ALTER TABLE salak.holdings DROP CONSTRAINT IF EXISTS holdings_ticket_start_check;
ALTER TABLE salak.holdings ADD CONSTRAINT holdings_ticket_start_check CHECK (ticket_start > 0);

DROP TABLE IF EXISTS salak.ticket_sequence;

CREATE TABLE salak.ticket_sequence (
    id                  int PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    next_ticket_number  bigint NOT NULL DEFAULT 1,
    updated_at          timestamptz NOT NULL DEFAULT now()
);

INSERT INTO salak.ticket_sequence (id, next_ticket_number) VALUES (1, 1);

DROP EXTENSION IF EXISTS btree_gist;
