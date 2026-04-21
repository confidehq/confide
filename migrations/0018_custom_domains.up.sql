ALTER TABLE workspaces
    ADD COLUMN custom_domain TEXT UNIQUE,
    ADD COLUMN custom_domain_verified BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE forms
    ADD COLUMN use_custom_domain BOOLEAN NOT NULL DEFAULT FALSE;
