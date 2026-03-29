-- name: CreateAccount :one
INSERT INTO accounts (
    id, created_at, credential_id, public_key, prf_salt,
    wrapped_master_key, recovery_wrapped_master, recovery_verifier, backup_eligible, username
) VALUES (
    $1, CURRENT_DATE, $2, $3, $4, $5, $6, $7, $8, $9
) RETURNING *;

-- name: GetAccountByCredentialID :one
SELECT * FROM accounts WHERE credential_id = $1;

-- name: GetAccountByID :one
SELECT * FROM accounts WHERE id = $1;

-- name: CreateSession :one
INSERT INTO sessions (id, account_id, token_hash, created_at, last_seen, credential_id, user_agent)
VALUES ($1, $2, $3, CURRENT_DATE, CURRENT_DATE, $4, $5)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT s.id, s.account_id, s.token_hash, s.created_at, s.last_seen, a.wrapped_master_key
FROM sessions s
JOIN accounts a ON a.id = s.account_id
WHERE s.token_hash = $1
  AND s.last_seen > CURRENT_DATE - INTERVAL '14 days'
  AND s.created_at > CURRENT_DATE - INTERVAL '30 days';

-- name: TouchSession :exec
UPDATE sessions SET last_seen = CURRENT_DATE WHERE id = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1 AND account_id = $2;

-- name: DeleteStaleSessions :exec
DELETE FROM sessions
WHERE last_seen <= CURRENT_DATE - INTERVAL '14 days'
   OR created_at <= CURRENT_DATE - INTERVAL '30 days';

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
UPDATE recovery_codes SET used = TRUE WHERE id = $1;

-- name: CountUnusedRecoveryCodes :one
SELECT COUNT(*) FROM recovery_codes WHERE account_id = $1 AND used = FALSE;

-- name: UpdateAccountCredential :exec
UPDATE accounts SET credential_id=$2, public_key=$3, prf_salt=$4,
    wrapped_master_key=$5, recovery_wrapped_master=$6, recovery_verifier=$7,
    backup_eligible=$8 WHERE id=$1;

-- name: DeleteRecoveryCodesByAccount :exec
DELETE FROM recovery_codes WHERE account_id=$1;

-- name: UpdateAccountBackupEligible :exec
UPDATE accounts SET backup_eligible=$2 WHERE credential_id=$1;

-- name: GetAccountByUsername :one
SELECT * FROM accounts WHERE username = $1;

-- name: ListCredentials :many
SELECT credential_id, prf_salt FROM accounts;
