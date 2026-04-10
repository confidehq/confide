-- name: UpsertIdentityKey :exec
INSERT INTO account_identity_keys (account_id, identity_public_key, wrapped_identity_private_key, created_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (account_id) DO UPDATE
  SET identity_public_key          = EXCLUDED.identity_public_key,
      wrapped_identity_private_key = EXCLUDED.wrapped_identity_private_key,
      created_at                   = EXCLUDED.created_at;

-- name: GetIdentityKey :one
SELECT identity_public_key, wrapped_identity_private_key
FROM account_identity_keys WHERE account_id = $1;

-- name: GetIdentityPublicKey :one
SELECT identity_public_key
FROM account_identity_keys WHERE account_id = $1;
