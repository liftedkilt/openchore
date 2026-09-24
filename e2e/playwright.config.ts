import { defineConfig } from '@playwright/test';

// Ports are overridable so several checkouts (e.g. git worktrees) can run the
// suite side by side: E2E_API_PORT=8181 E2E_WEB_PORT=5281 npx playwright test
const API_PORT = process.env.E2E_API_PORT || '8080';
const WEB_PORT = process.env.E2E_WEB_PORT || '5173';
// Optional: use an already-installed Chromium instead of Playwright's pinned build.
const CHROMIUM_PATH = process.env.E2E_CHROMIUM_PATH;

export default defineConfig({
  testDir: './tests',
  timeout: 30_000,
  retries: 0,
  use: {
    baseURL: `http://localhost:${WEB_PORT}`,
    headless: true,
    screenshot: 'only-on-failure',
    trace: 'on-first-retry',
    ...(CHROMIUM_PATH ? { launchOptions: { executablePath: CHROMIUM_PATH } } : {}),
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
      command: 'cd .. && rm -f openchore.db openchore.db-shm openchore.db-wal && cp -n config/config.example.yaml config/config.yaml 2>/dev/null; go run cmd/server/main.go',
      port: Number(API_PORT),
      // The points-decay worker ticks every 15 minutes by default, which no
      // test can wait for. Only kids with decay explicitly enabled are
      // touched, so a fast tick is inert for every other spec.
      env: { POINTS_DECAY_INTERVAL: '2s', PORT: API_PORT },
      reuseExistingServer: !process.env.CI,
      timeout: 30_000,
    },
    {
      command: `cd ../web && npm run dev -- --port ${WEB_PORT} --strictPort`,
      port: Number(WEB_PORT),
      env: { VITE_API_TARGET: `http://localhost:${API_PORT}` },
      reuseExistingServer: !process.env.CI,
      timeout: 15_000,
    },
  ],
});
