-- +goose Up
CREATE TABLE questions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    indicator_id uuid NOT NULL REFERENCES indicators(id) ON DELETE CASCADE,
    slug text NOT NULL,
    prompt text NOT NULL,
    target_role text NOT NULL CHECK (target_role IN ('executive', 'cio', 'risk', 'ops', 'any')),
    regulatory_references jsonb NOT NULL DEFAULT '{}'::jsonb,
    display_order integer NOT NULL CHECK (display_order > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT questions_indicator_slug_unique UNIQUE (indicator_id, slug),
    CONSTRAINT questions_indicator_order_unique UNIQUE (indicator_id, display_order),
    CONSTRAINT questions_regulatory_references_object CHECK (jsonb_typeof(regulatory_references) = 'object')
);

CREATE TABLE rubric_levels (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id uuid NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    level integer NOT NULL CHECK (level >= 0 AND level <= 4),
    label text NOT NULL CHECK (label IN ('Absent', 'Ad Hoc', 'Defined', 'Managed', 'Optimised')),
    description text NOT NULL,
    score numeric(5,2) NOT NULL CHECK (score >= 0 AND score <= 100),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT rubric_levels_question_level_unique UNIQUE (question_id, level),
    CONSTRAINT rubric_levels_level_label CHECK (
        (level = 0 AND label = 'Absent')
        OR (level = 1 AND label = 'Ad Hoc')
        OR (level = 2 AND label = 'Defined')
        OR (level = 3 AND label = 'Managed')
        OR (level = 4 AND label = 'Optimised')
    )
);

-- +goose Down
DROP TABLE IF EXISTS rubric_levels;
DROP TABLE IF EXISTS questions;
