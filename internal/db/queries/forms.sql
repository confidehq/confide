-- name: CreateForm :one
INSERT INTO forms (
    id, account_id, created_at, updated_at, status, schema_version,
    response_count, encrypted_schema, render_encrypted_schema, public_form_key
) VALUES (
    $1, $2, CURRENT_DATE, CURRENT_DATE, 'open', 1, 0, $3, $4, $5
) RETURNING *;

-- name: GetFormByOwner :one
SELECT * FROM forms WHERE id = $1 AND account_id = $2;

-- name: GetFormPublic :one
SELECT id, status, schema_version, render_encrypted_schema, public_form_key
FROM forms WHERE id = $1;

-- name: ListFormsByAccount :many
SELECT id, status, schema_version, response_count, created_at, updated_at
FROM forms WHERE account_id = $1 ORDER BY created_at DESC;

-- name: UpdateFormSchema :one
UPDATE forms
SET encrypted_schema = $3,
    render_encrypted_schema = $4,
    schema_version = schema_version + 1,
    updated_at = CURRENT_DATE
WHERE id = $1 AND account_id = $2
RETURNING schema_version;

-- name: UpdateFormStatus :exec
UPDATE forms SET status = $3, updated_at = CURRENT_DATE
WHERE id = $1 AND account_id = $2;

-- name: DeleteForm :exec
DELETE FROM forms WHERE id = $1 AND account_id = $2;

-- name: IncrementResponseCount :exec
UPDATE forms SET response_count = response_count + $2 WHERE id = $1;
