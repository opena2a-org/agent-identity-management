import { test, expect } from './fixtures/aim-test-stack';
import type { Page } from '@playwright/test';

// Two dashboard tabs whose access token the API refuses at the same moment, against a real
// backend and Postgres. With a revocation store configured, a login refresh token can be used
// once and presenting one a second time ends the sign-in, so the two tabs must send one
// POST /api/v1/auth/refresh between them and both carry on with the pair it returned. Without
// that store the API hands the same refresh token back, and one refresh is still what two
// tabs send.
//
// The tabs are two pages of one browser context: two documents that share one localStorage and
// one lock manager. The second case takes Web Locks away, as on a dashboard served over plain
// HTTP from a host other than localhost, where the tabs take the turn through a lease in
// localStorage.
//
// No assertion here compares a token with a matcher: a failure prints what it compared, and a
// token does not belong in a test log. Tokens are compared first and the outcome is asserted.

const REFRESH_PATH = '/api/v1/auth/refresh';
const DASHBOARD_PAGE = '/dashboard/agents';

// The client sends a tab to the sign-in page a second and a half after a refresh is refused.
const SIGN_OUT_DELAY_MS = 1_500;

type Pair = { accessToken: string | null; refreshToken: string | null };

// The same token under another signature. The dashboard reads its claims to open the page, and
// the API answers 401 to every request that carries it.
function withAnotherSignature(accessToken: string): string {
  const cut = accessToken.lastIndexOf('.') + 1;
  const first = accessToken[cut] === 'A' ? 'B' : 'A';
  return `${accessToken.slice(0, cut)}${first}${accessToken.slice(cut + 1)}`;
}

const storedPair = (tab: Page): Promise<Pair> =>
  tab.evaluate(() => ({
    accessToken: window.localStorage.getItem('auth_token'),
    refreshToken: window.localStorage.getItem('refresh_token'),
  }));

const CASES = [
  { name: 'with Web Locks', webLocks: true },
  { name: 'without Web Locks', webLocks: false },
] as const;

for (const { name, webLocks } of CASES) {
  test(`two tabs refused together send one refresh and stay signed in, ${name}`, async ({
    browser,
    baseURL,
    request,
    ownSession,
  }) => {
    // The first visit to a page of a development server compiles it.
    test.setTimeout(120_000);

    const refusedAccessToken = withAnotherSignature(ownSession.accessToken);
    const context = await browser.newContext({
      baseURL,
      storageState: {
        cookies: [],
        origins: [
          {
            origin: new URL(baseURL!).origin,
            localStorage: [
              { name: 'auth_token', value: refusedAccessToken },
              { name: 'refresh_token', value: ownSession.refreshToken },
            ],
          },
        ],
      },
    });
    try {
      if (!webLocks) {
        await context.addInitScript(() => {
          Object.defineProperty(Navigator.prototype, 'locks', { get: () => undefined, configurable: true });
        });
      }
      const tabs = [await context.newPage(), await context.newPage()];

      const refused = new Set<Page>();
      const carriedOn = new Set<Page>();
      const presented: unknown[] = [];
      const answers: Promise<Pair>[] = [];
      context.on('request', (sent) => {
        if (sent.method() === 'POST' && new URL(sent.url()).pathname === REFRESH_PATH) {
          presented.push(sent.postDataJSON()?.refreshToken);
        }
      });
      context.on('response', (response) => {
        const sent = response.request();
        const path = new URL(response.url()).pathname;
        if (path === REFRESH_PATH) {
          if (sent.method() === 'POST' && response.status() === 200) answers.push(response.json());
          return;
        }
        if (!path.startsWith('/api/v1/') || !sent.headers()['authorization']) return;
        const tab = sent.frame().page();
        if (response.status() === 401) refused.add(tab);
        if (response.status() === 200) carriedOn.add(tab);
      });
      // The answer to a refresh is kept back until both tabs have been refused and the later one
      // has had time to ask for its turn, so the two refreshes overlap.
      await context.route(`**${REFRESH_PATH}`, async (route) => {
        await expect.poll(() => refused.size, { timeout: 60_000 }).toBe(2);
        await new Promise((resolve) => setTimeout(resolve, 1_000));
        await route.continue();
      });

      await Promise.all(tabs.map((tab) => tab.goto(DASHBOARD_PAGE)));
      for (const tab of tabs) {
        expect(await tab.evaluate(() => typeof navigator.locks)).toBe(webLocks ? 'object' : 'undefined');
      }
      // The stored access token is refused, so a 200 is an answer to the one a refresh returned.
      await expect.poll(() => carriedOn.size, { timeout: 60_000 }).toBe(2);
      await tabs[0].waitForTimeout(2 * SIGN_OUT_DELAY_MS);

      expect(
        presented.map((token) => token === ownSession.refreshToken),
        'one refresh, presenting the stored refresh token',
      ).toEqual([true]);
      expect(answers).toHaveLength(1);
      const answer = await answers[0];
      expect(typeof answer.accessToken === 'string' && answer.accessToken !== refusedAccessToken).toBe(true);
      for (const tab of tabs) {
        expect(new URL(tab.url()).pathname).toBe(DASHBOARD_PAGE);
        const stored = await storedPair(tab);
        expect(
          {
            accessToken: stored.accessToken === answer.accessToken,
            refreshToken: stored.refreshToken === answer.refreshToken,
          },
          'the tab holds the pair the refresh returned',
        ).toEqual({ accessToken: true, refreshToken: true });
      }
      // The sign-in was not ended: the API still answers the access token the tabs hold.
      const me = await request.get('/api/v1/auth/me', {
        headers: { Authorization: `Bearer ${answer.accessToken}` },
      });
      expect(me.status()).toBe(200);
    } finally {
      // A refresh still kept back when the test ends is let go without an error.
      await context.unrouteAll({ behavior: 'ignoreErrors' });
      await context.close();
    }
  });
}
