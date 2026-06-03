import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  server: {
    proxy: {
      '/api': 'http://localhost:9999',
      '/ws': {
        target: 'ws://localhost:9999',
        ws: true,
      },
    },
  },
  build: {
    // Output to dist-svelte/ to not overwrite existing dist/
    // When ready for production, change to '../dist' and set emptyOutDir: false
    outDir: '../dist-svelte',
    emptyOutDir: true,
  },
});
