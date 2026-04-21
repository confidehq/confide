ALTER TABLE workspaces DROP CONSTRAINT workspaces_plan_check;
ALTER TABLE workspaces ADD CONSTRAINT workspaces_plan_check
    CHECK (plan IN ('free', 'pro'));
