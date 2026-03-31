DROP INDEX IF EXISTS idx_responses_read_at;
DROP INDEX IF EXISTS idx_responses_expires_at;

ALTER TABLE responses
  DROP COLUMN IF EXISTS read_at,
  DROP COLUMN IF EXISTS expires_at;

ALTER TABLE forms
  DROP COLUMN IF EXISTS burn_after_reading,
  DROP COLUMN IF EXISTS response_ttl_days;
