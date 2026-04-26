ALTER TABLE forms
  DROP COLUMN IF EXISTS notification_from,
  DROP COLUMN IF EXISTS notification_subject;
