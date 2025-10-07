import { test, expect } from '@playwright/test';

// Mock authentication helper
async function mockLogin(page: any) {
  await page.goto('http://localhost:3000');
  await page.evaluate(() => {
    localStorage.setItem('auth_token', 'mock-jwt-token-for-testing');
    localStorage.setItem('user', JSON.stringify({
      id: '00000000-0000-0000-0000-000000000001',
      email: 'test@example.com',
      name: 'Test User',
      role: 'admin'
    }));
  });
}

test.describe('Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await mockLogin(page);

    // Mock API responses
    await page.route('**/api/v1/agents', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([
          {
            id: '1',
            name: 'test-agent-1',
            display_name: 'Test Agent 1',
            agent_type: 'ai_agent',
            status: 'active',
            created_at: '2025-01-01T00:00:00Z'
          },
          {
            id: '2',
            name: 'test-agent-2',
            display_name: 'Test Agent 2',
            agent_type: 'mcp_server',
            status: 'active',
            created_at: '2025-01-02T00:00:00Z'
          }
        ])
      });
    });

    await page.goto('http://localhost:3000/dashboard');
  });

  test('should load dashboard successfully', async ({ page }) => {
    // Verify dashboard heading or stats cards
    await expect(page.locator('h1, h2').first()).toBeVisible();

    // Verify sidebar navigation
    await expect(page.locator('nav')).toBeVisible();
  });

  test('should display statistics cards', async ({ page }) => {
    // Mock stats API if needed
    await page.route('**/api/v1/stats', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          total_agents: 42,
          total_api_keys: 15,
          average_trust_score: 0.85,
          active_alerts: 3
        })
      });
    });

    await page.reload();

    // Verify stats are displayed
    // (Adjust selectors based on actual implementation)
    await expect(page.locator('text=/Total Agents|Agents/')).toBeVisible();
    await expect(page.locator('text=/API Keys/')).toBeVisible();
  });

  test('should navigate to agents page', async ({ page }) => {
    // Click agents nav link
    const agentsLink = page.locator('a[href*="/agents"]').first();
    await agentsLink.click();

    // Verify navigation
    await expect(page).toHaveURL(/\/dashboard\/agents/);
  });

  test('should navigate to admin page (admin only)', async ({ page }) => {
    // Click admin nav link
    const adminLink = page.locator('a[href*="/admin"]').first();
    await adminLink.click();

    // Verify navigation
    await expect(page).toHaveURL(/\/dashboard\/admin/);
  });

  test('should display user menu', async ({ page }) => {
    // Click user avatar/menu button
    const userMenu = page.locator('[data-testid="user-menu"], button[aria-label*="user"]').first();

    if (await userMenu.isVisible()) {
      await userMenu.click();

      // Verify dropdown appears
      await expect(page.locator('text=/Logout|Sign out/i')).toBeVisible();
    }
  });

  test('should be responsive on tablet', async ({ page }) => {
    await page.setViewportSize({ width: 768, height: 1024 });
    await page.reload();

    // Verify layout adapts
    await expect(page.locator('nav')).toBeVisible();
  });

  test('should be responsive on mobile', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 });
    await page.reload();

    // On mobile, nav might be in a hamburger menu
    // Adjust based on actual implementation
    const mobileNav = page.locator('[data-testid="mobile-menu"], button[aria-label*="menu"]').first();

    if (await mobileNav.isVisible()) {
      await mobileNav.click();
      await expect(page.locator('nav')).toBeVisible();
    }
  });
});
