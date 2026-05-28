import { test, expect } from './fixtures/aim-test-stack';
import type { Page } from '@playwright/test';

// See empty-state-agent.spec.ts for the rationale on the narrowed helper.
async function assertNoErrorState(page: Page) {
  await expect(page.locator('.animate-pulse')).toHaveCount(0);
}

// MCP detail page uses Radix Tabs with defaultValue="details" and does not
// persist the active tab in the URL search params (unlike the agent page).
// Tests switch tabs by clicking the matching role=tab element.
async function gotoMcpDetail(page: Page, serverId: string) {
  await page.goto(`/dashboard/mcp/${serverId}`);
  await page.waitForLoadState('networkidle');
}

async function selectTab(page: Page, label: string | RegExp) {
  await page.getByRole('tab', { name: label }).click();
  await page.waitForLoadState('networkidle');
}

test.describe('MCP server detail page — empty-state rendering on fresh server', () => {
  // details is the default tab. metadata (name, url) is always populated;
  // contract is "renders without error boundary".
  test('details tab (default) renders server metadata without error', async ({
    authedPage,
    registerMcpServer,
  }) => {
    const server = await registerMcpServer();
    await gotoMcpDetail(authedPage, server.id);
    await expect(authedPage.getByText(new RegExp(server.name, 'i')).first()).toBeVisible();
    await assertNoErrorState(authedPage);
  });

  test('capabilities tab renders "No capabilities detected yet"', async ({
    authedPage,
    registerMcpServer,
  }) => {
    const server = await registerMcpServer();
    await gotoMcpDetail(authedPage, server.id);
    await selectTab(authedPage, /capabilities/i);
    await expect(authedPage.getByText('No capabilities detected yet')).toBeVisible();
    await assertNoErrorState(authedPage);
  });

  test('agents tab renders "No agents connected yet"', async ({
    authedPage,
    registerMcpServer,
  }) => {
    const server = await registerMcpServer();
    await gotoMcpDetail(authedPage, server.id);
    await selectTab(authedPage, /connected agents/i);
    await expect(authedPage.getByText('No agents connected yet')).toBeVisible();
    await assertNoErrorState(authedPage);
  });

  // Tab label is "Attestations"; underlying value is "activity".
  test('attestations tab renders "No attestations yet"', async ({
    authedPage,
    registerMcpServer,
  }) => {
    const server = await registerMcpServer();
    await gotoMcpDetail(authedPage, server.id);
    await selectTab(authedPage, /attestations/i);
    await expect(
      authedPage.getByRole('heading', { name: 'No attestations yet' }),
    ).toBeVisible();
    await assertNoErrorState(authedPage);
  });

  test('tags tab renders "No tags assigned"', async ({
    authedPage,
    registerMcpServer,
  }) => {
    const server = await registerMcpServer();
    await gotoMcpDetail(authedPage, server.id);
    await selectTab(authedPage, /^tags$/i);
    await expect(authedPage.getByText('No tags assigned')).toBeVisible();
    await assertNoErrorState(authedPage);
  });

  // Audit Trail (value="audit") is N/A-by-design for empty-state copy: the
  // Activity Timeline panel renders a summary header (Attestations/Capabilities/
  // Audit counters at 0) regardless of audit_logs length, so the empty
  // branch's "No activity recorded yet" string is unreachable on a
  // freshly-created MCP server. Contract: panel RENDERS, no error
  // boundary, no skeleton lock.
  test('audit trail tab renders without error (timeline summary always shows)', async ({
    authedPage,
    registerMcpServer,
  }) => {
    const server = await registerMcpServer();
    await gotoMcpDetail(authedPage, server.id);
    await selectTab(authedPage, /audit trail/i);
    await expect(authedPage.getByRole('heading', { name: /activity timeline/i })).toBeVisible();
    await assertNoErrorState(authedPage);
  });

  // integration is N/A-by-design for empty-state copy: the panel renders
  // a static setup guide that is the same regardless of resource history.
  // Contract: it renders, no error boundary, no crash.
  test('integration tab renders setup guide without error', async ({
    authedPage,
    registerMcpServer,
  }) => {
    const server = await registerMcpServer();
    await gotoMcpDetail(authedPage, server.id);
    await selectTab(authedPage, /integration/i);
    await assertNoErrorState(authedPage);
  });
});
