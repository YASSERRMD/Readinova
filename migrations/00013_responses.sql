-- +goose Up

CREATE TABLE responses (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    assessment_id   uuid        NOT NULL REFERENCES assessments(id) ON DELETE CASCADE,
    question_id     uuid        NOT NULL REFERENCES questions(id),
    level           int         NOT NULL CHECK (level BETWEEN 0 AND 4),
    free_text       text,
    created_by      uuid        NOT NULL REFERENCES users(id),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (assessment_id, question_id)
);

ALTER TABLE responses ENABLE ROW LEVEL SECURITY;

CREATE TABLE response_evidence (
    id          uuid    PRIMARY KEY DEFAULT gen_random_uuid(),
    response_id uuid    NOT NULL REFERENCES responses(id) ON DELETE CASCADE,
    kind        text    NOT NULL CHECK (kind IN ('url','file_id','comment')),
    ref         text    NOT NULL
);

-- RLS: responses are scoped to the assessment's organisation.
CREATE POLICY responses_isolation ON responses
    USING (
        assessment_id IN (
            SELECT id FROM assessments WHERE organisation_id = current_org_id()
        )
    );

-- +goose Down
DROP TABLE IF EXISTS response_evidence;
DROP TABLE IF EXISTS responses;
