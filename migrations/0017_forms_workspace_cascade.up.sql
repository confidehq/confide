ALTER TABLE forms
    DROP CONSTRAINT forms_workspace_id_fkey,
    ADD CONSTRAINT forms_workspace_id_fkey
        FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE;
