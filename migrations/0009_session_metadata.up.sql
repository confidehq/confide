ALTER TABLE sessions
    ADD COLUMN credential_id BYTEA NOT NULL DEFAULT '', 
    ADD COLUMN user_agent    TEXT  NOT NULL DEFAULT '';
