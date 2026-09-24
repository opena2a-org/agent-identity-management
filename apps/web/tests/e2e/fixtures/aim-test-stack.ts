import { test as base, request as playwrightRequest, type APIRequestContext, type Page } from '@playwright/test';
import { randomUUID } from 'node:crypto';

const ADMIN_EMAIL = process.env.AIM_TEST_ADMIN_EMAIL ?? 'admin@opena2a.org';
const ADMIN_PASSWORD = process.env.AIM_TEST_ADMIN_PASSWORD ?? process.env.DEFAULT_ADMIN_PASSWORD ?? '';

type Resource = { id: string; name: string };

type WorkerFixtures = {
  // Worker-scoped: one admin login per worker. The backend rate-limits the
  // /api/v1/auth/login/local handler; logging in per-test (test-scoped) trips
  // the limiter once a worker runs more than a handful of tests in a window.
  adminAuth: { accessToken: string; refreshToken: string };
};

type TestFixtures = {
  registerAgent: (overrides?: { displayName?: string }) => Promise<Resource>;
  registerMcpServer: (overrides?: { url?: string }) => Promise<Resource>;
  authedPage: Page;
};

function freshSuffix(): string {
  return randomUUID().replace(/-/g, '').slice(0, 12);
}

async function login(request: APIRequestContext): Promise<{ accessToken: string; refreshToken: string }> {
  if (!ADMIN_PASSWORD) {
    throw new Error(
      'AIM_TEST_ADMIN_PASSWORD (or DEFAULT_ADMIN_PASSWORD) must be set to the seeded admin password.',
    );
  }
  const res = await request.post('/api/v1/auth/login/local', {
    data: { email: ADMIN_EMAIL, password: ADMIN_PASSWORD },
  });
  if (!res.ok()) {
    throw new Error(`login/local failed: ${res.status()} ${await res.text()}`);
  }
  const body = await res.json();
  return { accessToken: body.accessToken, refreshToken: body.refreshToken };
}

export const test = base.extend<TestFixtures, WorkerFixtures>({
  adminAuth: [
    async ({}, use) => {
      // Worker-scoped fixtures can't depend on test-scoped ones (like
      // `request`), so the login uses a freshly-built APIRequestContext.
      const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? 'http://localhost:3000';
      const ctx = await playwrightRequest.newContext({ baseURL });
      try {
        const tokens = await login(ctx);
        await use(tokens);
      } finally {
        await ctx.dispose();
      }
    },
    { scope: 'worker' },
  ],

  registerAgent: async ({ request, adminAuth }, use) => {
    const register = async (overrides?: { displayName?: string }): Promise<Resource> => {
      const name = `e2e-agent-${freshSuffix()}`;
      const res = await request.post('/api/v1/agents/', {
        headers: { Authorization: `Bearer ${adminAuth.accessToken}` },
        data: {
          name,
          displayName: overrides?.displayName ?? `E2E Agent ${name}`,
          description: 'Fixture-registered agent for empty-state e2e',
          agentType: 'ai_agent',
        },
      });
      if (!res.ok()) {
        throw new Error(`create agent failed: ${res.status()} ${await res.text()}`);
      }
      const body = await res.json();
      return { id: body.id, name };
    };
    await use(register);
  },

  registerMcpServer: async ({ request, adminAuth }, use) => {
    const register = async (overrides?: { url?: string }): Promise<Resource> => {
      const name = `e2e-mcp-${freshSuffix()}`;
      const res = await request.post('/api/v1/mcp-servers/', {
        headers: { Authorization: `Bearer ${adminAuth.accessToken}` },
        data: {
          name,
          description: 'Fixture-registered MCP server for empty-state e2e',
          url: overrides?.url ?? `https://example.test/${name}`,
        },
      });
      if (!res.ok()) {
        throw new Error(`create mcp-server failed: ${res.status()} ${await res.text()}`);
      }
      const body = await res.json();
      return { id: body.id, name };
    };
    await use(register);
  },

  // authedPage seeds the session store the dashboard reads: localStorage
  // auth_token and refresh_token. The shell's route gate and the API client
  // both read that store; no cookie is involved.
  authedPage: async ({ page, adminAuth }, use) => {
    await page.addInitScript(({ accessToken, refreshToken }) => {
      try {
        window.localStorage.setItem('auth_token', accessToken);
        window.localStorage.setItem('refresh_token', refreshToken);
      } catch {}
    }, { accessToken: adminAuth.accessToken, refreshToken: adminAuth.refreshToken });
    await use(page);
  },
});

export { expect } from '@playwright/test';
