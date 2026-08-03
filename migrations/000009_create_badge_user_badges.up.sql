CREATE TABLE badge.user_badges (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES "user".users(id),
    badge_id     uuid NOT NULL REFERENCES badge.badges(id),
    acquired_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, badge_id)
);

CREATE INDEX idx_user_badges_badge_id ON badge.user_badges (badge_id);
