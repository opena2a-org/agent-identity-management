import { test, expect } from './fixtures/aim-test-stack';
import type { Page } from '@playwright/test';

// Universal post-render assertion: the loading skeleton has resolved.
// The dashboard uses Radix `<Alert>` components (role="alert") for empty-
// state UIs themselves (e.g. "No Capabilities Detected"), so a zero-count
// on role=alert would catch the very thing the positive assertions verify.
// Words like "error" / "failed" appear in legitimate dashboard copy too
// (e.g. activity counters). The positive empty-state text assertions per
// tab already prove the page rendered without crashing; this helper just
// ensures the skeleton state was reached and replaced.
async function assertNoErrorState(page: Page) {
  await expect(page.locator('.animate-pulse')).toHaveCount(0);
}

async function gotoAgentTab(page: Page, agentId: string, tab: string) {
  await page.goto(`/dashboard/agents/${agentId}?tab=${tab}`);
  await page.waitForLoadState('networkidle');
}

test.describe('Agent detail page — empty-state rendering on fresh agent', () => {
  test('connections tab renders "No MCP Servers Detected"', async ({ authedPage, registerAgent }) => {
    const agent = await registerAgent();
    await gotoAgentTab(authedPage, agent.id, 'connections');
    await expect(authedPage.getByText('No MCP Servers Detected')).toBeVisible();
    await assertNoErrorState(authedPage);
  });

  test('capabilities tab renders "No Capabilities Detected"', async ({ authedPage, registerAgent }) => {
    const agent = await registerAgent();
    await gotoAgentTab(authedPage, agent.id, 'capabilities');
    await expect(authedPage.getByText('No Capabilities Detected')).toBeVisible();
    await assertNoErrorState(authedPage);
  });

  test('violations tab renders clean-record empty state', async ({ authedPage, registerAgent }) => {
    const agent = await registerAgent();
    await gotoAgentTab(authedPage, agent.id, 'violations');
    await expect(
      authedPage.getByText('No violations found. This agent has a clean record!'),
    ).toBeVisible();
    await assertNoErrorState(authedPage);
  });

  test('key-vault tab indicates no PQC key registered', async ({ authedPage, registerAgent }) => {
    const agent = await registerAgent();
    await gotoAgentTab(authedPage, agent.id, 'key-vault');
    await expect(
      authedPage.getByText(/No post-quantum public key registered/i),
    ).toBeVisible();
    await assertNoErrorState(authedPage);
  });

  test('api-keys tab renders "No API Keys"', async ({ authedPage, registerAgent }) => {
    const agent = await registerAgent();
    await gotoAgentTab(authedPage, agent.id, 'api-keys');
    await expect(authedPage.getByText('No API Keys', { exact: true })).toBeVisible();
    await assertNoErrorState(authedPage);
  });

  test('tags tab renders "No tags assigned"', async ({ authedPage, registerAgent }) => {
    const agent = await registerAgent();
    await gotoAgentTab(authedPage, agent.id, 'tags');
    await expect(authedPage.getByText('No tags assigned')).toBeVisible();
    await assertNoErrorState(authedPage);
  });

  test('activity tab renders "No activity recorded for this agent yet."', async ({
    authedPage,
    registerAgent,
  }) => {
    const agent = await registerAgent();
    await gotoAgentTab(authedPage, agent.id, 'activity');
    await expect(
      authedPage.getByText('No activity recorded for this agent yet.'),
    ).toBeVisible();
    await assertNoErrorState(authedPage);
  });

  test('trust tab renders "No historical data available yet"', async ({
    authedPage,
    registerAgent,
  }) => {
    const agent = await registerAgent();
    await gotoAgentTab(authedPage, agent.id, 'trust');
    // A freshly-created agent has no trust history yet. The breakdown panel
    // may render a default score, but the history sub-panel must surface
    // the empty state — that's the contract this test pins.
    await expect(
      authedPage.getByText(/No historical data available yet/i),
    ).toBeVisible();
    await assertNoErrorState(authedPage);
  });

  // details tab is N/A-by-design for empty-state copy: the agent's own
  // metadata (displayName, registration timestamps) is always populated.
  // Contract for this panel: it RENDERS, no error boundary, no crash.
  test('details tab renders agent metadata without error', async ({
    authedPage,
    registerAgent,
  }) => {
    const agent = await registerAgent();
    await gotoAgentTab(authedPage, agent.id, 'details');
    // The display name we registered the agent under must surface somewhere.
    await expect(authedPage.getByText(new RegExp(agent.name, 'i')).first()).toBeVisible();
    await assertNoErrorState(authedPage);
  });
});
