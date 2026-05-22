interface AppConfig {
	formsDomain: string;
	registrationOpen: boolean;
	emailEnabled: boolean;
	smtpSender: string;
	edition: string; // "community" = self-hosted, "" = managed
}

let cached: AppConfig | null = null;

const envFormsDomain = import.meta.env.VITE_FORMS_DOMAIN ?? '';

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
	return { formsDomain: envFormsDomain, registrationOpen: true, emailEnabled: false, smtpSender: '', edition: '' };
}
