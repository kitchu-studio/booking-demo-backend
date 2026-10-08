CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT        NOT NULL,
    name          TEXT        NOT NULL DEFAULT '',
    password_hash TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email_lower_idx ON users (lower(email));
