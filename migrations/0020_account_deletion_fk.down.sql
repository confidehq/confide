ALTER TABLE forms
    DROP CONSTRAINT forms_created_by_account_id_fkey,
    ALTER COLUMN created_by_account_id SET NOT NULL,
    ADD CONSTRAINT forms_created_by_account_id_fkey
        FOREIGN KEY (created_by_account_id) REFERENCES accounts(id);

ALTER TABLE workspace_member_keys
    DROP CONSTRAINT workspace_member_keys_granted_by_account_id_fkey,
    ALTER COLUMN granted_by_account_id SET NOT NULL,
    ADD CONSTRAINT workspace_member_keys_granted_by_account_id_fkey
        FOREIGN KEY (granted_by_account_id) REFERENCES accounts(id);

ALTER TABLE workspace_invitations
    DROP CONSTRAINT workspace_invitations_invited_by_account_id_fkey,
    ADD CONSTRAINT workspace_invitations_invited_by_account_id_fkey
        FOREIGN KEY (invited_by_account_id) REFERENCES accounts(id);
