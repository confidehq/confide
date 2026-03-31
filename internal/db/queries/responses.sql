-- name: InsertResponseWithTTL :exec
INSERT INTO responses (id, form_id, received_at, schema_version, encrypted_data, ephemeral_public_key, expires_at)
SELECT $1, $2, CURRENT_DATE, $3, $4, $5,
       CASE WHEN f.response_ttl_days IS NOT NULL
            THEN NOW() + (f.response_ttl_days || ' days')::INTERVAL
            ELSE NULL END
FROM forms f
WHERE f.id = $2;

-- name: ListResponsesFirst :many
SELECT id, form_id, received_at, schema_version, encrypted_data, ephemeral_public_key, expires_at, read_at
FROM responses
WHERE form_id = $1
  AND (expires_at IS NULL OR expires_at > NOW())
  AND (read_at IS NULL OR form_id NOT IN (SELECT id FROM forms WHERE burn_after_reading = true))
ORDER BY received_at DESC, id DESC
LIMIT $2;

-- name: ListResponsesAfter :many
SELECT id, form_id, received_at, schema_version, encrypted_data, ephemeral_public_key, expires_at, read_at
FROM responses
WHERE form_id = $1
  AND (received_at < $2 OR (received_at = $2 AND id < $3))
  AND (expires_at IS NULL OR expires_at > NOW())
  AND (read_at IS NULL OR form_id NOT IN (SELECT id FROM forms WHERE burn_after_reading = true))
ORDER BY received_at DESC, id DESC
LIMIT $4;

-- name: GetResponse :one
SELECT id, form_id, received_at, schema_version, encrypted_data, ephemeral_public_key, expires_at, read_at
FROM responses
WHERE id = $1 AND form_id = $2
  AND (expires_at IS NULL OR expires_at > NOW())
  AND (read_at IS NULL OR form_id NOT IN (SELECT id FROM forms WHERE burn_after_reading = true));

-- name: MarkResponsesRead :exec
UPDATE responses
SET read_at = NOW()
WHERE form_id = $1
  AND id = ANY($2::text[])
  AND read_at IS NULL;

-- name: DeleteExpiredResponses :exec
DELETE FROM responses
WHERE (expires_at IS NOT NULL AND expires_at < NOW())
   OR (read_at IS NOT NULL AND form_id IN (
         SELECT id FROM forms WHERE burn_after_reading = true
       ));

-- name: DeleteResponse :exec
DELETE FROM responses WHERE id = $1 AND form_id = $2;
