import { defineConfig } from '@playwright/test'
export default defineConfig({
  testDir: '.',
  testMatch: 'e2e/**/*.spec.ts',
  timeout: 60000,
  workers: 1,
  use: { headless: true },
  webServer: {
    command: 'npm run dev -- --port 5175',
    port: 5175,
    reuseExistingServer: true,
  },
})
