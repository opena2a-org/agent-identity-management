import { test, expect, type Page } from '@playwright/test';

// The MCP server detail page at a phone viewport under mobile emulation: the
// viewport meta applies and the pointer is a touch. When the page's content is
// wider than the device, Chromium lays the page out at the content's width and
// shows it zoomed out: window.innerWidth reports that width and a tap at a tab's
// centre lands on another element. The page is loaded as a signed-in admin over
// route mocks, with the server's name and URL pinned, since the header's width
// follows them; no backend is needed.

const SERVER_ID = '00000000-0000-4000-8000-000000000001';
const SERVER = {
  id: SERVER_ID,
  organizationId: SERVER_ID,
  name: 'Sample MCP server',
  url: 'https://mcp-server.example.com/mcp/v1/sse',
  description: 'Fixture MCP server for the phone layout test',
  status: 'verified',
  isVerified: true,
  trustScore: 0.82,
  lastVerifiedAt: '2026-01-01T00:00:00.000Z',
  createdAt: '2026-01-01T00:00:00.000Z',
  updatedAt: '2026-01-01T00:00:00.000Z',
};
const VIEWPORT = { width: 375, height: 812 };

// Unsigned session token: the dashboard only decodes the payload (lib/jwt-payload.ts)
// to pass its route gate; the API that would verify it is mocked below.
function sessionToken(): string {
  const b64url = (o: Record<string, unknown>) => Buffer.from(JSON.stringify(o)).toString('base64url');
  const now = Math.floor(Date.now() / 1000);
  const header = b64url({ alg: 'none', typ: 'JWT' });
  const payload = b64url({ user_id: SERVER_ID, organization_id: SERVER_ID, email: 'admin@example.com', role: 'admin', iat: now, exp: now + 8 * 3600 });
  return `${header}.${payload}.`;
}

// The answers the page and the dashboard shell read; every other /api/v1 call gets {}.
const MOCKS: Array<[RegExp, unknown]> = [
  [/^\/api\/v1\/mcp-servers\/[^/]+$/, SERVER],
  [/^\/api\/v1\/mcp-servers\/[^/]+\/capabilities$/, { capabilities: [], total: 0 }],
  [/^\/api\/v1\/mcp-servers\/[^/]+\/agents$/, { agents: [], total: 0 }],
  [/^\/api\/v1\/mcp-servers\/[^/]+\/attestations$/, { attestations: [], total: 0 }],
  [/^\/api\/v1\/mcp-servers\/[^/]+\/audit-logs/, { logs: [], total: 0 }],
  [/^\/api\/v1\/mcp-servers\/[^/]+\/tags$/, { tags: [] }],
  [/^\/api\/v1\/agents$/, { agents: [] }],
  [/^\/api\/v1\/auth\/me$/, { id: SERVER_ID, email: 'admin@example.com', name: 'Admin', role: 'admin', organizationId: SERVER_ID }],
];

test.use({ viewport: VIEWPORT, hasTouch: true, isMobile: true });

const widths = (page: Page) =>
  page.evaluate(() => ({
    innerWidth: window.innerWidth,
    scrollWidth: document.documentElement.scrollWidth,
  }));

async function openServerPage(page: Page) {
  await page.addInitScript(
    ({ token, now }) => {
      localStorage.setItem('auth_token', token);
      localStorage.setItem('session_start', now);
      localStorage.setItem('last_activity', now);
    },
    { token: sessionToken(), now: String(Date.now()) },
  );
  // The API can sit on another origin than the dashboard (NEXT_PUBLIC_API_URL), so
  // every mocked answer carries CORS headers and a preflight is answered as well.
  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const headers = {
      'access-control-allow-origin': '*',
      'access-control-allow-headers': 'authorization, content-type',
      'access-control-allow-methods': 'GET, POST, PUT, PATCH, DELETE, OPTIONS',
    };
    if (request.method() === 'OPTIONS') {
      await route.fulfill({ status: 204, headers });
      return;
    }
    const { pathname } = new URL(request.url());
    const body = request.method() === 'GET' ? (MOCKS.find(([path]) => path.test(pathname))?.[1] ?? {}) : {};
    await route.fulfill({ status: 200, headers, contentType: 'application/json', body: JSON.stringify(body) });
  });
  await page.goto(`/dashboard/mcp/${SERVER_ID}`);
  await expect(page.getByRole('heading', { name: SERVER.name })).toBeVisible();
  await expect(page.getByRole('tab', { name: 'Integration' })).toBeVisible();
}

test('the MCP server page keeps the device width and its Integration tab opens on a tap', async ({ page }) => {
  await openServerPage(page);

  expect(await widths(page), 'the page as loaded').toEqual({ innerWidth: VIEWPORT.width, scrollWidth: VIEWPORT.width });

  const tabs = await page.getByRole('tablist').boundingBox();
  if (!tabs) throw new Error('the tab list has no box');
  expect(tabs.x + tabs.width, 'the tab list ends inside the viewport').toBeLessThanOrEqual(VIEWPORT.width);

  const integration = page.getByRole('tab', { name: 'Integration' });
  await integration.tap();
  await expect(integration).toHaveAttribute('aria-selected', 'true');
  const panel = page.getByRole('tabpanel', { name: 'Integration' });
  await expect(panel).toContainText('Query via cURL');

  expect(await widths(page), 'the page on the Integration tab').toEqual({ innerWidth: VIEWPORT.width, scrollWidth: VIEWPORT.width });

  const curl = panel.locator('pre').first();
  const box = await curl.boundingBox();
  if (!box) throw new Error('the cURL example has no box');
  expect(box.x + box.width, 'the cURL example ends inside the viewport').toBeLessThanOrEqual(VIEWPORT.width);
  const scrolls = await curl.evaluate((pre) => pre.scrollWidth > pre.clientWidth);
  expect(scrolls, 'the cURL example scrolls inside its card').toBe(true);
});
