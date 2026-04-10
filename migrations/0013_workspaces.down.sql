-- Reverse Phase 1 workspace migration.

DROP INDEX IF EXISTS idx_workspace_members_account_id;
DROP INDEX IF EXISTS idx_forms_workspace_id;

-- Restore account_id on forms
ALTER TABLE forms ADD COLUMN account_id TEXT REFERENCES accounts(id);

UPDATE forms f
SET account_id = f.created_by_account_id;

ALTER TABLE forms
    ALTER COLUMN account_id SET NOT NULL,
    DROP COLUMN workspace_id,
    DROP COLUMN created_by_account_id;

CREATE INDEX idx_forms_account_id ON forms(account_id);

DROP TABLE IF EXISTS workspace_member_keys;
DROP TABLE IF EXISTS account_identity_keys;
DROP TABLE IF EXISTS workspace_invitations;
DROP TABLE IF EXISTS workspace_members;
DROP TABLE IF EXISTS workspaces;
