-- Allow 'org' as a valid plan value in preparation for the Organization tier.
ALTER TABLE workspaces DROP CONSTRAINT IF EXISTS workspaces_plan_check;
ALTER TABLE workspaces ADD CONSTRAINT workspaces_plan_check
    CHECK (plan IN ('free', 'pro', 'org'));
