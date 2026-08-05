-- Records why an unattended auto-purchase attempt hasn't succeeded yet -
-- the worker already retries a failing goal forever (see internal/kapook/
-- worker's own doc comment), but until now that retrying was invisible:
-- no persisted trace beyond a stdout log line, and no way for the client
-- to tell "still within a normal tick or two" apart from "has been failing
-- the same way for the last hour". auto_purchase_attempts/last_error/
-- last_attempted_at are reset to zero/NULL the moment a purchase succeeds
-- (UpdateAfterPurchase) - they only ever describe the *current* unresolved
-- failure streak, never a goal's full history.
ALTER TABLE kapook.kapook_goals
    ADD COLUMN auto_purchase_attempts integer NOT NULL DEFAULT 0,
    ADD COLUMN auto_purchase_last_error text,
    ADD COLUMN auto_purchase_last_attempted_at timestamptz;
