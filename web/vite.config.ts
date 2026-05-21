import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vitest/config';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	define: {
		'import.meta.env.VITE_FORMS_DOMAIN': JSON.stringify(process.env.CONFIDE_FORMS_DOMAIN ?? '')
	},
	server: {
		port: parseInt(process.env.CONFIDE_WEB_PORT ?? '3000', 10),
		proxy: {
			'/api': `http://localhost:${process.env.CONFIDE_PORT ?? '8080'}`,
			'/relay': `http://localhost:${process.env.CONFIDE_PORT ?? '8080'}`
		}
	},
	test: {
		environment: 'node',
		setupFiles: ['./vitest.setup.ts'],
		coverage: {
			provider: 'v8',
			include: ['src/lib/crypto.ts'],
			reporter: ['text', 'lcov'],
			thresholds: {
				functions: 100,
				lines: 100
			}
		}
	}
});
