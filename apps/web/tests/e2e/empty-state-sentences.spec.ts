import { test, expect } from './fixtures/aim-test-stack';
import type { Page } from '@playwright/test';

// Three sentences a user reads on the dashboard are punctuated with a full
// stop, a colon or commas, not an em dash. tests/sentences-carry-no-em-dash.test.ts
// reads the component sources; this spec reads the sentences as the browser
// renders them, at a phone width and a desktop width. Each locator matches the
// whole sentence exactly, so the sentence with an em dash in it does not match.
const CHECKIN_SENTENCE =
  'Run the install commands below. The moment your agent calls in, it appears here with its identity and trust score, and this page fills in on its own.';
const DRIFT_DESCRIPTION =
  'Behavioral drift signal: runtime vs registered baseline (0% clean, 100% maximum drift)';
const ISOLATION_DESCRIPTION =
  'Self-reported runtime isolation posture (sandbox, network, filesystem, process). Nothing verifies the report, so it is capped at 0.65, the score of an ordinary hardened container, no matter how strong a posture is claimed. Reports expire after 90 days, and an agent that has never reported sits at a low 0.3 baseline.';

// The overview shows the live check-in panel only while the organisation has
// no agents. The other specs register agents in the same organisation, in
// parallel, so the agent list and the dashboard count are answered as empty
// here; every other request reaches the backend.
async function answerAsAnOrganisationWithNoAgents(page: Page) {
  await page.route(
    (url) => url.pathname === '/api/v1/agents',
    async (route) => {
      if (route.request().method() !== 'GET') return route.fallback();
      const response = await route.fetch();
      await route.fulfill({ response, json: { ...(await response.json()), agents: [] } });
    },
  );
  await page.route(
    (url) => url.pathname === '/api/v1/analytics/dashboard',
    async (route) => {
      const response = await route.fetch();
      await route.fulfill({ response, json: { ...(await response.json()), totalAgents: 0 } });
    },
  );
}

for (const width of [375, 1280]) {
  test.describe(`Dashboard sentences at ${width} px wide`, () => {
    test.use({ viewport: { width, height: 900 } });

    test('the overview zero state asks for the install commands in two sentences', async ({ authedPage }) => {
      await answerAsAnOrganisationWithNoAgents(authedPage);
      await authedPage.goto('/dashboard');
      await expect(authedPage.getByText('Listening for the first check-in')).toBeVisible();
      await expect(authedPage.getByText(CHECKIN_SENTENCE, { exact: true })).toBeVisible();
    });

    test("an agent's drift card and execution isolation factor read without an em dash", async ({
      authedPage,
      registerAgent,
    }) => {
      const agent = await registerAgent();
      await authedPage.goto(`/dashboard/agents/${agent.id}?tab=trust`);
      const trust = authedPage.getByRole('region', { name: /trust score/i });
      await expect(trust).toBeVisible();
      await expect(trust.getByText(DRIFT_DESCRIPTION, { exact: true })).toBeVisible();

      // The factor's description is the tooltip of its info button, which
      // opens when the button takes focus. A hover lands wherever the pointer
      // is once the cards above it have loaded, so the spec focuses instead.
      await trust.getByRole('button', { name: 'About execution isolation' }).focus();
      // Radix renders the content once on screen and once more inside the
      // tooltip role for screen readers; the first match is the one on screen.
      const popper = authedPage.locator('[data-radix-popper-content-wrapper]');
      await expect(popper.getByText(ISOLATION_DESCRIPTION, { exact: true }).first()).toBeVisible();
      await expect(authedPage.getByRole('tooltip')).toHaveText(ISOLATION_DESCRIPTION);
    });
  });
}
