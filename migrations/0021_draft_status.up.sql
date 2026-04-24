-- Introduce 'draft' status for forms that have never been published.
-- New forms default to 'draft'; the publish endpoint is the only way to become 'open'.
-- has_unpublished_changes tracks whether the owner schema has diverged from the render schema.

ALTER TABLE forms DROP CONSTRAINT IF EXISTS forms_status_check;
ALTER TABLE forms ADD CONSTRAINT forms_status_check CHECK (status IN ('draft', 'open', 'closed'));
ALTER TABLE forms ALTER COLUMN status SET DEFAULT 'draft';

ALTER TABLE forms ADD COLUMN has_unpublished_changes BOOLEAN NOT NULL DEFAULT TRUE;
-- Existing forms used a flow where every save updated both schemas in sync,
-- so none of them have genuine unpublished changes.
UPDATE forms SET has_unpublished_changes = FALSE;
