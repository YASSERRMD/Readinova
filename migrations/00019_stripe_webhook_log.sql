-- +goose Up
-- Stripe webhook event log: idempotency guard and audit trail.
CREATE TABLE stripe_webhook_events (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stripe_event_id TEXT NOT NULL UNIQUE,          -- prevents duplicate processing
    event_type    TEXT NOT NULL,
    payload_json  JSONB,
    processed_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    error_message TEXT                              -- set when processing failed
);

CREATE INDEX stripe_webhook_events_type_idx ON stripe_webhook_events (event_type);
CREATE INDEX stripe_webhook_events_processed_idx ON stripe_webhook_events (processed_at DESC);

-- +goose Down
DROP TABLE IF EXISTS stripe_webhook_events;
