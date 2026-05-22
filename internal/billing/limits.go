package billing

// InstanceLimits holds configured overrides for a self-hosted instance.
// Zero values fall through to plan defaults. Use -1 for explicit unlimited.
type InstanceLimits struct {
	Edition                 string // "community" = all pro features unlocked, no managed billing
	MaxMembersPerWorkspace  int64
	MaxMonthlyResponses     int64
	MaxStoredResponses      int64
	MaxMonthlyEmails        int64
	MaxFileStorageBytes     int64
	MaxWorkspacesPerAccount int64
}

// IsCommunity reports whether this instance runs in self-hosted community mode.
func (l InstanceLimits) IsCommunity() bool {
	return l.Edition == "community"
}

// MemberLimit returns the effective member limit for the given plan.
// Returns -1 for unlimited.
func (l InstanceLimits) MemberLimit(plan string) int64 {
	if l.MaxMembersPerWorkspace != 0 {
		return l.MaxMembersPerWorkspace
	}
	if l.IsCommunity() {
		return -1
	}
	return PlanMemberLimit(plan)
}

// MonthlyResponseLimit returns the effective monthly response limit.
func (l InstanceLimits) MonthlyResponseLimit(plan string) int64 {
	if l.MaxMonthlyResponses != 0 {
		return l.MaxMonthlyResponses
	}
	if l.IsCommunity() {
		return -1
	}
	return PlanMonthlyResponseLimit(plan)
}

// StoredResponseLimit returns the effective stored response limit.
func (l InstanceLimits) StoredResponseLimit(plan string) int64 {
	if l.MaxStoredResponses != 0 {
		return l.MaxStoredResponses
	}
	if l.IsCommunity() {
		return -1
	}
	return PlanStoredResponseLimit(plan)
}

// MonthlyEmailLimit returns the effective monthly email limit.
func (l InstanceLimits) MonthlyEmailLimit(plan string) int64 {
	if l.MaxMonthlyEmails != 0 {
		return l.MaxMonthlyEmails
	}
	if l.IsCommunity() {
		return -1
	}
	return PlanMonthlyEmailLimit(plan)
}

// FileStorageLimit returns the effective file storage limit in bytes.
func (l InstanceLimits) FileStorageLimit(plan string) int64 {
	if l.MaxFileStorageBytes != 0 {
		return l.MaxFileStorageBytes
	}
	if l.IsCommunity() {
		return -1
	}
	return PlanFileStorageLimit(plan)
}

// WorkspaceLimit returns the max number of workspaces an account may own.
// Returns -1 for unlimited.
func (l InstanceLimits) WorkspaceLimit() int64 {
	if l.MaxWorkspacesPerAccount != 0 {
		return l.MaxWorkspacesPerAccount
	}
	if l.IsCommunity() {
		return -1
	}
	return 1
}
