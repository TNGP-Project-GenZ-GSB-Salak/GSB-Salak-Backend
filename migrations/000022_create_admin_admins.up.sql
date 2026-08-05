-- A deliberately minimal admin identity: username + bcrypt hash, no
-- roles/permissions. Separate schema from "user".users since admin and
-- customer identity are unrelated concepts that happen to both be
-- username/password - never joined against, never mixed into the same
-- auth flow (see internal/platform/jwtutil's separate AdminSigner).
CREATE SCHEMA IF NOT EXISTS admin;

CREATE TABLE admin.admins (
    id uuid PRIMARY KEY,
    username text UNIQUE NOT NULL,
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
