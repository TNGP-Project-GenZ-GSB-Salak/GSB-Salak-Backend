-- Redesigns salak.ticket_sequence from one global singleton (letter chosen
-- at random per holding, number an unbounded counter) into one cursor row
-- per product, keyed by product_id, with a bounded 7-digit number that
-- rolls over into the next Thai-consonant letter rather than growing
-- forever - see docs/GAPS.md §2.3. No backfill needed: no durable
-- holdings data exists (cmd/seed never creates holdings), so this is a
-- clean drop/recreate rather than a data migration.
CREATE EXTENSION IF NOT EXISTS btree_gist;

DROP TABLE IF EXISTS salak.ticket_sequence;

CREATE TABLE salak.ticket_sequence (
    product_id          uuid PRIMARY KEY REFERENCES salak.products(id),
    next_ticket_letter  varchar(1) NOT NULL DEFAULT 'ก',
    next_ticket_number  bigint NOT NULL DEFAULT 0 CHECK (next_ticket_number BETWEEN 0 AND 9999999),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

-- Belt-and-braces on top of the row-locked cursor: ticket ranges are
-- prize-determining (the official sales sheets award by งวด and
-- หมวดอักษร together), so a silent overlap would mean two customers
-- holding the same winning ticket. A plain UNIQUE on ticket_start alone
-- would miss two overlapping ranges with different starts (e.g. ก1..ก10
-- and ก5..ก14) - only a range-aware exclusion constraint catches those.
ALTER TABLE salak.holdings
    ADD CONSTRAINT holdings_no_overlapping_ranges
    EXCLUDE USING gist (
        product_id WITH =,
        ticket_letter WITH =,
        int8range(ticket_start, ticket_end, '[]') WITH &&
    );

-- Relax ticket_start's old ">0" check (written when a product's cursor
-- started at 1, the global singleton's old default): a fresh per-product
-- cursor now legitimately starts at 0 (ก0000000 is a real, valid ticket),
-- so 0 must be an allowed ticket_start.
ALTER TABLE salak.holdings DROP CONSTRAINT holdings_ticket_start_check;
ALTER TABLE salak.holdings ADD CONSTRAINT holdings_ticket_start_check CHECK (ticket_start >= 0);
