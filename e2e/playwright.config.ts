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
      testIgnore: /admin-pin-change|sign-in-options|leaderboard|custom-animal|team-goal-decoration/,
    },
    {
      // Specs that turn a household-wide feature on (farm pets, the
      // leaderboard), run after the rest so they can't change the screens
      // other specs are looking at.
      name: 'shared-state',
      use: { browserName: 'chromium' },
      testMatch: /leaderboard|custom-animal/,
      dependencies: ['main'],
    },
    {
      // Runs after leaderboard.spec.ts because both specs mutate hub_url,
      // leaderboard_enabled, and friend_links on the shared backend.
      name: 'shared-state-friends',
      use: { browserName: 'chromium' },
      testMatch: /team-goal-decoration/,
      dependencies: ['shared-state'],
    },
    {
      // Specs that change household-wide sign-in state, run last.
      name: 'pin-change',
      use: { browserName: 'chromium' },
      testMatch: /admin-pin-change|sign-in-options/,
      dependencies: ['main', 'shared-state', 'shared-state-friends'],
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
