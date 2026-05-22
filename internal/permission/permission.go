package permission

import "fmt"

type contextKey string

const workspaceRoleKey contextKey = "workspaceRole"

// Action represents a gateable operation within a workspace.
type Action string

const (
	ActionViewForms       Action = "view_forms"
	ActionManageForms     Action = "manage_forms"
	ActionDistributeKeys  Action = "distribute_keys"
	ActionInviteMembers   Action = "invite_members"
	ActionChangeRoles     Action = "change_roles"
	ActionRenameWorkspace Action = "rename_workspace"
	ActionManageBilling      Action = "manage_billing"
	ActionDeleteWorkspace    Action = "delete_workspace"
	ActionManageCustomDomain Action = "manage_custom_domain"
)

// Feature represents a Pro-tier gated capability.
type Feature string

const (
	FeatureCustomStyles       Feature = "custom_styles"
	FeatureWhitelabel         Feature = "whitelabel"
	FeatureCustomDomains      Feature = "custom_domains"
	FeatureAdvancedAnalytics  Feature = "advanced_analytics"
	FeaturePartialSubmissions Feature = "partial_submissions"
	FeatureVersionHistory     Feature = "version_history"
	FeatureExtendedEmailFwd   Feature = "extended_email_forwarding"
)

// RoleRank returns a numeric rank for a role string (higher = more privileged).
func RoleRank(role string) int {
	switch role {
	case "owner":
		return 4
	case "admin":
		return 3
	case "member":
		return 2
	case "viewer":
		return 1
	default:
		return 0
	}
}

// roleMatrix maps each action to the minimum rank required.
var roleMatrix = map[Action]int{
	ActionViewForms:       1,
	ActionManageForms:     2,
	ActionDistributeKeys:  3,
	ActionInviteMembers:   3,
	ActionChangeRoles:     3,
	ActionRenameWorkspace: 3,
	ActionManageBilling:      4,
	ActionDeleteWorkspace:    4,
	ActionManageCustomDomain: 3,
}

// Can reports whether a role is permitted to perform an action.
func Can(role string, action Action) bool {
	minimum, ok := roleMatrix[action]
	if !ok {
		return false
	}
	return RoleRank(role) >= minimum
}

// proFeatures is the set of features that require the pro plan.
var proFeatures = map[Feature]struct{}{
	FeatureCustomStyles:       {},
	FeatureWhitelabel:         {},
	FeatureCustomDomains:      {},
	FeatureAdvancedAnalytics:  {},
	FeaturePartialSubmissions: {},
	FeatureVersionHistory:     {},
	FeatureExtendedEmailFwd:   {},
}

// PlanAllows reports whether the given plan may use the given feature.
func PlanAllows(plan string, f Feature) bool {
	if _, gated := proFeatures[f]; !gated {
		return true
	}
	return plan == "pro" || plan == "community"
}

// UpgradeRequiredError is returned by service methods when a Pro feature is
// accessed on the free plan. Handlers translate it to HTTP 402.
type UpgradeRequiredError struct {
	Feature Feature
}

func (e *UpgradeRequiredError) Error() string {
	return fmt.Sprintf("pro plan required for feature: %s", e.Feature)
}
