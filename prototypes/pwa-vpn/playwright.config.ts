import { defineConfig } from '@playwright/test';

// Native Windows / local: expects Vite :5173 and Go API :8080 already running
// (no Docker). CI starts them as services/steps.
export default defineConfig({
  testDir: './e2e',
  timeout: 45000,
  fullyParallel: false,
  retries: 0,
  use: {
    baseURL: process.env.APP_URL || 'http://localhost:5173',
    trace: 'on-first-retry',
  },
  // Keep deep.spec + messenger + alice-bob under e2e/
});
