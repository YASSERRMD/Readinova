-- +goose Up

-- connector_configs: per-org connector settings
CREATE TABLE connector_configs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
    connector_type  TEXT NOT NULL,                 -- e.g. 'test', 'azure', 'aws'
    display_name    TEXT NOT NULL,
    credentials     JSONB NOT NULL DEFAULT '{}',   -- encrypted at app layer
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    last_sync_at    TIMESTAMPTZ,
    last_sync_error TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organisation_id, connector_type)
);

-- evidence_signals: raw signals collected from connectors
CREATE TABLE evidence_signals (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
    connector_type  TEXT NOT NULL,
    dimension_slug  TEXT NOT NULL,
    signal_key      TEXT NOT NULL,
    signal_value    JSONB NOT NULL,
    collected_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Index for fast per-org lookups
CREATE INDEX idx_evidence_signals_org ON evidence_signals (organisation_id, collected_at DESC);
CREATE INDEX idx_connector_configs_org ON connector_configs (organisation_id);

-- RLS
ALTER TABLE connector_configs ENABLE ROW LEVEL SECURITY;
ALTER TABLE evidence_signals  ENABLE ROW LEVEL SECURITY;

CREATE POLICY connector_configs_isolation ON connector_configs
    USING (organisation_id = current_org_id());

CREATE POLICY evidence_signals_isolation ON evidence_signals
    USING (organisation_id = current_org_id());

-- +goose Down
DROP TABLE IF EXISTS evidence_signals;
DROP TABLE IF EXISTS connector_configs;
