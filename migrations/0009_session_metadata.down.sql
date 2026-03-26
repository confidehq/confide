ALTER TABLE sessions
    DROP COLUMN IF EXISTS credential_id,
    DROP COLUMN IF EXISTS user_agent;
