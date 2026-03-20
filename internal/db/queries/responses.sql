-- name: CreateResponse :exec
INSERT INTO responses (id, form_id, received_at, schema_version, encrypted_data, ephemeral_public_key)
VALUES ($1, $2, CURRENT_DATE, $3, $4, $5);

-- name: ListResponsesFirst :many
SELECT id, form_id, received_at, schema_version, encrypted_data, ephemeral_public_key
FROM responses
WHERE form_id = $1
ORDER BY received_at DESC, id DESC
LIMIT $2;

-- name: ListResponsesAfter :many
SELECT id, form_id, received_at, schema_version, encrypted_data, ephemeral_public_key
FROM responses
WHERE form_id = $1
  AND (received_at < $2 OR (received_at = $2 AND id < $3))
ORDER BY received_at DESC, id DESC
LIMIT $4;

-- name: GetResponse :one
SELECT id, form_id, received_at, schema_version, encrypted_data, ephemeral_public_key
FROM responses WHERE id = $1 AND form_id = $2;

-- name: DeleteResponse :exec
DELETE FROM responses WHERE id = $1 AND form_id = $2;
