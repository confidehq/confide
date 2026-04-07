import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vitest/config';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		port: 3000,
		proxy: {
			'/api': 'http://localhost:8080',
			'/relay': 'http://localhost:8080'
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
