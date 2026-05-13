-- +goose Up

CREATE TABLE perception_gap_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    assessment_id   UUID NOT NULL REFERENCES assessments(id) ON DELETE CASCADE,
    organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
    layer_a_score   NUMERIC(6,2) NOT NULL,  -- self-assessment composite (S)
    layer_b_score   NUMERIC(6,2) NOT NULL,  -- evidence composite (E)
    gap_score       NUMERIC(6,2) NOT NULL,  -- S - E  (P)
    master_composite NUMERIC(6,2) NOT NULL, -- 0.4S + 0.5E + 0.1*(100-|P|)
    result_json     JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_perception_gap_runs_assessment ON perception_gap_runs (assessment_id, created_at DESC);

ALTER TABLE perception_gap_runs ENABLE ROW LEVEL SECURITY;

CREATE POLICY perception_gap_isolation ON perception_gap_runs
    USING (organisation_id = current_org_id());

-- +goose Down
DROP TABLE IF EXISTS perception_gap_runs;
