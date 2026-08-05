CREATE TABLE IF NOT EXISTS custom_domains (
    id           TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    domain       TEXT NOT NULL UNIQUE,
    txt_token    TEXT NOT NULL,
    cname_ok     BOOLEAN NOT NULL DEFAULT FALSE,
    txt_ok       BOOLEAN NOT NULL DEFAULT FALSE,
    enabled      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    verified_at  TIMESTAMPTZ
);

-- Backfill + column drop. Runs only on first application: it reads
-- workspaces.custom_domain, which the same block drops. Guard on its presence.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'workspaces' AND column_name = 'custom_domain'
    ) THEN
        -- Migrate existing verified domains into the new table.
        INSERT INTO custom_domains (workspace_id, domain, txt_token, cname_ok, txt_ok, enabled, verified_at)
        SELECT id, custom_domain, gen_random_uuid(), TRUE, TRUE, custom_domain_verified,
               CASE WHEN custom_domain_verified THEN now() ELSE NULL END
        FROM workspaces
        WHERE custom_domain IS NOT NULL
        ON CONFLICT (domain) DO NOTHING;
    END IF;
END $$;

-- Unconditional: these columns must not exist after this migration, even if the
-- backfill above was skipped because it had already run.
ALTER TABLE workspaces
    DROP COLUMN IF EXISTS custom_domain,
    DROP COLUMN IF EXISTS custom_domain_verified;

ALTER TABLE forms
    DROP COLUMN IF EXISTS use_custom_domain;
