ALTER TABLE forms
    DROP COLUMN IF EXISTS use_custom_domain;

ALTER TABLE workspaces
    DROP COLUMN IF EXISTS custom_domain_verified,
    DROP COLUMN IF EXISTS custom_domain;
