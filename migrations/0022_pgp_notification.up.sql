ALTER TABLE forms
  ADD COLUMN IF NOT EXISTS notification_email TEXT,
  ADD COLUMN IF NOT EXISTS pgp_public_key     TEXT;
