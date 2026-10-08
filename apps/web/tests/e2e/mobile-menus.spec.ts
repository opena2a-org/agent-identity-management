import { test, expect, ADMIN_EMAIL } from './fixtures/aim-test-stack';
import { DESKTOP_VIEWPORT, PHONE_VIEWPORTS, expectBottomBarCovered, expectClearOfBottomBar } from './mobile-overlay';

// The dashboard's menus open above the fixed bottom tab bar on phones: the navigation
// drawer ends above the bar with its account section and Sign out in full, the header
// account menu stays clear of it, and the drawer's backdrop paints over the bar.
for (const viewport of PHONE_VIEWPORTS) {
  test.describe(`menus above the bottom tab bar at ${viewport.width}x${viewport.height}`, () => {
    test.use({ viewport, hasTouch: true, isMobile: true });

    test('the navigation drawer shows its account section and Sign out in full, and its last link opens', async ({ authedPage: page }) => {
      await page.goto('/dashboard/developers');
      await page.getByRole('navigation', { name: 'Primary' }).getByRole('button', { name: 'Open menu' }).click();

      const drawer = page.locator('aside[aria-hidden="false"]');
      const signOut = drawer.getByRole('button', { name: 'Sign out' });
      await expect(signOut).toBeVisible();
      await expectClearOfBottomBar(page, drawer.getByText(ADMIN_EMAIL, { exact: true }).or(signOut));
      await expectBottomBarCovered(page);

      // The last navigation entry sits directly above the account section.
      const organization = drawer.getByRole('link', { name: 'Organization', exact: true });
      await organization.scrollIntoViewIfNeeded();
      await expectClearOfBottomBar(page, organization);
      await organization.click();
      await expect(page).toHaveURL(/\/dashboard\/admin\/users$/);
    });

    test('the header account menu shows every item in full', async ({ authedPage: page }) => {
      await page.goto('/dashboard/developers');
      await page.locator('header button[aria-haspopup="menu"]').click();

      const menu = page.getByRole('menu');
      await expect(menu).toBeVisible();
      await expectClearOfBottomBar(page, menu.getByRole('menuitem').or(menu.getByText(ADMIN_EMAIL, { exact: true })));
    });
  });
}

test.describe(`menus at ${DESKTOP_VIEWPORT.width}x${DESKTOP_VIEWPORT.height}`, () => {
  test.use({ viewport: DESKTOP_VIEWPORT });

  test('the bottom tab bar is hidden and the header account menu and sidebar account section show in full', async ({ authedPage: page }) => {
    await page.goto('/dashboard/developers');
    await expect(page.getByRole('navigation', { name: 'Primary' })).toBeHidden();
    const barHeight = await page.evaluate(() =>
      getComputedStyle(document.documentElement).getPropertyValue('--mobile-tab-bar-height').trim(),
    );
    expect(barHeight, '--mobile-tab-bar-height at lg and up').toBe('0px');

    const sidebar = page.locator('aside:not([aria-hidden])');
    await expectClearOfBottomBar(
      page,
      sidebar.getByText(ADMIN_EMAIL, { exact: true }).or(sidebar.getByRole('button', { name: 'Sign out' })),
    );

    await page.locator('header button[aria-haspopup="menu"]').click();
    const menu = page.getByRole('menu');
    await expect(menu).toBeVisible();
    await expectClearOfBottomBar(page, menu.getByRole('menuitem').or(menu.getByText(ADMIN_EMAIL, { exact: true })));
  });
});
