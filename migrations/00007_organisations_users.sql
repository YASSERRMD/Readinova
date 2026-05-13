-- +goose Up

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE organisations (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            text        NOT NULL UNIQUE,
    name            text        NOT NULL,
    country_code    char(2)     NOT NULL,
    sector          text        NOT NULL,
    size_band       text        NOT NULL CHECK (size_band IN ('1-50','51-250','251-1000','1001+')),
    regulatory_regimes jsonb    NOT NULL DEFAULT '[]',
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    email           text        NOT NULL UNIQUE,
    hashed_password text        NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    last_login_at   timestamptz
);

CREATE INDEX users_email_idx ON users (email);

-- +goose Down
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS organisations;
