const STORAGE_KEY = 'theme';

function getInitialTheme(): 'dark' | 'light' {
	if (typeof localStorage === 'undefined') return 'light';
	const stored = localStorage.getItem(STORAGE_KEY);
	if (stored === 'light' || stored === 'dark') return stored;
	return typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches
		? 'dark'
		: 'light';
}

function applyTheme(value: 'dark' | 'light') {
	if (typeof document === 'undefined') return;
	document.documentElement.setAttribute('data-theme', value);
}

function createThemeStore() {
	let value = $state<'dark' | 'light'>(getInitialTheme());
	applyTheme(value);

	return {
		get value() { return value; },
		toggle() {
			value = value === 'dark' ? 'light' : 'dark';
			if (typeof localStorage !== 'undefined') {
				localStorage.setItem(STORAGE_KEY, value);
			}
			applyTheme(value);
		}
	};
}

export const theme = createThemeStore();
