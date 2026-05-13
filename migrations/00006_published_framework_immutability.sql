-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_framework_is_editable(target_framework_id uuid)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
    current_status text;
BEGIN
    SELECT status
    INTO current_status
    FROM frameworks
    WHERE id = target_framework_id;

    IF current_status = 'published' THEN
        RAISE EXCEPTION 'published framework % is immutable', target_framework_id
            USING ERRCODE = '23514';
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_dimension_is_editable()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM assert_framework_is_editable(COALESCE(NEW.framework_id, OLD.framework_id));
    RETURN COALESCE(NEW, OLD);
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_sub_dimension_is_editable()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    target_framework_id uuid;
BEGIN
    SELECT framework_id
    INTO target_framework_id
    FROM dimensions
    WHERE id = COALESCE(NEW.dimension_id, OLD.dimension_id);

    PERFORM assert_framework_is_editable(target_framework_id);
    RETURN COALESCE(NEW, OLD);
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_indicator_is_editable()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    target_framework_id uuid;
BEGIN
    SELECT d.framework_id
    INTO target_framework_id
    FROM sub_dimensions sd
    JOIN dimensions d ON d.id = sd.dimension_id
    WHERE sd.id = COALESCE(NEW.sub_dimension_id, OLD.sub_dimension_id);

    PERFORM assert_framework_is_editable(target_framework_id);
    RETURN COALESCE(NEW, OLD);
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_question_is_editable()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    target_framework_id uuid;
BEGIN
    SELECT d.framework_id
    INTO target_framework_id
    FROM indicators i
    JOIN sub_dimensions sd ON sd.id = i.sub_dimension_id
    JOIN dimensions d ON d.id = sd.dimension_id
    WHERE i.id = COALESCE(NEW.indicator_id, OLD.indicator_id);

    PERFORM assert_framework_is_editable(target_framework_id);
    RETURN COALESCE(NEW, OLD);
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_rubric_level_is_editable()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    target_framework_id uuid;
BEGIN
    SELECT d.framework_id
    INTO target_framework_id
    FROM questions q
    JOIN indicators i ON i.id = q.indicator_id
    JOIN sub_dimensions sd ON sd.id = i.sub_dimension_id
    JOIN dimensions d ON d.id = sd.dimension_id
    WHERE q.id = COALESCE(NEW.question_id, OLD.question_id);

    PERFORM assert_framework_is_editable(target_framework_id);
    RETURN COALESCE(NEW, OLD);
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_industry_override_is_editable()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM assert_framework_is_editable(COALESCE(NEW.framework_id, OLD.framework_id));
    RETURN COALESCE(NEW, OLD);
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER dimensions_published_framework_immutable
BEFORE INSERT OR UPDATE OR DELETE ON dimensions
FOR EACH ROW EXECUTE FUNCTION assert_dimension_is_editable();

CREATE TRIGGER sub_dimensions_published_framework_immutable
BEFORE INSERT OR UPDATE OR DELETE ON sub_dimensions
FOR EACH ROW EXECUTE FUNCTION assert_sub_dimension_is_editable();

CREATE TRIGGER indicators_published_framework_immutable
BEFORE INSERT OR UPDATE OR DELETE ON indicators
FOR EACH ROW EXECUTE FUNCTION assert_indicator_is_editable();

CREATE TRIGGER questions_published_framework_immutable
BEFORE INSERT OR UPDATE OR DELETE ON questions
FOR EACH ROW EXECUTE FUNCTION assert_question_is_editable();

CREATE TRIGGER rubric_levels_published_framework_immutable
BEFORE INSERT OR UPDATE OR DELETE ON rubric_levels
FOR EACH ROW EXECUTE FUNCTION assert_rubric_level_is_editable();

CREATE TRIGGER industry_weight_overrides_published_framework_immutable
BEFORE INSERT OR UPDATE OR DELETE ON industry_weight_overrides
FOR EACH ROW EXECUTE FUNCTION assert_industry_override_is_editable();

-- +goose Down
DROP TRIGGER IF EXISTS industry_weight_overrides_published_framework_immutable ON industry_weight_overrides;
DROP TRIGGER IF EXISTS rubric_levels_published_framework_immutable ON rubric_levels;
DROP TRIGGER IF EXISTS questions_published_framework_immutable ON questions;
DROP TRIGGER IF EXISTS indicators_published_framework_immutable ON indicators;
DROP TRIGGER IF EXISTS sub_dimensions_published_framework_immutable ON sub_dimensions;
DROP TRIGGER IF EXISTS dimensions_published_framework_immutable ON dimensions;

DROP FUNCTION IF EXISTS assert_industry_override_is_editable();
DROP FUNCTION IF EXISTS assert_rubric_level_is_editable();
DROP FUNCTION IF EXISTS assert_question_is_editable();
DROP FUNCTION IF EXISTS assert_indicator_is_editable();
DROP FUNCTION IF EXISTS assert_sub_dimension_is_editable();
DROP FUNCTION IF EXISTS assert_dimension_is_editable();
DROP FUNCTION IF EXISTS assert_framework_is_editable(uuid);
