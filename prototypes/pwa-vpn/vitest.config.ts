import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte({ hot: false })],
  test: {
    environment: 'node',
    globals: true,
    include: ['test/**/*.test.ts'],
    setupFiles: ['test/_setup.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json-summary'],
      include: ['src/lib/**/*.{ts,js}'],
      exclude: ['src/lib/nostr_old.ts', 'src/lib/**/*.d.ts'],
    },
  },
  resolve: {
    alias: {
      '$app': '/dev/null',
    },
  },
});
