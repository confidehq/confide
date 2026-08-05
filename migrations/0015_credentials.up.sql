-- Move credential fields from accounts to a new credentials table (1-to-many).

CREATE TABLE IF NOT EXISTS credentials (
    id                 TEXT        PRIMARY KEY,
    account_id         TEXT        NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    credential_id      BYTEA       NOT NULL UNIQUE,
    public_key         BYTEA       NOT NULL,
    prf_salt           BYTEA       NOT NULL,
    wrapped_master_key BYTEA       NOT NULL,
    backup_eligible    BOOLEAN     NOT NULL DEFAULT FALSE,
    name               TEXT        NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Data migration + column drop. Runs only on first application: it reads the
-- accounts columns that the same block drops. Guard on their presence.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'accounts' AND column_name = 'credential_id'
    ) THEN
        -- Migrate existing single credentials into the new table.
        -- Use the account id as the credential row id (still unique since it's 1:1 now).
        INSERT INTO credentials
            (id, account_id, credential_id, public_key, prf_salt, wrapped_master_key,
             backup_eligible, name, created_at)
        SELECT
            id,
            id,
            credential_id,
            public_key,
            prf_salt,
            wrapped_master_key,
            backup_eligible,
            '',
            NOW()
        FROM accounts
        ON CONFLICT (id) DO NOTHING;
    END IF;
END $$;

-- Remove the credential columns that now live in the credentials table.
-- Unconditional: these columns must not exist after this migration, even if the
-- copy above was skipped because it had already run.
ALTER TABLE accounts
    DROP COLUMN IF EXISTS credential_id,
    DROP COLUMN IF EXISTS public_key,
    DROP COLUMN IF EXISTS prf_salt,
    DROP COLUMN IF EXISTS wrapped_master_key,
    DROP COLUMN IF EXISTS backup_eligible;
