-- Move credential fields from accounts to a new credentials table (1-to-many).

CREATE TABLE credentials (
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
FROM accounts;

-- Remove the credential columns that now live in the credentials table.
ALTER TABLE accounts
    DROP COLUMN credential_id,
    DROP COLUMN public_key,
    DROP COLUMN prf_salt,
    DROP COLUMN wrapped_master_key,
    DROP COLUMN backup_eligible;
