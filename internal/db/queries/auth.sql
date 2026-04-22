-- name: CreateAccount :one
INSERT INTO accounts (
    id, created_at, recovery_wrapped_master, recovery_verifier, username
) VALUES (
    $1, NOW(), $2, $3, $4
) RETURNING *;

-- name: GetAccountByID :one
SELECT * FROM accounts WHERE id = $1;

-- name: GetAccountByUsername :one
SELECT * FROM accounts WHERE username = $1;

-- name: CreateSession :one
INSERT INTO sessions (id, account_id, token_hash, created_at, last_seen, credential_id, user_agent)
VALUES ($1, $2, $3, NOW(), NOW(), $4, $5)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT s.id, s.account_id, s.token_hash, s.created_at, s.last_seen, c.wrapped_master_key
FROM sessions s
JOIN credentials c ON c.credential_id = s.credential_id
WHERE s.token_hash = $1
  AND s.last_seen > NOW() - INTERVAL '14 days'
  AND s.created_at > NOW() - INTERVAL '30 days';

-- name: TouchSession :exec
UPDATE sessions SET last_seen = NOW(), updated_at = NOW() WHERE id = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1 AND account_id = $2;

-- name: DeleteStaleSessions :exec
DELETE FROM sessions
WHERE last_seen <= NOW() - INTERVAL '14 days'
   OR created_at <= NOW() - INTERVAL '30 days';

-- name: ListSessionsByAccount :many
SELECT id, created_at, last_seen, credential_id, user_agent FROM sessions WHERE account_id = $1;

-- name: CreateRecoveryCodes :copyfrom
INSERT INTO recovery_codes (id, account_id, code_hash, used, created_at)
VALUES ($1, $2, $3, $4, $5);

-- name: GetUnusedRecoveryCode :one
SELECT * FROM recovery_codes
WHERE account_id = $1 AND code_hash = $2 AND used = FALSE
LIMIT 1;

-- name: BurnRecoveryCode :exec
UPDATE recovery_codes SET used = TRUE, updated_at = NOW() WHERE id = $1;

-- name: CountUnusedRecoveryCodes :one
SELECT COUNT(*) FROM recovery_codes WHERE account_id = $1 AND used = FALSE;

-- name: DeleteRecoveryCodesByAccount :exec
DELETE FROM recovery_codes WHERE account_id=$1;

-- name: UpdateAccountRecovery :exec
UPDATE accounts SET recovery_wrapped_master=$2, recovery_verifier=$3, updated_at=NOW() WHERE id=$1;

-- name: CreateCredential :one
INSERT INTO credentials (
    id, account_id, credential_id, public_key, prf_salt,
    wrapped_master_key, backup_eligible, name, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
RETURNING *;

-- name: GetCredentialByWebAuthnID :one
SELECT c.id, c.account_id, c.credential_id, c.public_key, c.prf_salt,
       c.wrapped_master_key, c.backup_eligible, c.name, c.created_at,
       a.recovery_wrapped_master, a.recovery_verifier, a.username
FROM credentials c
JOIN accounts a ON a.id = c.account_id
WHERE c.credential_id = $1;

-- name: GetCredentialForLogin :one
SELECT c.wrapped_master_key, c.prf_salt, c.account_id
FROM credentials c
WHERE c.credential_id = $1;

-- name: GetPrimaryCredentialByAccount :one
SELECT prf_salt FROM credentials
WHERE account_id = $1
ORDER BY created_at ASC
LIMIT 1;

-- name: GetPrimaryCredentialIDByAccount :one
SELECT credential_id FROM credentials
WHERE account_id = $1
ORDER BY created_at ASC
LIMIT 1;

-- name: GetAllPrfSalts :many
SELECT credential_id, prf_salt FROM credentials;

-- name: ListCredentialsByAccount :many
SELECT id, account_id, credential_id, backup_eligible, name, created_at
FROM credentials
WHERE account_id = $1
ORDER BY created_at ASC;

-- name: UpdateCredentialName :exec
UPDATE credentials SET name = $3, updated_at = NOW()
WHERE id = $1 AND account_id = $2;

-- name: UpdateCredentialBackupEligible :exec
UPDATE credentials SET backup_eligible = $2, updated_at = NOW() WHERE credential_id = $1;

-- name: DeleteCredential :exec
DELETE FROM credentials WHERE id = $1 AND account_id = $2;

-- name: CountCredentialsByAccount :one
SELECT COUNT(*) FROM credentials WHERE account_id = $1;

-- name: DeleteCredentialsByAccount :exec
DELETE FROM credentials WHERE account_id = $1;

-- name: DeleteAccount :exec
DELETE FROM accounts WHERE id = $1;

-- name: ListOwnedWorkspacesForDeletion :many
SELECT w.id, w.stripe_subscription_id
FROM workspaces w
JOIN workspace_members wm ON wm.workspace_id = w.id
WHERE wm.account_id = $1 AND wm.role = 'owner';
