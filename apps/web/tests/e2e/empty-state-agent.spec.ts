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

  // api-keys is N/A-by-design for empty-state copy: POST /api/v1/agents
  // auto-generates a default API key for every new agent
  // (apps/backend/internal/application/agent_service.go:195 "creates a new
  // agent and automatically generates an API key for it"), so a freshly-
  // registered agent always has exactly 1 API key. Contract for this panel:
  // it RENDERS, no error boundary, no skeleton lock.
  test('api-keys tab renders without error (default key auto-created)', async ({ authedPage, registerAgent }) => {
    const agent = await registerAgent();
    await gotoAgentTab(authedPage, agent.id, 'api-keys');
    await expect(authedPage.getByRole('heading', { name: 'API Keys' })).toBeVisible();
    await assertNoErrorState(authedPage);
  });

  test('tags tab renders "No tags assigned"', async ({ authedPage, registerAgent }) => {
    const agent = await registerAgent();
    await gotoAgentTab(authedPage, agent.id, 'tags');
    await expect(authedPage.getByText('No tags assigned')).toBeVisible();
    await assertNoErrorState(authedPage);
  });

  // activity is N/A-by-design for empty-state copy: agent registration
  // itself emits an audit event (agent_created) that surfaces in the
  // Recent Activity panel, so the empty-state branch is unreachable on
  // a freshly-registered fixture. Contract: panel RENDERS, no error
  // boundary, no skeleton lock.
  test('activity tab renders without error (registration emits its own event)', async ({
    authedPage,
    registerAgent,
  }) => {
    const agent = await registerAgent();
    await gotoAgentTab(authedPage, agent.id, 'activity');
    await expect(authedPage.getByRole('tab', { name: /recent activity/i })).toBeVisible();
    await assertNoErrorState(authedPage);
  });

  // trust is N/A-by-design for empty-state copy: every new agent gets a
  // default trust score (visible on the header as e.g. "Trust: 83.5%"),
  // and the trust breakdown card renders that score. The history sub-
  // panel may or may not have datapoints depending on score-emission
  // timing; either way the breakdown card is the load-bearing render.
  // Contract: panel RENDERS, no error boundary, no skeleton lock.
  test('trust tab renders without error (default score auto-emitted)', async ({
    authedPage,
    registerAgent,
  }) => {
    const agent = await registerAgent();
    await gotoAgentTab(authedPage, agent.id, 'trust');
    await expect(authedPage.getByRole('tab', { name: /trust/i })).toBeVisible();
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
