CREATE TABLE IF NOT EXISTS forms (
    id                        TEXT PRIMARY KEY,
    account_id                TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    created_at                DATE NOT NULL,
    updated_at                DATE NOT NULL,
    status                    TEXT NOT NULL DEFAULT 'open',
    schema_version            INTEGER NOT NULL DEFAULT 1,
    response_count            INTEGER NOT NULL DEFAULT 0,
    encrypted_schema          BYTEA NOT NULL,
    render_encrypted_schema   BYTEA NOT NULL,
    public_form_key           BYTEA NOT NULL
);

-- forms.account_id is replaced by workspace_id in 0013, so on a replay against an
-- evolved schema the column is gone and this index is obsolete.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'forms' AND column_name = 'account_id'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_forms_account_id ON forms(account_id);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS responses (
    id                   TEXT PRIMARY KEY,
    form_id              TEXT NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    received_at          DATE NOT NULL,
    schema_version       INTEGER NOT NULL,
    encrypted_data       BYTEA NOT NULL,
    ephemeral_public_key BYTEA NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_responses_form_id_received ON responses(form_id, received_at DESC, id DESC);
