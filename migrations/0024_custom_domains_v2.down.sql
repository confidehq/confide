ALTER TABLE workspaces
    ADD COLUMN IF NOT EXISTS custom_domain TEXT UNIQUE,
    ADD COLUMN IF NOT EXISTS custom_domain_verified BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE forms
    ADD COLUMN IF NOT EXISTS use_custom_domain BOOLEAN NOT NULL DEFAULT FALSE;

-- Restore verified domains from the new table (best-effort; only the first domain per workspace).
UPDATE workspaces w
SET custom_domain          = cd.domain,
    custom_domain_verified = cd.enabled
FROM custom_domains cd
WHERE cd.workspace_id = w.id;

DROP TABLE IF EXISTS custom_domains;
