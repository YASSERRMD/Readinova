-- +goose Up

CREATE TABLE scoring_runs (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    assessment_id   uuid        NOT NULL REFERENCES assessments(id) ON DELETE CASCADE,
    organisation_id uuid        NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
    triggered_by    uuid        NOT NULL REFERENCES users(id),
    status          text        NOT NULL DEFAULT 'pending'
                                CHECK (status IN ('pending','running','completed','failed')),
    result_json     jsonb,
    error_message   text,
    engine_version  text,
    started_at      timestamptz,
    completed_at    timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE scoring_runs ENABLE ROW LEVEL SECURITY;

CREATE POLICY scoring_runs_isolation ON scoring_runs
    USING (organisation_id = current_org_id());

-- Only one completed run per assessment (allow re-scoring by using latest).
CREATE INDEX idx_scoring_runs_assessment ON scoring_runs (assessment_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS scoring_runs;
