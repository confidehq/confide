-- Upgrade all audit DATE columns to TIMESTAMPTZ and add updated_at to every table.

-- ── accounts ──────────────────────────────────────────────────────────────────
ALTER TABLE accounts
    ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at::TIMESTAMPTZ;
ALTER TABLE accounts
    ALTER COLUMN created_at SET DEFAULT NOW();
ALTER TABLE accounts
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
UPDATE accounts SET updated_at = created_at;

-- ── recovery_codes ────────────────────────────────────────────────────────────
ALTER TABLE recovery_codes
    ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at::TIMESTAMPTZ;
ALTER TABLE recovery_codes
    ALTER COLUMN created_at SET DEFAULT NOW();
ALTER TABLE recovery_codes
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
UPDATE recovery_codes SET updated_at = created_at;

-- ── sessions ──────────────────────────────────────────────────────────────────
ALTER TABLE sessions
    ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at::TIMESTAMPTZ,
    ALTER COLUMN last_seen   TYPE TIMESTAMPTZ USING last_seen::TIMESTAMPTZ;
ALTER TABLE sessions
    ALTER COLUMN created_at SET DEFAULT NOW(),
    ALTER COLUMN last_seen   SET DEFAULT NOW();
ALTER TABLE sessions
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
UPDATE sessions SET updated_at = last_seen;

-- ── forms ─────────────────────────────────────────────────────────────────────
ALTER TABLE forms
    ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at::TIMESTAMPTZ,
    ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at::TIMESTAMPTZ;
ALTER TABLE forms
    ALTER COLUMN created_at SET DEFAULT NOW(),
    ALTER COLUMN updated_at SET DEFAULT NOW();

-- ── responses ─────────────────────────────────────────────────────────────────
ALTER TABLE responses
    ALTER COLUMN received_at TYPE TIMESTAMPTZ USING received_at::TIMESTAMPTZ;
ALTER TABLE responses
    ALTER COLUMN received_at SET DEFAULT NOW();
ALTER TABLE responses
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
UPDATE responses SET updated_at = COALESCE(read_at, received_at);

-- ── form_schema_versions ──────────────────────────────────────────────────────
ALTER TABLE form_schema_versions
    ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at::TIMESTAMPTZ;
ALTER TABLE form_schema_versions
    ALTER COLUMN created_at SET DEFAULT NOW();
ALTER TABLE form_schema_versions
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
UPDATE form_schema_versions SET updated_at = created_at;

-- ── workspaces ────────────────────────────────────────────────────────────────
ALTER TABLE workspaces
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
UPDATE workspaces SET updated_at = created_at;

-- ── workspace_members ─────────────────────────────────────────────────────────
ALTER TABLE workspace_members
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
UPDATE workspace_members SET updated_at = joined_at;

-- ── workspace_invitations ─────────────────────────────────────────────────────
ALTER TABLE workspace_invitations
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
UPDATE workspace_invitations SET updated_at = COALESCE(accepted_at, created_at);

-- ── account_identity_keys ─────────────────────────────────────────────────────
ALTER TABLE account_identity_keys
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
UPDATE account_identity_keys SET updated_at = created_at;

-- ── workspace_member_keys ─────────────────────────────────────────────────────
ALTER TABLE workspace_member_keys
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
UPDATE workspace_member_keys SET updated_at = created_at;

-- ── credentials ───────────────────────────────────────────────────────────────
ALTER TABLE credentials
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
UPDATE credentials SET updated_at = created_at;
