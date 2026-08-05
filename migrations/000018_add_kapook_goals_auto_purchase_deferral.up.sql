-- Nullable, cleared once a purchase succeeds - the worker persists this when
-- a draw-day guard rejects an otherwise-due auto-purchase, so the client can
-- learn why nothing happened instead of watching a "processing" state that
-- never resolves for the whole draw day. Also lets ClaimDueGoals skip a
-- deferred goal until its recorded retry date instead of re-claiming it
-- (and re-hitting the same draw-day rejection) every tick.
ALTER TABLE kapook.kapook_goals
    ADD COLUMN auto_purchase_deferred_until date;
