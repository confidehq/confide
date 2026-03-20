CREATE TABLE accounts (
    id                      TEXT  PRIMARY KEY,
    created_at              DATE  NOT NULL,
    credential_id           BYTEA NOT NULL UNIQUE,
    public_key              BYTEA NOT NULL,
    prf_salt                BYTEA NOT NULL,
    wrapped_master_key      BYTEA NOT NULL,
    recovery_wrapped_master BYTEA NOT NULL,
    recovery_verifier       BYTEA NOT NULL
);

CREATE INDEX idx_accounts_credential_id ON accounts (credential_id);
