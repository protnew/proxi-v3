import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  server: {
    host: '127.0.0.1',
    port: 5173,
    proxy: {
      '/api': { target: 'http://127.0.0.1:8090', changeOrigin: true },
      '/ws': { target: 'ws://127.0.0.1:8090', ws: true, changeOrigin: true },
      '/nostr': { target: 'ws://127.0.0.1:8090', ws: true, changeOrigin: true },
    },
  },
  plugins: [
    svelte(),
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['favicon.svg'],
      manifest: {
        name: 'Indestructible Messenger',
        short_name: 'Messenger',
        description: 'Serverless E2E messenger — Nostr + P2P, no server needed',
        theme_color: '#1a1a2e',
        background_color: '#0f0f23',
        display: 'standalone',
        scope: '/',
        start_url: '/',
        icons: [
          { src: '/icon-192.png', sizes: '192x192', type: 'image/png' },
          { src: '/icon-512.png', sizes: '512x512', type: 'image/png' },
          { src: '/icon-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      workbox: {
        globPatterns: ['**/*.{js,css,html,ico,png,svg,webmanifest,woff2}'],
        navigateFallback: '/',
        navigateFallbackDenylist: [/^\/api\//],
      },
    }),
  ],
})
