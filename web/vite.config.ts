import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [sveltekit()],
	server: {
		port: 3000,
		proxy: {
			'/api': 'http://localhost:8080'
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
