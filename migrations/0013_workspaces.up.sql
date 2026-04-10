-- Phase 1: Workspace database foundation.
-- Creates all workspace tables, migrates existing data, and removes forms.account_id.

-- 1. workspaces
CREATE TABLE workspaces (
    id                     TEXT PRIMARY KEY,
    name                   TEXT NOT NULL,
    slug                   TEXT UNIQUE NOT NULL,
    stripe_customer_id     TEXT UNIQUE,
    stripe_subscription_id TEXT UNIQUE,
    plan                   TEXT NOT NULL DEFAULT 'free'
                             CHECK (plan IN ('free', 'pro')),
    plan_status            TEXT NOT NULL DEFAULT 'active'
                             CHECK (plan_status IN ('active', 'past_due', 'canceled')),
    plan_period_end        TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 2. workspace_members
CREATE TABLE workspace_members (
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    account_id   TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    role         TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    joined_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, account_id)
);

-- 3. workspace_invitations
CREATE TABLE workspace_invitations (
    id                    TEXT PRIMARY KEY,
    workspace_id          TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    invited_by_account_id TEXT NOT NULL REFERENCES accounts(id),
    email                 TEXT NOT NULL,
    role                  TEXT NOT NULL,
    token_hash            TEXT NOT NULL UNIQUE,
    expires_at            TIMESTAMPTZ NOT NULL,
    accepted_at           TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 4. account_identity_keys
CREATE TABLE account_identity_keys (
    account_id                   TEXT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    identity_public_key          BYTEA NOT NULL,
    wrapped_identity_private_key BYTEA NOT NULL,
    created_at                   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 5. workspace_member_keys — one wrapped workspace key per member
CREATE TABLE workspace_member_keys (
    workspace_id           TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    account_id             TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    wrapped_workspace_key  BYTEA NOT NULL,
    ephemeral_public_key   BYTEA NOT NULL,  -- 32-byte X25519 ephemeral public key
    granted_by_account_id  TEXT NOT NULL REFERENCES accounts(id),
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, account_id)
);

-- 6. Add workspace columns to forms (nullable during migration)
ALTER TABLE forms
    ADD COLUMN workspace_id          TEXT REFERENCES workspaces(id),
    ADD COLUMN created_by_account_id TEXT REFERENCES accounts(id);

-- 7. Data migration: create one personal workspace per account
CREATE TEMP TABLE _account_workspace_map AS
SELECT
    a.id                   AS account_id,
    gen_random_uuid()::text AS workspace_id
FROM accounts a;

INSERT INTO workspaces (id, name, slug, plan, plan_status, created_at)
SELECT workspace_id, 'Private', workspace_id, 'free', 'active', now()
FROM _account_workspace_map;

INSERT INTO workspace_members (workspace_id, account_id, role, joined_at)
SELECT workspace_id, account_id, 'owner', now()
FROM _account_workspace_map;

UPDATE forms f
SET workspace_id          = m.workspace_id,
    created_by_account_id = f.account_id
FROM _account_workspace_map m
WHERE m.account_id = f.account_id;

DROP TABLE _account_workspace_map;

-- 8. Enforce NOT NULL now that all rows are populated
ALTER TABLE forms
    ALTER COLUMN workspace_id          SET NOT NULL,
    ALTER COLUMN created_by_account_id SET NOT NULL;

-- 9. Drop the old account_id column
ALTER TABLE forms DROP COLUMN account_id;

CREATE INDEX idx_forms_workspace_id ON forms(workspace_id);
CREATE INDEX idx_workspace_members_account_id ON workspace_members(account_id);
