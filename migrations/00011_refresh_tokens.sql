-- +goose Up

CREATE TABLE refresh_tokens (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organisation_id uuid        NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
    token_hash      text        NOT NULL UNIQUE,
    expires_at      timestamptz NOT NULL DEFAULT now() + interval '30 days',
    revoked_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_hash_idx ON refresh_tokens (token_hash);
CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id);

-- +goose Down
DROP TABLE IF EXISTS refresh_tokens;
