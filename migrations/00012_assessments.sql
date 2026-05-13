-- +goose Up

CREATE TABLE assessments (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organisation_id uuid        NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
    framework_id    uuid        NOT NULL REFERENCES frameworks(id),
    title           text        NOT NULL,
    status          text        NOT NULL DEFAULT 'draft'
                                CHECK (status IN ('draft','in_progress','ready_to_score','scored','archived')),
    started_at      timestamptz,
    completed_at    timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE assessments ENABLE ROW LEVEL SECURITY;

CREATE POLICY assessments_isolation ON assessments
    USING (organisation_id = current_org_id());

CREATE TABLE assessment_assignments (
    assessment_id   uuid        NOT NULL REFERENCES assessments(id) ON DELETE CASCADE,
    user_id         uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role            text        NOT NULL,
    status          text        NOT NULL DEFAULT 'pending'
                                CHECK (status IN ('pending','in_progress','completed')),
    due_at          timestamptz,
    PRIMARY KEY (assessment_id, user_id)
);

CREATE TABLE question_assignments (
    assessment_id       uuid    NOT NULL REFERENCES assessments(id) ON DELETE CASCADE,
    question_id         uuid    NOT NULL REFERENCES questions(id),
    assigned_role       text    NOT NULL,
    assigned_user_id    uuid    REFERENCES users(id),
    PRIMARY KEY (assessment_id, question_id)
);

-- +goose Down
DROP TABLE IF EXISTS question_assignments;
DROP TABLE IF EXISTS assessment_assignments;
DROP TABLE IF EXISTS assessments;
