ALTER TABLE forms
  ADD COLUMN IF NOT EXISTS notification_from    TEXT,
  ADD COLUMN IF NOT EXISTS notification_subject TEXT;
