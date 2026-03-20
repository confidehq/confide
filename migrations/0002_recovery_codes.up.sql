CREATE TABLE recovery_codes (
    id          TEXT    PRIMARY KEY,
    account_id  TEXT    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    code_hash   BYTEA   NOT NULL,
    used        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  DATE    NOT NULL
);

CREATE INDEX idx_recovery_codes_account_id ON recovery_codes (account_id);
CREATE INDEX idx_recovery_codes_hash       ON recovery_codes (code_hash);
