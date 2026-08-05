CREATE TABLE IF NOT EXISTS sessions (
    id          TEXT  PRIMARY KEY,
    account_id  TEXT  NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    token_hash  BYTEA NOT NULL UNIQUE,
    created_at  DATE  NOT NULL,
    last_seen   DATE  NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_account_id ON sessions (account_id);
CREATE INDEX IF NOT EXISTS idx_sessions_token_hash ON sessions (token_hash);
