-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE frameworks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug text NOT NULL,
    name text NOT NULL,
    version_major integer NOT NULL CHECK (version_major >= 0),
    version_minor integer NOT NULL CHECK (version_minor >= 0),
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    published_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT frameworks_slug_major_unique UNIQUE (slug, version_major),
    CONSTRAINT frameworks_published_at_required CHECK (
        (status = 'published' AND published_at IS NOT NULL)
        OR (status <> 'published')
    )
);

CREATE TABLE dimensions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    framework_id uuid NOT NULL REFERENCES frameworks(id) ON DELETE CASCADE,
    slug text NOT NULL,
    name text NOT NULL,
    description text NOT NULL,
    default_weight numeric(5,4) NOT NULL CHECK (default_weight >= 0.0000 AND default_weight <= 1.0000),
    display_order integer NOT NULL CHECK (display_order > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT dimensions_framework_slug_unique UNIQUE (framework_id, slug),
    CONSTRAINT dimensions_framework_order_unique UNIQUE (framework_id, display_order)
);

-- +goose Down
DROP TABLE IF EXISTS dimensions;
DROP TABLE IF EXISTS frameworks;
