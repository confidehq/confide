-- Postgres has no ADD CONSTRAINT IF NOT EXISTS; DROP IF EXISTS + ADD is the
-- idempotent equivalent.
ALTER TABLE forms
    DROP CONSTRAINT IF EXISTS forms_workspace_id_fkey,
    ADD CONSTRAINT forms_workspace_id_fkey
        FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE;
