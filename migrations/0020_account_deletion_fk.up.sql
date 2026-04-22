-- Allow accounts to be fully deleted without FK violations.
-- invited_by_account_id: cascade-delete invitations when inviter is deleted.
-- granted_by_account_id: set to NULL when granter is deleted.
-- forms.created_by_account_id: set to NULL when creator is deleted.

ALTER TABLE workspace_invitations
    DROP CONSTRAINT workspace_invitations_invited_by_account_id_fkey,
    ADD CONSTRAINT workspace_invitations_invited_by_account_id_fkey
        FOREIGN KEY (invited_by_account_id) REFERENCES accounts(id) ON DELETE CASCADE;

ALTER TABLE workspace_member_keys
    DROP CONSTRAINT workspace_member_keys_granted_by_account_id_fkey,
    ALTER COLUMN granted_by_account_id DROP NOT NULL,
    ADD CONSTRAINT workspace_member_keys_granted_by_account_id_fkey
        FOREIGN KEY (granted_by_account_id) REFERENCES accounts(id) ON DELETE SET NULL;

ALTER TABLE forms
    DROP CONSTRAINT forms_created_by_account_id_fkey,
    ALTER COLUMN created_by_account_id DROP NOT NULL,
    ADD CONSTRAINT forms_created_by_account_id_fkey
        FOREIGN KEY (created_by_account_id) REFERENCES accounts(id) ON DELETE SET NULL;
