// Progressive-load smoke for /dashboard/a2a.
//
// Verifies, as a real browser user, that the A2A page paints section-by-section
// instead of blocking the whole page on the slowest API call, and that the
// consent filter debounces. The API is stubbed in-browser so response timing
// can be controlled — no backend needed.
//
// IMPORTANT: run against a PRODUCTION build (`next build` + `next start`), NOT
// `next dev`. Under dev-mode StrictMode/HMR the dashboard tab content does not
// render deterministically, which is a dev-server artifact, not how users see
// the app. The committed Playwright e2e suite (tests/e2e, dev webServer) is
// therefore the wrong host for this timing-sensitive check; this standalone
// prod smoke is.
//
// Usage:
//   npm run build
//   npx next start -p 3737 &            # serve the production build
//   BASE_URL=http://localhost:3737 node scripts/smoke-a2a-progressive.mjs
//
// Exit codes: 0 = both checks passed, 1 = a check failed.

import { chromium } from 'playwright';

const BASE = process.env.BASE_URL ?? 'http://localhost:3737';
const b64url = (o) => Buffer.from(JSON.stringify(o)).toString('base64url');
const JWT = `${b64url({ alg: 'HS256', typ: 'JWT' })}.${b64url({
  sub: 'smoke', email: 'admin@opena2a.org', role: 'admin', organizationId: 'org-smoke',
  exp: Math.floor(Date.now() / 1000) + 3600,
})}.sig`;
const USER = {
  id: 'u-smoke', email: 'admin@opena2a.org', name: 'Smoke Admin', firstName: 'Smoke', lastName: 'Admin',
  role: 'admin', organizationId: 'org-smoke', organizationName: 'Smoke Org',
};

async function newAuthedContext(browser) {
  const ctx = await browser.newContext();
  // Middleware gates /dashboard on the access_token cookie; the API client
  // reads auth_token from localStorage.
  await ctx.addCookies([{ name: 'access_token', value: JWT, domain: new URL(BASE).hostname, path: '/' }]);
  await ctx.addInitScript((jwt) => {
    localStorage.setItem('auth_token', jwt);
    localStorage.setItem('refresh_token', jwt);
  }, JWT);
  return ctx;
}

async function progressiveCheck(browser) {
  const ctx = await newAuthedContext(browser);
  const page = await ctx.newPage();
  const cards = [
    { id: 'c1', agentId: 'a1', name: 'Agent One', verified: true, aimAttestation: true, skills: [{ id: 's1' }] },
    { id: 'c2', agentId: 'a2', name: 'Agent Two', verified: false, aimAttestation: false, skills: [] },
  ];
  const tasks = [{ id: 't1', state: 'COMPLETED' }, { id: 't2', state: 'WORKING' }];
  const scores = [{ agentId: 'a1', agentName: 'Agent One', a2aTrustScore: 0.95, totalTasksAsClient: 2, totalTasksAsRemote: 0, uniquePeersCount: 1 }];

  let releaseTasks;
  const tasksGate = new Promise((r) => { releaseTasks = r; });
  await page.route('**/api/v1/**', async (route) => {
    const url = route.request().url();
    const ok = (body) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
    if (url.includes('/a2a/cards')) return ok({ cards, limit: 100, offset: 0 });
    if (url.includes('/a2a/tasks')) { await tasksGate; return ok({ tasks }); } // held pending
    if (url.includes('/a2a/consents')) return ok({ consents: [], total: 0 });
    if (url.includes('/a2a/trust-scores')) return ok({ scores });
    if (url.includes('/auth/me')) return ok(USER);
    return ok({});
  });

  await page.goto(`${BASE}/dashboard/a2a`, { waitUntil: 'domcontentloaded' });
  // Overview's task panel mounts (heading is outside the loading conditional).
  await page.getByRole('heading', { name: 'Task State Distribution' }).waitFor({ timeout: 15000 });
  const taskBox = page.locator('div').filter({ has: page.getByText('Task State Distribution') }).last();
  // Cards stat grid painted ("AIM Attested" is unique to it) while the task
  // chart still shows its own spinner -> progressive.
  const statsPainted = await page.getByText('AIM Attested').isVisible();
  const spinnerWhilePending = await taskBox.locator('.animate-spin').count();
  const chartWhilePending = await taskBox.locator('.recharts-wrapper').count();
  // Release tasks: the chart fills, its spinner clears.
  releaseTasks();
  await taskBox.locator('.recharts-wrapper').waitFor({ timeout: 10000 });
  const spinnerAfter = await taskBox.locator('.animate-spin').count();

  await ctx.close();
  const pass = statsPainted && spinnerWhilePending === 1 && chartWhilePending === 0 && spinnerAfter === 0;
  console.log(`[progressive] statsPainted=${statsPainted} spinnerWhilePending=${spinnerWhilePending} chartWhilePending=${chartWhilePending} spinnerAfter=${spinnerAfter} -> ${pass ? 'PASS' : 'FAIL'}`);
  return pass;
}

async function debounceCheck(browser) {
  const ctx = await newAuthedContext(browser);
  const page = await ctx.newPage();
  let filteredRequests = 0;
  await page.route('**/api/v1/**', async (route) => {
    const url = route.request().url();
    const ok = (body) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
    if (url.includes('/a2a/consent/user/')) { filteredRequests++; return ok({ consents: [], count: 0 }); }
    if (url.includes('/a2a/cards')) return ok({ cards: [], limit: 100, offset: 0 });
    if (url.includes('/a2a/tasks')) return ok({ tasks: [] });
    if (url.includes('/a2a/consents')) return ok({ consents: [], total: 0 });
    if (url.includes('/a2a/trust-scores')) return ok({ scores: [] });
    if (url.includes('/auth/me')) return ok(USER);
    return ok({});
  });

  await page.goto(`${BASE}/dashboard/a2a`, { waitUntil: 'domcontentloaded' });
  await page.getByRole('button', { name: 'Consent' }).click();
  const input = page.getByPlaceholder('Search by user ID...');
  await input.waitFor({ timeout: 15000 });
  await input.pressSequentially('user42', { delay: 25 }); // 6 keystrokes inside the 350ms debounce
  await page.waitForTimeout(900);

  await ctx.close();
  const pass = filteredRequests === 1; // 6 keystrokes -> 1 request
  console.log(`[debounce] keystrokes=6 filteredRequests=${filteredRequests} (expected 1) -> ${pass ? 'PASS' : 'FAIL'}`);
  return pass;
}

const browser = await chromium.launch();
let ok = true;
try {
  ok = (await progressiveCheck(browser)) && ok;
  ok = (await debounceCheck(browser)) && ok;
} finally {
  await browser.close();
}
console.log(ok ? '\n==> PASS: A2A progressive-load + debounce verified' : '\n==> FAIL');
process.exit(ok ? 0 : 1);
