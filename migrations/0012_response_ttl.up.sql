ALTER TABLE forms
  ADD COLUMN IF NOT EXISTS response_ttl_days  INTEGER,
  ADD COLUMN IF NOT EXISTS burn_after_reading BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE responses
  ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS read_at    TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_responses_expires_at ON responses (expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_responses_read_at    ON responses (read_at)    WHERE read_at    IS NOT NULL;
