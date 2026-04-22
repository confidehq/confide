-- name: GetWorkspaceForBilling :one
SELECT id, name, plan, plan_status, plan_period_end, stripe_customer_id, stripe_subscription_id
FROM workspaces WHERE id = $1;

-- name: CountFormsByWorkspace :one
SELECT COUNT(*) FROM forms WHERE workspace_id = $1;

-- name: CountMonthlyResponses :one
SELECT COUNT(*)
FROM responses r
JOIN forms f ON f.id = r.form_id
WHERE f.workspace_id = $1
  AND r.received_at >= date_trunc('month', NOW());

-- name: SetStripeCustomerID :exec
UPDATE workspaces SET stripe_customer_id = $2, updated_at = NOW() WHERE id = $1;

-- name: SetStripeSubscriptionID :exec
UPDATE workspaces SET stripe_subscription_id = $2, updated_at = NOW()
WHERE stripe_customer_id = $1;

-- name: UpdateWorkspacePlan :exec
UPDATE workspaces
SET plan                   = $2,
    plan_status            = $3,
    plan_period_end        = $4,
    stripe_subscription_id = $5,
    updated_at             = NOW()
WHERE id = $1;

-- name: GetWorkspaceByStripeCustomerID :one
SELECT id, plan, plan_status, stripe_subscription_id
FROM workspaces WHERE stripe_customer_id = $1;
