ALTER TABLE custom_domains DROP CONSTRAINT IF EXISTS custom_domains_workspace_id_key;
ALTER TABLE custom_domains ADD CONSTRAINT custom_domains_workspace_id_key UNIQUE (workspace_id);
