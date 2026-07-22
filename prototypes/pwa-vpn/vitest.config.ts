import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte({ hot: false })],
  test: {
    environment: 'node',
    globals: true,
    include: ['test/**/*.test.ts'],
  },
  resolve: {
    alias: {
      '$app': '/dev/null',
    }
  }
});
