-- Add workspace_wrapped_form_key to forms.
-- Stores the raw form key bytes encrypted with the workspace key (AES-256-GCM).
-- NULL for forms created before this migration; set lazily when the workspace
-- owner next views or edits the form.
ALTER TABLE forms ADD COLUMN workspace_wrapped_form_key BYTEA;
