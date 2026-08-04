import { defineConfig } from '@playwright/test';

// Native Windows: Vite :5173 + Go API :8080 must be up
export default defineConfig({
  testDir: '.',
  testMatch: [
    'e2e/**/*.{spec,test}.{ts,js}',
    'tests/e2e-messenger.spec.ts',
  ],
  // WebRTC preview suite needs :4173 — not part of native dev smoke
  testIgnore: ['tests/e2e.spec.ts', '**/node_modules/**'],
  timeout: 45000,
  fullyParallel: false,
  retries: 0,
  workers: 1,
  use: {
    baseURL: process.env.APP_URL || 'http://127.0.0.1:5173',
    trace: 'on-first-retry',
  },
});
