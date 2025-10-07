import { test, expect } from '@playwright/test';

// Mock authentication helper
async function mockLogin(page: any) {
  // In a real test, you'd use a test user or mock OAuth callback
  // For now, we'll set a mock JWT token in localStorage
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

test.describe('Agent Registration', () => {
  test.beforeEach(async ({ page }) => {
    await mockLogin(page);
    await page.goto('http://localhost:3000/dashboard/agents/new');
  });

  test('should load agent registration form', async ({ page }) => {
    // Verify form elements are present
    await expect(page.locator('h1', { hasText: 'Register New Agent' })).toBeVisible();

    // Verify agent type buttons
    await expect(page.locator('button', { hasText: 'AI Agent' })).toBeVisible();
    await expect(page.locator('button', { hasText: 'MCP Server' })).toBeVisible();

    // Verify form fields
    await expect(page.locator('input[name="name"]')).toBeVisible();
    await expect(page.locator('input[name="display_name"]')).toBeVisible();
    await expect(page.locator('textarea[name="description"]')).toBeVisible();
  });

  test('should select agent type', async ({ page }) => {
    // Click AI Agent type
    const aiAgentButton = page.locator('button', { hasText: 'AI Agent' }).first();
    await aiAgentButton.click();

    // Verify selection (button should have blue border/background)
    await expect(aiAgentButton).toHaveClass(/border-blue-600/);
  });

  test('should fill agent registration form', async ({ page }) => {
    // Select AI Agent type
    await page.locator('button', { hasText: 'AI Agent' }).first().click();

    // Fill form fields
    await page.fill('input[name="name"]', 'test-agent');
    await page.fill('input[name="display_name"]', 'Test Agent');
    await page.fill('textarea[name="description"]', 'A comprehensive test agent for E2E testing');
    await page.fill('input[name="version"]', '1.0.0');
    await page.fill('input[name="repository_url"]', 'https://github.com/test/agent');
    await page.fill('input[name="documentation_url"]', 'https://docs.test.com');

    // Verify values are filled
    await expect(page.locator('input[name="name"]')).toHaveValue('test-agent');
    await expect(page.locator('input[name="display_name"]')).toHaveValue('Test Agent');
  });

  test('should validate required fields', async ({ page }) => {
    // Try to submit empty form
    const submitButton = page.locator('button[type="submit"]');
    await submitButton.click();

    // Verify HTML5 validation or custom validation messages
    const nameInput = page.locator('input[name="name"]');
    const isInvalid = await nameInput.evaluate((el: HTMLInputElement) => !el.validity.valid);
    expect(isInvalid).toBe(true);
  });

  test('should submit agent registration', async ({ page }) => {
    // Intercept API request
    await page.route('**/api/v1/agents', async (route) => {
      if (route.request().method() === 'POST') {
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({
            id: '00000000-0000-0000-0000-000000000002',
            name: 'test-agent',
            display_name: 'Test Agent',
            agent_type: 'ai_agent',
            status: 'active'
          })
        });
      }
    });

    // Fill form
    await page.locator('button', { hasText: 'AI Agent' }).first().click();
    await page.fill('input[name="name"]', 'test-agent');
    await page.fill('input[name="display_name"]', 'Test Agent');
    await page.fill('textarea[name="description"]', 'A test agent');

    // Submit form
    await page.locator('button[type="submit"]').click();

    // Verify redirect to agent list
    await expect(page).toHaveURL(/\/dashboard\/agents/);
  });

  test('should handle API errors gracefully', async ({ page }) => {
    // Intercept API request with error
    await page.route('**/api/v1/agents', async (route) => {
      await route.fulfill({
        status: 400,
        contentType: 'application/json',
        body: JSON.stringify({
          error: 'Agent name already exists'
        })
      });
    });

    // Fill and submit form
    await page.locator('button', { hasText: 'AI Agent' }).first().click();
    await page.fill('input[name="name"]', 'duplicate-agent');
    await page.fill('input[name="display_name"]', 'Duplicate Agent');
    await page.fill('textarea[name="description"]', 'A duplicate agent');
    await page.locator('button[type="submit"]').click();

    // Verify error message is displayed
    // (Adjust selector based on actual error display implementation)
    await expect(page.locator('text=already exists')).toBeVisible();
  });
});
