-- +goose Up
CREATE TABLE sub_dimensions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    dimension_id uuid NOT NULL REFERENCES dimensions(id) ON DELETE CASCADE,
    slug text NOT NULL,
    name text NOT NULL,
    description text NOT NULL,
    weight_within_dimension numeric(5,4) NOT NULL CHECK (weight_within_dimension >= 0.0000 AND weight_within_dimension <= 1.0000),
    display_order integer NOT NULL CHECK (display_order > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT sub_dimensions_dimension_slug_unique UNIQUE (dimension_id, slug),
    CONSTRAINT sub_dimensions_dimension_order_unique UNIQUE (dimension_id, display_order)
);

CREATE TABLE indicators (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    sub_dimension_id uuid NOT NULL REFERENCES sub_dimensions(id) ON DELETE CASCADE,
    slug text NOT NULL,
    name text NOT NULL,
    description text NOT NULL,
    display_order integer NOT NULL CHECK (display_order > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT indicators_sub_dimension_slug_unique UNIQUE (sub_dimension_id, slug),
    CONSTRAINT indicators_sub_dimension_order_unique UNIQUE (sub_dimension_id, display_order)
);

-- +goose Down
DROP TABLE IF EXISTS indicators;
DROP TABLE IF EXISTS sub_dimensions;
