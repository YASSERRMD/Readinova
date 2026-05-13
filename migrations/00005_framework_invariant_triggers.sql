-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_dimension_weights_sum()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    target_framework_id uuid;
    total numeric;
BEGIN
    IF TG_TABLE_NAME = 'frameworks' THEN
        target_framework_id := COALESCE(NEW.id, OLD.id);
    ELSE
        target_framework_id := COALESCE(NEW.framework_id, OLD.framework_id);
    END IF;

    SELECT COALESCE(sum(default_weight), 0)
    INTO total
    FROM dimensions
    WHERE framework_id = target_framework_id;

    IF total <> 1.0000 THEN
        RAISE EXCEPTION 'dimension weights for framework % must sum to 1.0000, got %', target_framework_id, total
            USING ERRCODE = '23514';
    END IF;

    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER dimensions_weight_sum_check
AFTER INSERT OR UPDATE OR DELETE ON dimensions
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION assert_dimension_weights_sum();

CREATE CONSTRAINT TRIGGER frameworks_dimension_weight_sum_check
AFTER INSERT OR UPDATE ON frameworks
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION assert_dimension_weights_sum();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_sub_dimension_weights_sum()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    target_dimension_id uuid;
    total numeric;
BEGIN
    IF TG_TABLE_NAME = 'dimensions' THEN
        target_dimension_id := COALESCE(NEW.id, OLD.id);
    ELSE
        target_dimension_id := COALESCE(NEW.dimension_id, OLD.dimension_id);
    END IF;

    SELECT COALESCE(sum(weight_within_dimension), 0)
    INTO total
    FROM sub_dimensions
    WHERE dimension_id = target_dimension_id;

    IF total <> 1.0000 THEN
        RAISE EXCEPTION 'sub_dimension weights for dimension % must sum to 1.0000, got %', target_dimension_id, total
            USING ERRCODE = '23514';
    END IF;

    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER sub_dimensions_weight_sum_check
AFTER INSERT OR UPDATE OR DELETE ON sub_dimensions
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION assert_sub_dimension_weights_sum();

CREATE CONSTRAINT TRIGGER dimensions_sub_dimension_weight_sum_check
AFTER INSERT OR UPDATE ON dimensions
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION assert_sub_dimension_weights_sum();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_rubric_levels_cover_zero_to_four()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    target_question_id uuid;
    level_count integer;
    level_sum integer;
BEGIN
    IF TG_TABLE_NAME = 'questions' THEN
        target_question_id := COALESCE(NEW.id, OLD.id);
    ELSE
        target_question_id := COALESCE(NEW.question_id, OLD.question_id);
    END IF;

    SELECT count(*), COALESCE(sum(level), 0)
    INTO level_count, level_sum
    FROM rubric_levels
    WHERE question_id = target_question_id;

    IF level_count <> 5 OR level_sum <> 10 THEN
        RAISE EXCEPTION 'rubric_levels for question % must cover levels 0 through 4 exactly', target_question_id
            USING ERRCODE = '23514';
    END IF;

    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER rubric_levels_cover_zero_to_four_check
AFTER INSERT OR UPDATE OR DELETE ON rubric_levels
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION assert_rubric_levels_cover_zero_to_four();

CREATE CONSTRAINT TRIGGER questions_rubric_levels_cover_zero_to_four_check
AFTER INSERT OR UPDATE ON questions
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION assert_rubric_levels_cover_zero_to_four();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION assert_industry_override_weights_sum()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    target_framework_id uuid;
    target_industry_slug text;
    total numeric;
BEGIN
    target_framework_id := COALESCE(NEW.framework_id, OLD.framework_id);
    target_industry_slug := COALESCE(NEW.industry_slug, OLD.industry_slug);

    SELECT COALESCE(sum(override_weight), 0)
    INTO total
    FROM industry_weight_overrides
    WHERE framework_id = target_framework_id
      AND industry_slug = target_industry_slug;

    IF total <> 1.0000 THEN
        RAISE EXCEPTION 'industry override weights for framework % and industry % must sum to 1.0000, got %',
            target_framework_id, target_industry_slug, total
            USING ERRCODE = '23514';
    END IF;

    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER industry_override_weights_sum_check
AFTER INSERT OR UPDATE OR DELETE ON industry_weight_overrides
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION assert_industry_override_weights_sum();

-- +goose Down
DROP TRIGGER IF EXISTS industry_override_weights_sum_check ON industry_weight_overrides;
DROP FUNCTION IF EXISTS assert_industry_override_weights_sum();

DROP TRIGGER IF EXISTS questions_rubric_levels_cover_zero_to_four_check ON questions;
DROP TRIGGER IF EXISTS rubric_levels_cover_zero_to_four_check ON rubric_levels;
DROP FUNCTION IF EXISTS assert_rubric_levels_cover_zero_to_four();

DROP TRIGGER IF EXISTS dimensions_sub_dimension_weight_sum_check ON dimensions;
DROP TRIGGER IF EXISTS sub_dimensions_weight_sum_check ON sub_dimensions;
DROP FUNCTION IF EXISTS assert_sub_dimension_weights_sum();

DROP TRIGGER IF EXISTS frameworks_dimension_weight_sum_check ON frameworks;
DROP TRIGGER IF EXISTS dimensions_weight_sum_check ON dimensions;
DROP FUNCTION IF EXISTS assert_dimension_weights_sum();
