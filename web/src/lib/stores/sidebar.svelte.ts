const STORAGE_KEY = 'sidebar:collapsed';

function createSidebarStore() {
	let collapsed = $state(
		typeof localStorage !== 'undefined'
			? localStorage.getItem(STORAGE_KEY) === 'true'
			: false
	);

	let mobileOpen = $state(false);

	return {
		get collapsed() { return collapsed; },
		get mobileOpen() { return mobileOpen; },
		get width() { return collapsed ? 52 : 200; },
		toggle() {
			collapsed = !collapsed;
			if (typeof localStorage !== 'undefined') {
				localStorage.setItem(STORAGE_KEY, String(collapsed));
			}
		},
		openMobile() { mobileOpen = true; },
		closeMobile() { mobileOpen = false; }
	};
}

export const sidebar = createSidebarStore();
