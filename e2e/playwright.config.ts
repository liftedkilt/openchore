import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  timeout: 30_000,
  retries: 0,
  use: {
    baseURL: 'http://localhost:5173',
    headless: true,
    screenshot: 'only-on-failure',
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'main',
      use: { browserName: 'chromium' },
      testIgnore: /admin-pin-change/,
    },
    {
      name: 'pin-change',
      use: { browserName: 'chromium' },
      testMatch: /admin-pin-change/,
      dependencies: ['main'],
    },
  ],
  webServer: [
    {
      command: 'cd .. && rm -f openchore.db openchore.db-shm openchore.db-wal && go run cmd/server/main.go',
      port: 8080,
      reuseExistingServer: !process.env.CI,
      timeout: 30_000,
    },
    {
      command: 'cd ../web && npm run dev',
      port: 5173,
      reuseExistingServer: !process.env.CI,
      timeout: 15_000,
    },
  ],
});
