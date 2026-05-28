import { defineConfig, devices } from '@playwright/test';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL ?? 'http://localhost:3000';

export default defineConfig({
  testDir: './tests/e2e',
  // Only the empty-state suite is wired into CI under this PR. The existing
  // landing-page / dashboard / agent-registration specs predate this work,
  // use mocked routes + a fake JWT, and were never run by any CI job — under
  // the real middleware (apps/web/middleware.ts) they redirect to /auth/login
  // before any mocked route fires. Rehabilitating them is out of scope here;
  // a follow-up PR can refit them to the aim-test-stack fixture pattern.
  testMatch: '**/empty-state-*.spec.ts',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 2 : undefined,
  reporter: process.env.CI
    ? [['list'], ['html', { open: 'never' }], ['github']]
    : [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: BASE_URL,
    trace: 'retain-on-failure',
    video: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  // Only start `next dev` when not already running. In CI the workflow starts
  // both the Go backend and `next dev` before invoking playwright, so the
  // server is already up and reuseExistingServer takes effect.
  webServer: {
    command: 'npm run dev',
    url: BASE_URL,
    reuseExistingServer: true,
    timeout: 120_000,
    stdout: 'pipe',
    stderr: 'pipe',
  },
});
