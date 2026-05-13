-- +goose Up
CREATE TABLE industry_weight_overrides (
    framework_id uuid NOT NULL REFERENCES frameworks(id) ON DELETE CASCADE,
    industry_slug text NOT NULL,
    dimension_id uuid NOT NULL REFERENCES dimensions(id) ON DELETE CASCADE,
    override_weight numeric(5,4) NOT NULL CHECK (override_weight >= 0.0000 AND override_weight <= 1.0000),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (framework_id, industry_slug, dimension_id)
);

-- +goose Down
DROP TABLE IF EXISTS industry_weight_overrides;
