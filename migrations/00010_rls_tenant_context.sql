-- +goose Up

-- Enable RLS on all tenant-scoped tables.
ALTER TABLE organisation_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE invitations           ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_log             ENABLE ROW LEVEL SECURITY;

-- Helper: set the current org context for this session.
CREATE OR REPLACE FUNCTION set_tenant_context(p_org_id uuid) RETURNS void
    LANGUAGE plpgsql AS $$
BEGIN
    PERFORM set_config('app.current_org', p_org_id::text, true);
END;
$$;

-- Helper: read the current org context; returns NULL if not set or not a valid UUID.
CREATE OR REPLACE FUNCTION current_org_id() RETURNS uuid
    LANGUAGE plpgsql STABLE AS $$
BEGIN
    RETURN NULLIF(current_setting('app.current_org', true), '')::uuid;
EXCEPTION
    WHEN invalid_text_representation THEN RETURN NULL;
END;
$$;

-- RLS policies: organisation_members
CREATE POLICY org_members_isolation ON organisation_members
    USING (organisation_id = current_org_id());

-- RLS policies: invitations
CREATE POLICY invitations_isolation ON invitations
    USING (organisation_id = current_org_id());

-- RLS policies: audit_log
CREATE POLICY audit_log_isolation ON audit_log
    USING (organisation_id = current_org_id());

-- +goose Down
DROP POLICY IF EXISTS audit_log_isolation    ON audit_log;
DROP POLICY IF EXISTS invitations_isolation  ON invitations;
DROP POLICY IF EXISTS org_members_isolation  ON organisation_members;
DROP FUNCTION IF EXISTS current_org_id();
DROP FUNCTION IF EXISTS set_tenant_context(uuid);
ALTER TABLE audit_log             DISABLE ROW LEVEL SECURITY;
ALTER TABLE invitations           DISABLE ROW LEVEL SECURITY;
ALTER TABLE organisation_members  DISABLE ROW LEVEL SECURITY;
