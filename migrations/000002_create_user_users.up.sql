CREATE TABLE "user".users (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username        varchar(50) NOT NULL UNIQUE,
    password_hash   varchar(255) NOT NULL,
    full_name       varchar(150) NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
