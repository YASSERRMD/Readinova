-- +goose Up

-- signing_keys: per-org Ed25519 key pairs (public key stored, private never persisted here)
CREATE TABLE signing_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE UNIQUE,
    public_key_b64  TEXT NOT NULL,  -- base64-encoded Ed25519 public key
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- audit_artefacts: immutable signed records of assessment results
CREATE TABLE audit_artefacts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
    assessment_id   UUID NOT NULL REFERENCES assessments(id) ON DELETE CASCADE,
    scoring_run_id  UUID REFERENCES scoring_runs(id),
    payload_json    JSONB NOT NULL,        -- canonical payload that was signed
    payload_hash    TEXT NOT NULL,         -- SHA-256 hex of payload_json bytes
    signature_b64   TEXT NOT NULL,         -- base64 Ed25519 signature
    public_key_b64  TEXT NOT NULL,         -- public key snapshot at signing time
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_artefacts_assessment ON audit_artefacts (assessment_id, created_at DESC);

ALTER TABLE signing_keys     ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_artefacts  ENABLE ROW LEVEL SECURITY;

CREATE POLICY signing_keys_isolation ON signing_keys
    USING (organisation_id = current_org_id());

CREATE POLICY audit_artefacts_isolation ON audit_artefacts
    USING (organisation_id = current_org_id());

-- +goose Down
DROP TABLE IF EXISTS audit_artefacts;
DROP TABLE IF EXISTS signing_keys;
