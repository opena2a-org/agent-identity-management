import { test as base, type APIRequestContext, type Page } from '@playwright/test';
import { randomUUID } from 'node:crypto';

const ADMIN_EMAIL = process.env.AIM_TEST_ADMIN_EMAIL ?? 'admin@opena2a.org';
const ADMIN_PASSWORD = process.env.AIM_TEST_ADMIN_PASSWORD ?? process.env.DEFAULT_ADMIN_PASSWORD ?? '';

type Resource = { id: string; name: string };

type Fixtures = {
  adminAuth: { accessToken: string; refreshToken: string };
  registerAgent: (overrides?: { displayName?: string }) => Promise<Resource>;
  registerMcpServer: (overrides?: { url?: string }) => Promise<Resource>;
  authedPage: Page;
};

// freshSuffix is unique per test invocation so parallel workers never collide
// on the (organizationId, name) unique constraint on agents / mcp_servers.
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

export const test = base.extend<Fixtures>({
  adminAuth: async ({ request }, use) => {
    const tokens = await login(request);
    await use(tokens);
  },

  registerAgent: async ({ request, adminAuth }, use) => {
    const created: string[] = [];
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
      created.push(body.id);
      return { id: body.id, name };
    };
    await use(register);
  },

  registerMcpServer: async ({ request, adminAuth }, use) => {
    const created: string[] = [];
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
      created.push(body.id);
      return { id: body.id, name };
    };
    await use(register);
  },

  // authedPage seeds BOTH the cookie (for middleware.ts:34 which gates every
  // dashboard route on `access_token` in cookies) AND localStorage.auth_token
  // (for the client-side useAuth() hook which calls api.getToken()).
  //
  // Why both: Playwright's `request` fixture lives in its own APIRequestContext
  // with a separate cookie jar from the browser's BrowserContext, so cookies
  // set by /api/v1/auth/login/local during `adminAuth` do NOT propagate to
  // `page`. We have to write the access_token cookie onto page.context()
  // explicitly. middleware.ts redirects to /auth/login on every request that
  // lacks the cookie.
  authedPage: async ({ page, adminAuth }, use) => {
    await page.context().addCookies([
      {
        name: 'access_token',
        value: adminAuth.accessToken,
        domain: 'localhost',
        path: '/',
        httpOnly: true,
        secure: false,
        sameSite: 'Lax',
      },
      {
        name: 'refresh_token',
        value: adminAuth.refreshToken,
        domain: 'localhost',
        path: '/',
        httpOnly: true,
        secure: false,
        sameSite: 'Lax',
      },
    ]);
    await page.addInitScript((token) => {
      try { window.localStorage.setItem('auth_token', token); } catch {}
    }, adminAuth.accessToken);
    await use(page);
  },
});

export { expect } from '@playwright/test';
