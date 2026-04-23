interface AppConfig {
	formsDomain: string;
}

let cached: AppConfig | null = null;

export async function getAppConfig(): Promise<AppConfig> {
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
