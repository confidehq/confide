CREATE TABLE workspace_email_usage (
    workspace_id TEXT        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    period       TEXT        NOT NULL, -- "YYYY-MM"
    count        BIGINT      NOT NULL DEFAULT 0,
    PRIMARY KEY (workspace_id, period)
);
