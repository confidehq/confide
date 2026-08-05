CREATE TABLE IF NOT EXISTS accounts (
    id                      TEXT  PRIMARY KEY,
    created_at              DATE  NOT NULL,
    credential_id           BYTEA NOT NULL UNIQUE,
    public_key              BYTEA NOT NULL,
    prf_salt                BYTEA NOT NULL,
    wrapped_master_key      BYTEA NOT NULL,
    recovery_wrapped_master BYTEA NOT NULL,
    recovery_verifier       BYTEA NOT NULL
);

-- accounts.credential_id moves to the credentials table in 0015, so on a replay
-- against an evolved schema the column is gone and this index is obsolete.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'accounts' AND column_name = 'credential_id'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_accounts_credential_id ON accounts (credential_id);
    END IF;
END $$;
