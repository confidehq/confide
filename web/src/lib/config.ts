interface AppConfig {
	formsDomain: string;
}

let cached: AppConfig | null = null;

const envFormsDomain = import.meta.env.VITE_FORMS_DOMAIN ?? '';

export async function getAppConfig(): Promise<AppConfig> {
	if (envFormsDomain) return { formsDomain: envFormsDomain };
	if (cached) return cached;
	try {
		const res = await fetch('/api/config');
		if (res.ok) {
			cached = await res.json();
			return cached!;
		}
	} catch {
		// ignore — caller falls back to window.location.origin
	}
	return { formsDomain: '' };
}
