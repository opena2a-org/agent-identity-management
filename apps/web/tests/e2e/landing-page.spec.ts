import { test, expect } from '@playwright/test';

test.describe('Landing Page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('http://localhost:3000');
  });

  test('should load landing page successfully', async ({ page }) => {
    // Verify page title
    await expect(page).toHaveTitle(/Agent Identity Management/);

    // Verify main heading
    const heading = page.locator('h1').first();
    await expect(heading).toBeVisible();

    // Verify SSO buttons are present
    const googleButton = page.locator('button', { hasText: 'Google' });
    await expect(googleButton).toBeVisible();
  });

  test('should have no console errors', async ({ page }) => {
    const consoleErrors: string[] = [];

    page.on('console', (message) => {
      if (message.type() === 'error') {
        consoleErrors.push(message.text());
      }
    });

    await page.goto('http://localhost:3000');
    await page.waitForLoadState('networkidle');

    expect(consoleErrors).toHaveLength(0);
  });

  test('should be responsive on mobile', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 });
    await page.goto('http://localhost:3000');

    // Verify mobile layout
    const heading = page.locator('h1').first();
    await expect(heading).toBeVisible();
  });

  test('should initiate Google OAuth flow', async ({ page }) => {
    const googleButton = page.locator('button', { hasText: 'Google' });

    // Click Google SSO button
    const [popup] = await Promise.all([
      page.waitForEvent('popup'),
      googleButton.click()
    ]);

    // Verify redirect to OAuth endpoint
    expect(popup.url()).toContain('/api/v1/auth/login/google');
  });
});
