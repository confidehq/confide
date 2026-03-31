-- name: CreateForm :one
INSERT INTO forms (
    id, account_id, created_at, updated_at, status, schema_version,
    response_count, encrypted_schema, render_encrypted_schema, public_form_key,
    render_key_salt, expires_at, response_limit
) VALUES (
    $1, $2, CURRENT_DATE, CURRENT_DATE, 'open', 1, 0, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: GetFormByOwner :one
SELECT * FROM forms WHERE id = $1 AND account_id = $2;

-- name: GetFormPublic :one
SELECT id, status, schema_version, response_count, render_encrypted_schema, public_form_key, expires_at, response_limit
FROM forms WHERE id = $1;

-- name: ListFormsByAccount :many
SELECT id, status, schema_version, response_count, created_at, updated_at, expires_at, response_limit
FROM forms WHERE account_id = $1 ORDER BY created_at DESC;

-- name: UpdateFormSchema :one
UPDATE forms
SET encrypted_schema = $3,
    render_encrypted_schema = $4,
    render_key_salt = $5,
    schema_version = schema_version + 1,
    updated_at = CURRENT_DATE
WHERE id = $1 AND account_id = $2
RETURNING schema_version;

-- name: UpdateFormStatus :exec
UPDATE forms SET status = $3, updated_at = CURRENT_DATE
WHERE id = $1 AND account_id = $2;

-- name: DeleteForm :exec
DELETE FROM forms WHERE id = $1 AND account_id = $2;

-- name: UpdateFormExpiration :exec
UPDATE forms
SET expires_at = $3, response_limit = $4, updated_at = CURRENT_DATE
WHERE id = $1 AND account_id = $2;

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
VALUES ($1, $2, $3, CURRENT_DATE);

-- name: GetSchemaVersion :one
SELECT encrypted_schema FROM form_schema_versions
WHERE form_id = $1 AND version = $2;

-- name: ListSchemaVersions :many
SELECT version, created_at FROM form_schema_versions
WHERE form_id = $1 ORDER BY version DESC;
