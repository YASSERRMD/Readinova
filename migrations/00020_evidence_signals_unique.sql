-- +goose Up
-- Add UNIQUE constraint on evidence_signals so the ON CONFLICT upsert in the
-- connector sync handler works correctly without creating duplicate rows.
ALTER TABLE evidence_signals
    ADD CONSTRAINT evidence_signals_org_connector_key_unique
    UNIQUE (organisation_id, connector_type, signal_key);

-- +goose Down
ALTER TABLE evidence_signals
    DROP CONSTRAINT IF EXISTS evidence_signals_org_connector_key_unique;
