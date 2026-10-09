import { test, expect } from './fixtures/aim-test-stack';
import type { APIRequestContext, Locator, Page } from '@playwright/test';

// Delete agent, from the agents list and from the agent page, against a real backend and
// Postgres. The agent being deleted is the client of an A2A task, the row that made
// DELETE /api/v1/agents/:id fail before the A2A references followed the agent.
//
// Every case runs at a phone and a desktop width, in light and dark mode.

const GONE_REASON = 'This agent no longer exists in your organization.';

const VIEWPORTS = [
  { label: '375px', size: { width: 375, height: 812 } },
  { label: '1280px', size: { width: 1280, height: 800 } },
] as const;

const THEMES = ['light', 'dark'] as const;

type AgentRef = { id: string; name: string; displayName: string };

function bearer(token: string) {
  return { Authorization: `Bearer ${token}` };
}

async function logA2ATask(
  request: APIRequestContext,
  token: string,
  clientAgentId: string,
  remoteAgentId: string,
): Promise<void> {
  const res = await request.post('/api/v1/a2a/tasks', {
    headers: bearer(token),
    data: {
      clientAgentId,
      remoteAgentId,
      externalTaskId: `e2e-task-${clientAgentId}`,
      contextId: 'e2e-delete-agent',
      skillId: 'e2e-delete-agent',
    },
  });
  if (res.status() !== 201) {
    throw new Error(`log A2A task failed: ${res.status()} ${await res.text()}`);
  }
}

async function agentStatus(request: APIRequestContext, token: string, id: string): Promise<number> {
  const res = await request.get(`/api/v1/agents/${id}`, { headers: bearer(token) });
  return res.status();
}

// next-themes keeps the pick in localStorage under `theme` and sets the `dark` class on
// <html>; the provider does not follow the OS preference.
async function useTheme(page: Page, theme: (typeof THEMES)[number]) {
  await page.addInitScript((value) => {
    try {
      window.localStorage.setItem('theme', value);
    } catch {}
  }, theme);
}

async function expectTheme(page: Page, theme: (typeof THEMES)[number]) {
  const html = page.locator('html');
  if (theme === 'dark') {
    await expect(html).toHaveClass(/(^|\s)dark(\s|$)/);
  } else {
    await expect(html).not.toHaveClass(/(^|\s)dark(\s|$)/);
  }
}

async function openFromList(page: Page, agent: AgentRef): Promise<Locator> {
  await page.goto('/dashboard/agents');
  await page.getByPlaceholder('Search agents by name...').fill(agent.name);
  const row = page.locator('tr', { hasText: agent.name });
  await expect(row).toHaveCount(1);
  await row.getByRole('button', { name: 'Delete agent' }).click();
  return page.getByRole('alertdialog', { name: 'Delete agent' });
}

async function openFromAgentPage(page: Page, agent: AgentRef): Promise<Locator> {
  await page.goto(`/dashboard/agents/${agent.id}`);
  await page.getByRole('button', { name: 'Delete', exact: true }).click();
  return page.getByRole('alertdialog', { name: 'Delete agent' });
}

const ENTRY_POINTS = [
  { label: 'agents list', open: openFromList },
  { label: 'agent page', open: openFromAgentPage },
] as const;

async function expectDialogText(dialog: Locator, agent: AgentRef) {
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText(
    `Deleting "${agent.displayName}" removes the agent and the records that belong to it`,
  );
  await expect(dialog).toContainText('the A2A tasks, messages and consent records it took part in');
  await expect(dialog).toContainText(
    'Audit log entries, API call records, A2A security violations and the MCP servers it registered are kept.',
  );
  await expect(dialog).toContainText('This cannot be undone.');
}

for (const viewport of VIEWPORTS) {
  for (const theme of THEMES) {
    test.describe(`Delete agent at ${viewport.label}, ${theme} mode`, () => {
      test.use({ viewport: viewport.size });

      for (const entry of ENTRY_POINTS) {
        test(`from the ${entry.label}, an agent with an A2A task is deleted and the list says so`, async ({
          authedPage,
          request,
          adminAuth,
          registerAgent,
        }) => {
          const created = await registerAgent();
          const agent = { ...created, displayName: `E2E Agent ${created.name}` };
          const peer = await registerAgent();
          await logA2ATask(request, adminAuth.accessToken, agent.id, peer.id);

          await useTheme(authedPage, theme);
          const dialog = await entry.open(authedPage, agent);
          await expectTheme(authedPage, theme);
          await expectDialogText(dialog, agent);

          await dialog.getByRole('button', { name: 'Delete', exact: true }).click();

          await expect(dialog).toBeHidden();
          await expect(authedPage).toHaveURL(/\/dashboard\/agents\/?$/);
          await expect(
            authedPage.getByRole('status').filter({ hasText: `Agent "${agent.displayName}" was deleted.` }),
          ).toBeVisible();
          await authedPage.getByPlaceholder('Search agents by name...').fill(agent.name);
          await expect(authedPage.locator('tr', { hasText: agent.name })).toHaveCount(0);

          expect(await agentStatus(request, adminAuth.accessToken, agent.id)).toBe(404);
          // The other side of the task is not reached by the delete.
          expect(await agentStatus(request, adminAuth.accessToken, peer.id)).toBe(200);
        });

        test(`from the ${entry.label}, a delete that fails keeps the dialog open and states the reason in it`, async ({
          authedPage,
          request,
          adminAuth,
          registerAgent,
        }) => {
          const created = await registerAgent();
          const agent = { ...created, displayName: `E2E Agent ${created.name}` };

          await useTheme(authedPage, theme);
          const dialog = await entry.open(authedPage, agent);
          await expectTheme(authedPage, theme);
          await expectDialogText(dialog, agent);

          // Deleted elsewhere while the dialog is open: the route answers 404, which the
          // dialog states as an agent that no longer exists, with a link to the list.
          const res = await request.delete(`/api/v1/agents/${agent.id}`, {
            headers: bearer(adminAuth.accessToken),
          });
          expect(res.status()).toBe(204);

          await dialog.getByRole('button', { name: 'Delete', exact: true }).click();

          const failure = dialog.getByRole('alert');
          await expect(failure).toContainText(GONE_REASON);
          await expect(failure.getByRole('link', { name: 'Back to agents' })).toHaveAttribute(
            'href',
            '/dashboard/agents',
          );
          await expect(dialog).toBeVisible();
          expect(
            await dialog.evaluate((node) => node.contains(document.activeElement)),
          ).toBe(true);
          await expect(dialog.getByRole('button', { name: 'Delete', exact: true })).not.toHaveAttribute(
            'aria-disabled',
            'true',
          );
          await expect(authedPage.getByText(`Agent "${agent.displayName}" was deleted.`)).toHaveCount(0);
        });
      }
    });
  }
}
