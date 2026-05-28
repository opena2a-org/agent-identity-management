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

  // authedPage seeds localStorage.auth_token (read by useAuth() via api.getToken())
  // before any navigation, so dashboard routes render without redirecting to /login.
  // Cookies set by /auth/login/local are already on the context via APIRequestContext.
  authedPage: async ({ page, adminAuth }, use) => {
    await page.addInitScript((token) => {
      try { window.localStorage.setItem('auth_token', token); } catch {}
    }, adminAuth.accessToken);
    await use(page);
  },
});

export { expect } from '@playwright/test';
