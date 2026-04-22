-- name: CreateForm :one
INSERT INTO forms (
    id, workspace_id, created_by_account_id, created_at, updated_at, status, schema_version,
    response_count, encrypted_schema, render_encrypted_schema, public_form_key,
    render_key_salt, expires_at, response_limit, response_ttl_days, burn_after_reading,
    workspace_wrapped_form_key
) VALUES (
    $1, $2, $3, NOW(), NOW(), 'open', 1, 0, $4, $5, $6, $7, $8, $9, $10, $11, $12
) RETURNING *;

-- name: GetFormWorkspaceID :one
SELECT workspace_id FROM forms WHERE id = $1;

-- name: GetFormByWorkspace :one
SELECT * FROM forms WHERE id = $1 AND workspace_id = $2;

-- name: GetFormPublic :one
SELECT id, status, schema_version, response_count, render_encrypted_schema, public_form_key, expires_at, response_limit, workspace_id, use_custom_domain
FROM forms WHERE id = $1;

-- name: ListFormsByWorkspace :many
SELECT id, status, schema_version, response_count, created_at, updated_at, expires_at, response_limit, response_ttl_days, burn_after_reading, use_custom_domain
FROM forms WHERE workspace_id = $1 ORDER BY created_at DESC;

-- name: UpdateFormSchema :one
UPDATE forms
SET encrypted_schema = $3,
    render_encrypted_schema = $4,
    render_key_salt = $5,
    schema_version = schema_version + 1,
    updated_at = NOW()
WHERE id = $1 AND workspace_id = $2
RETURNING schema_version;

-- name: SetWorkspaceFormKey :exec
UPDATE forms
SET workspace_wrapped_form_key = $3
WHERE id = $1 AND workspace_id = $2;

-- name: UpdateFormStatus :exec
UPDATE forms SET status = $3, updated_at = NOW()
WHERE id = $1 AND workspace_id = $2;

-- name: DeleteForm :exec
DELETE FROM forms WHERE id = $1 AND workspace_id = $2;

-- name: UpdateFormExpiration :exec
UPDATE forms
SET expires_at          = $3,
    response_limit      = $4,
    response_ttl_days   = $5,
    burn_after_reading  = $6,
    updated_at          = NOW()
WHERE id = $1 AND workspace_id = $2;

-- name: SetFormCustomDomain :exec
UPDATE forms SET use_custom_domain = $3, updated_at = NOW()
WHERE id = $1 AND workspace_id = $2;

-- name: IncrementResponseCount :one
UPDATE forms
SET response_count = response_count + 1
WHERE id = $1
  AND (response_limit IS NULL OR response_count < response_limit)
  AND (expires_at IS NULL OR expires_at >= CURRENT_DATE)
  AND status = 'open'
RETURNING response_count;

-- name: InsertSchemaVersion :exec
INSERT INTO form_schema_versions (form_id, version, encrypted_schema, created_at)
VALUES ($1, $2, $3, NOW());

-- name: GetSchemaVersion :one
SELECT encrypted_schema FROM form_schema_versions
WHERE form_id = $1 AND version = $2;

-- name: ListSchemaVersions :many
SELECT version, created_at FROM form_schema_versions
WHERE form_id = $1 ORDER BY version DESC;
