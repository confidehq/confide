-- Revert draft status and has_unpublished_changes.
UPDATE forms SET status = 'closed' WHERE status = 'draft';

ALTER TABLE forms DROP COLUMN has_unpublished_changes;
ALTER TABLE forms ALTER COLUMN status SET DEFAULT 'closed';
ALTER TABLE forms DROP CONSTRAINT IF EXISTS forms_status_check;
ALTER TABLE forms ADD CONSTRAINT forms_status_check CHECK (status IN ('open', 'closed'));
