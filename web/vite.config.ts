import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [sveltekit()],
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
