-- name: CreateWorkspace :one
INSERT INTO workspaces (id, name, slug, plan, plan_status, created_at)
VALUES ($1, $2, $3, 'free', 'active', NOW())
RETURNING *;

-- name: CreateWorkspaceMember :exec
INSERT INTO workspace_members (workspace_id, account_id, role, joined_at)
VALUES ($1, $2, $3, NOW());

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
SELECT workspace_id, account_id, role, joined_at, updated_at
FROM workspace_members WHERE workspace_id = $1 AND account_id = $2;

-- name: CountOwnerWorkspaces :one
SELECT COUNT(*) FROM workspace_members
WHERE account_id = $1 AND role = 'owner';

-- name: UpsertWorkspaceMemberKey :exec
INSERT INTO workspace_member_keys (workspace_id, account_id, wrapped_workspace_key, ephemeral_public_key, granted_by_account_id, created_at)
VALUES ($1, $2, $3, $4, $5, NOW())
ON CONFLICT (workspace_id, account_id) DO UPDATE
  SET wrapped_workspace_key = EXCLUDED.wrapped_workspace_key,
      ephemeral_public_key  = EXCLUDED.ephemeral_public_key,
      granted_by_account_id = EXCLUDED.granted_by_account_id,
      updated_at            = NOW();

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
SELECT w.id, w.name, w.slug, w.plan, w.plan_status, wm.role,
  CASE WHEN wmk.account_id IS NOT NULL THEN 'active' ELSE 'pending' END AS status
FROM workspaces w
JOIN workspace_members wm ON wm.workspace_id = w.id
LEFT JOIN workspace_member_keys wmk
  ON wmk.workspace_id = wm.workspace_id AND wmk.account_id = wm.account_id
WHERE wm.account_id = $1
ORDER BY w.created_at ASC;

-- name: RenameWorkspace :exec
UPDATE workspaces SET name = $2, updated_at = NOW() WHERE id = $1;

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
UPDATE workspace_members SET role = $3, updated_at = NOW() WHERE workspace_id = $1 AND account_id = $2;

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

-- name: InsertCustomDomain :one
INSERT INTO custom_domains (workspace_id, domain, txt_token)
VALUES ($1, $2, $3)
ON CONFLICT (workspace_id) DO UPDATE
    SET domain    = EXCLUDED.domain,
        txt_token = EXCLUDED.txt_token,
        cname_ok  = FALSE,
        txt_ok    = FALSE,
        enabled   = FALSE,
        verified_at = NULL
RETURNING *;

-- name: GetCustomDomainByWorkspace :one
SELECT * FROM custom_domains WHERE workspace_id = $1;

-- name: GetCustomDomainByHost :one
SELECT * FROM custom_domains WHERE domain = $1;

-- name: UpdateDNSStatus :exec
UPDATE custom_domains SET cname_ok = $2, txt_ok = $3 WHERE id = $1;

-- name: EnableCustomDomain :exec
UPDATE custom_domains SET enabled = TRUE, verified_at = NOW() WHERE id = $1;

-- name: DeleteCustomDomain :exec
DELETE FROM custom_domains WHERE workspace_id = $1;

-- name: ListAllEnabledDomains :many
SELECT domain FROM custom_domains WHERE enabled = TRUE;

-- name: ListAllUnverifiedDomains :many
SELECT * FROM custom_domains WHERE enabled = FALSE;

-- name: GetMembersWithoutWorkspaceKeyWithUsername :many
SELECT wm.account_id, a.username
FROM workspace_members wm
LEFT JOIN workspace_member_keys wmk
  ON wmk.workspace_id = wm.workspace_id AND wmk.account_id = wm.account_id
JOIN accounts a ON a.id = wm.account_id
WHERE wm.workspace_id = $1 AND wmk.account_id IS NULL;
