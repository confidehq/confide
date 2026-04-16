-- Revert: drop updated_at columns and convert TIMESTAMPTZ back to DATE.

-- credentials
ALTER TABLE credentials DROP COLUMN updated_at;

-- workspace_member_keys
ALTER TABLE workspace_member_keys DROP COLUMN updated_at;

-- account_identity_keys
ALTER TABLE account_identity_keys DROP COLUMN updated_at;

-- workspace_invitations
ALTER TABLE workspace_invitations DROP COLUMN updated_at;

-- workspace_members
ALTER TABLE workspace_members DROP COLUMN updated_at;

-- workspaces
ALTER TABLE workspaces DROP COLUMN updated_at;

-- form_schema_versions
ALTER TABLE form_schema_versions DROP COLUMN updated_at;
ALTER TABLE form_schema_versions
    ALTER COLUMN created_at TYPE DATE USING created_at::DATE;
ALTER TABLE form_schema_versions
    ALTER COLUMN created_at DROP DEFAULT;

-- responses
ALTER TABLE responses DROP COLUMN updated_at;
ALTER TABLE responses
    ALTER COLUMN received_at TYPE DATE USING received_at::DATE;
ALTER TABLE responses
    ALTER COLUMN received_at DROP DEFAULT;

-- forms
ALTER TABLE forms
    ALTER COLUMN created_at TYPE DATE USING created_at::DATE,
    ALTER COLUMN updated_at TYPE DATE USING updated_at::DATE;
ALTER TABLE forms
    ALTER COLUMN created_at DROP DEFAULT,
    ALTER COLUMN updated_at DROP DEFAULT;

-- sessions
ALTER TABLE sessions DROP COLUMN updated_at;
ALTER TABLE sessions
    ALTER COLUMN created_at TYPE DATE USING created_at::DATE,
    ALTER COLUMN last_seen   TYPE DATE USING last_seen::DATE;
ALTER TABLE sessions
    ALTER COLUMN created_at DROP DEFAULT,
    ALTER COLUMN last_seen   DROP DEFAULT;

-- recovery_codes
ALTER TABLE recovery_codes DROP COLUMN updated_at;
ALTER TABLE recovery_codes
    ALTER COLUMN created_at TYPE DATE USING created_at::DATE;
ALTER TABLE recovery_codes
    ALTER COLUMN created_at DROP DEFAULT;

-- accounts
ALTER TABLE accounts DROP COLUMN updated_at;
ALTER TABLE accounts
    ALTER COLUMN created_at TYPE DATE USING created_at::DATE;
ALTER TABLE accounts
    ALTER COLUMN created_at DROP DEFAULT;
