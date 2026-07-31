CREATE SCHEMA IF NOT EXISTS badge;

CREATE TABLE badge.badges (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code        varchar(50) NOT NULL UNIQUE,
    name        varchar(100) NOT NULL,
    image_url   text NOT NULL,
    weight      numeric(10,4) NOT NULL CHECK (weight > 0),
    is_active   boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE badge.salak_badges (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    salak_id              uuid NOT NULL UNIQUE REFERENCES salak.holdings(id),
    badge_id              uuid NOT NULL REFERENCES badge.badges(id),
    weight_at_assignment  numeric(10,4) NOT NULL CHECK (weight_at_assignment > 0),
    assigned_at           timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_salak_badges_badge_id ON badge.salak_badges (badge_id);
