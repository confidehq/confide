ALTER TABLE forms
  ADD COLUMN response_ttl_days  INTEGER,
  ADD COLUMN burn_after_reading BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE responses
  ADD COLUMN expires_at TIMESTAMPTZ,
  ADD COLUMN read_at    TIMESTAMPTZ;

CREATE INDEX idx_responses_expires_at ON responses (expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_responses_read_at    ON responses (read_at)    WHERE read_at    IS NOT NULL;
