-- Restore credential columns on accounts
ALTER TABLE accounts
    ADD COLUMN credential_id      BYTEA,
    ADD COLUMN public_key         BYTEA,
    ADD COLUMN prf_salt           BYTEA,
    ADD COLUMN wrapped_master_key BYTEA,
    ADD COLUMN backup_eligible    BOOLEAN NOT NULL DEFAULT FALSE;

-- Restore single credential per account (take the oldest one)
UPDATE accounts a
SET
    credential_id      = c.credential_id,
    public_key         = c.public_key,
    prf_salt           = c.prf_salt,
    wrapped_master_key = c.wrapped_master_key,
    backup_eligible    = c.backup_eligible
FROM (
    SELECT DISTINCT ON (account_id) *
    FROM credentials
    ORDER BY account_id, created_at ASC
) c
WHERE a.id = c.account_id;

-- Restore NOT NULL after backfill
ALTER TABLE accounts
    ALTER COLUMN credential_id      SET NOT NULL,
    ALTER COLUMN public_key         SET NOT NULL,
    ALTER COLUMN prf_salt           SET NOT NULL,
    ALTER COLUMN wrapped_master_key SET NOT NULL;

DROP TABLE credentials;
