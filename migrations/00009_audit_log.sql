-- +goose Up

CREATE TABLE audit_log (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organisation_id uuid        REFERENCES organisations(id) ON DELETE SET NULL,
    user_id         uuid        REFERENCES users(id) ON DELETE SET NULL,
    action          text        NOT NULL,
    target_type     text,
    target_id       text,
    metadata        jsonb       NOT NULL DEFAULT '{}',
    occurred_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_log_org_idx  ON audit_log (organisation_id, occurred_at DESC);
CREATE INDEX audit_log_user_idx ON audit_log (user_id, occurred_at DESC);

-- +goose Down
DROP TABLE IF EXISTS audit_log;
