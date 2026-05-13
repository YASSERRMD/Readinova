-- +goose Up

CREATE TABLE organisation_members (
    organisation_id uuid        NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
    user_id         uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role            text        NOT NULL CHECK (role IN ('owner','admin','executive','cio','risk','ops','viewer')),
    joined_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organisation_id, user_id)
);

CREATE TABLE invitations (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organisation_id uuid        NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
    email           text        NOT NULL,
    role            text        NOT NULL CHECK (role IN ('owner','admin','executive','cio','risk','ops','viewer')),
    token           text        NOT NULL UNIQUE DEFAULT encode(gen_random_bytes(32), 'hex'),
    expires_at      timestamptz NOT NULL DEFAULT now() + interval '7 days',
    accepted_at     timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX invitations_token_idx ON invitations (token);
CREATE INDEX invitations_email_idx ON invitations (email);

-- +goose Down
DROP TABLE IF EXISTS invitations;
DROP TABLE IF EXISTS organisation_members;
