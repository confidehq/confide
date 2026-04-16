-- name: InsertResponseWithTTL :exec
INSERT INTO responses (id, form_id, received_at, schema_version, encrypted_data, ephemeral_public_key, expires_at)
SELECT $1, $2, NOW(), $3, $4, $5,
       CASE WHEN f.response_ttl_days IS NOT NULL
            THEN NOW() + (f.response_ttl_days || ' days')::INTERVAL
            ELSE NULL END
FROM forms f
WHERE f.id = $2;

-- name: ListResponsesFirst :many
SELECT responses.id, responses.form_id, responses.received_at, responses.schema_version, responses.encrypted_data, responses.ephemeral_public_key, responses.expires_at, responses.read_at, responses.updated_at
FROM responses
WHERE responses.form_id = $1
  AND (responses.expires_at IS NULL OR responses.expires_at > NOW())
  AND (responses.read_at IS NULL OR responses.form_id NOT IN (SELECT f.id FROM forms f WHERE f.burn_after_reading = true))
ORDER BY responses.received_at DESC, responses.id DESC
LIMIT $2;

-- name: ListResponsesAfter :many
SELECT responses.id, responses.form_id, responses.received_at, responses.schema_version, responses.encrypted_data, responses.ephemeral_public_key, responses.expires_at, responses.read_at, responses.updated_at
FROM responses
WHERE responses.form_id = $1
  AND (responses.received_at < $2 OR (responses.received_at = $2 AND responses.id < $3))
  AND (responses.expires_at IS NULL OR responses.expires_at > NOW())
  AND (responses.read_at IS NULL OR responses.form_id NOT IN (SELECT f.id FROM forms f WHERE f.burn_after_reading = true))
ORDER BY responses.received_at DESC, responses.id DESC
LIMIT $4;

-- name: GetResponse :one
SELECT responses.id, responses.form_id, responses.received_at, responses.schema_version, responses.encrypted_data, responses.ephemeral_public_key, responses.expires_at, responses.read_at, responses.updated_at
FROM responses
WHERE responses.id = $1 AND responses.form_id = $2
  AND (responses.expires_at IS NULL OR responses.expires_at > NOW())
  AND (responses.read_at IS NULL OR responses.form_id NOT IN (SELECT f.id FROM forms f WHERE f.burn_after_reading = true));

-- name: MarkResponsesRead :exec
UPDATE responses
SET read_at = NOW(), updated_at = NOW()
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
