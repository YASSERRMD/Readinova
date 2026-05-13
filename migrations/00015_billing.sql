-- +goose Up

CREATE TABLE subscriptions (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organisation_id     uuid        NOT NULL UNIQUE REFERENCES organisations(id) ON DELETE CASCADE,
    stripe_customer_id  text        UNIQUE,
    stripe_sub_id       text        UNIQUE,
    tier                text        NOT NULL DEFAULT 'free'
                                    CHECK (tier IN ('free','starter','growth','enterprise')),
    status              text        NOT NULL DEFAULT 'active'
                                    CHECK (status IN ('active','past_due','canceled','trialing')),
    current_period_end  timestamptz,
    cancel_at_period_end boolean    NOT NULL DEFAULT false,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

-- Seed a free subscription for every existing org.
INSERT INTO subscriptions (organisation_id)
SELECT id FROM organisations
ON CONFLICT (organisation_id) DO NOTHING;

-- Ensure new orgs always get a free subscription via trigger.
CREATE OR REPLACE FUNCTION create_free_subscription()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO subscriptions (organisation_id) VALUES (NEW.id)
    ON CONFLICT DO NOTHING;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_create_free_subscription
AFTER INSERT ON organisations
FOR EACH ROW EXECUTE FUNCTION create_free_subscription();

ALTER TABLE subscriptions ENABLE ROW LEVEL SECURITY;

CREATE POLICY subscriptions_isolation ON subscriptions
    USING (organisation_id = current_org_id());

-- +goose Down
DROP TRIGGER IF EXISTS trg_create_free_subscription ON organisations;
DROP FUNCTION IF EXISTS create_free_subscription();
DROP TABLE IF EXISTS subscriptions;
