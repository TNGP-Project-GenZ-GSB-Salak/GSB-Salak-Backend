CREATE SCHEMA IF NOT EXISTS kapook;

-- Single blanket acceptance per user - no version/document tracking. UNIQUE
-- on user_id both enforces "one row per user" and backs the idempotent
-- ON CONFLICT DO NOTHING write GormTermsRepository.Accept uses, the same
-- shape as badge.user_badges' acquired_at (one semantic timestamp, no
-- separate created_at/updated_at pair, since this row is never updated).
CREATE TABLE kapook.terms_acceptances (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL UNIQUE REFERENCES "user".users(id),
    accepted_at  timestamptz NOT NULL DEFAULT now()
);
