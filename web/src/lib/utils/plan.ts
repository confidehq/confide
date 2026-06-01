export function planLabel(plan: string, planStatus: string): string {
	if (plan === "pro") {
		if (planStatus === "past_due") return "Pro · past due";
		if (planStatus === "canceled") return "Pro · canceled";
		if (planStatus === "canceling") return "Pro · cancels at period end";
		return "Pro";
	}
	if (plan === "community") return "Community";
	return "Free";
}

export function isManagedEdition(edition: string): boolean {
	return edition !== "community";
}
