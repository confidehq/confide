-- name: CreateWorkspace :one
INSERT INTO workspaces (id, name, slug, plan, plan_status, created_at)
VALUES ($1, $2, $3, 'free', 'active', now())
RETURNING *;

-- name: CreateWorkspaceMember :exec
INSERT INTO workspace_members (workspace_id, account_id, role, joined_at)
VALUES ($1, $2, $3, now());

-- name: GetPersonalWorkspace :one
SELECT w.id, w.name, w.slug, w.plan, w.plan_status, w.plan_period_end, w.created_at
FROM workspaces w
JOIN workspace_members wm ON wm.workspace_id = w.id
WHERE wm.account_id = $1 AND wm.role = 'owner'
ORDER BY w.created_at ASC
LIMIT 1;

-- name: GetWorkspaceByID :one
SELECT id, name, slug, plan, plan_status, plan_period_end, created_at
FROM workspaces WHERE id = $1;

-- name: GetWorkspaceMember :one
SELECT workspace_id, account_id, role, joined_at
FROM workspace_members WHERE workspace_id = $1 AND account_id = $2;

-- name: CountOwnerWorkspaces :one
SELECT COUNT(*) FROM workspace_members
WHERE account_id = $1 AND role = 'owner';

-- name: UpsertWorkspaceMemberKey :exec
INSERT INTO workspace_member_keys (workspace_id, account_id, wrapped_workspace_key, ephemeral_public_key, granted_by_account_id, created_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (workspace_id, account_id) DO UPDATE
  SET wrapped_workspace_key = EXCLUDED.wrapped_workspace_key,
      ephemeral_public_key  = EXCLUDED.ephemeral_public_key,
      granted_by_account_id = EXCLUDED.granted_by_account_id,
      created_at            = EXCLUDED.created_at;

-- name: GetWorkspaceMemberKey :one
SELECT wrapped_workspace_key, ephemeral_public_key
FROM workspace_member_keys
WHERE workspace_id = $1 AND account_id = $2;

-- name: GetMembersWithoutWorkspaceKey :many
SELECT wm.account_id
FROM workspace_members wm
LEFT JOIN workspace_member_keys wmk
  ON wmk.workspace_id = wm.workspace_id AND wmk.account_id = wm.account_id
WHERE wm.workspace_id = $1 AND wmk.account_id IS NULL;

-- name: ListWorkspacesByAccount :many
SELECT w.id, w.name, w.slug, w.plan, w.plan_status, wm.role
FROM workspaces w
JOIN workspace_members wm ON wm.workspace_id = w.id
WHERE wm.account_id = $1
ORDER BY w.created_at ASC;

-- name: RenameWorkspace :exec
UPDATE workspaces SET name = $2 WHERE id = $1;

-- name: DeleteWorkspace :exec
DELETE FROM workspaces WHERE id = $1;

-- name: ListWorkspaceMembers :many
SELECT
  wm.account_id,
  wm.role,
  wm.joined_at,
  a.username,
  CASE WHEN wmk.account_id IS NOT NULL THEN 'active' ELSE 'pending' END AS status,
  (SELECT MAX(s.last_seen) FROM sessions s WHERE s.account_id = wm.account_id) AS last_seen
FROM workspace_members wm
JOIN accounts a ON a.id = wm.account_id
LEFT JOIN workspace_member_keys wmk
  ON wmk.workspace_id = wm.workspace_id AND wmk.account_id = wm.account_id
WHERE wm.workspace_id = $1
ORDER BY wm.joined_at ASC;

-- name: UpdateWorkspaceMemberRole :exec
UPDATE workspace_members SET role = $3 WHERE workspace_id = $1 AND account_id = $2;

-- name: DeleteWorkspaceMember :exec
DELETE FROM workspace_members WHERE workspace_id = $1 AND account_id = $2;

-- name: DeleteWorkspaceMemberKey :exec
DELETE FROM workspace_member_keys WHERE workspace_id = $1 AND account_id = $2;

-- name: CountWorkspaceOwners :one
SELECT COUNT(*) FROM workspace_members WHERE workspace_id = $1 AND role = 'owner';

-- name: CountNonOwnerMembers :one
SELECT COUNT(*) FROM workspace_members WHERE workspace_id = $1 AND role != 'owner';

-- name: CountWorkspaceMembers :one
SELECT COUNT(*) FROM workspace_members WHERE workspace_id = $1;

-- name: ListMemberIdentityKeys :many
SELECT wm.account_id, aik.identity_public_key
FROM workspace_members wm
JOIN account_identity_keys aik ON aik.account_id = wm.account_id
WHERE wm.workspace_id = $1
ORDER BY wm.joined_at ASC;

-- name: GetMembersWithoutWorkspaceKeyWithUsername :many
SELECT wm.account_id, a.username
FROM workspace_members wm
LEFT JOIN workspace_member_keys wmk
  ON wmk.workspace_id = wm.workspace_id AND wmk.account_id = wm.account_id
JOIN accounts a ON a.id = wm.account_id
WHERE wm.workspace_id = $1 AND wmk.account_id IS NULL;
