-- name: CreateInvitation :one
INSERT INTO workspace_invitations (id, workspace_id, invited_by_account_id, email, role, token_hash, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, workspace_id, invited_by_account_id, email, role, expires_at, created_at;

-- name: GetInvitationByTokenHash :one
SELECT wi.id, wi.workspace_id, wi.invited_by_account_id, wi.email, wi.role,
       wi.expires_at, wi.accepted_at, wi.created_at,
       w.name AS workspace_name, a.username AS inviter_username
FROM workspace_invitations wi
JOIN workspaces w ON w.id = wi.workspace_id
JOIN accounts a ON a.id = wi.invited_by_account_id
WHERE wi.token_hash = $1;

-- name: ListPendingInvitations :many
SELECT id, email, role, expires_at, created_at
FROM workspace_invitations
WHERE workspace_id = $1 AND accepted_at IS NULL AND expires_at > now()
ORDER BY created_at DESC;

-- name: DeleteInvitation :exec
DELETE FROM workspace_invitations WHERE id = $1 AND workspace_id = $2;

-- name: AcceptInvitation :exec
UPDATE workspace_invitations SET accepted_at = now() WHERE id = $1;
