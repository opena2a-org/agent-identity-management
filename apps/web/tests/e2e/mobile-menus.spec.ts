import { test, expect, ADMIN_EMAIL } from './fixtures/aim-test-stack';
import { DESKTOP_VIEWPORT, PHONE_VIEWPORTS, SHORT_PHONE_VIEWPORT, expectBottomBarCovered, expectClearOfBottomBar } from './mobile-overlay';

// The dashboard's menus open above the fixed bottom tab bar on phones: the navigation
// drawer ends above the bar with its account section and Sign out in full, the header
// account menu stays clear of it (on a viewport too short for it, by capping its height
// and scrolling), and the drawer's backdrop paints over the bar.
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

test.describe(`header account menu at ${SHORT_PHONE_VIEWPORT.width}x${SHORT_PHONE_VIEWPORT.height}`, () => {
  test.use({ viewport: SHORT_PHONE_VIEWPORT, hasTouch: true, isMobile: true });

  test('the header account menu caps its height above the bottom tab bar and scrolls to Sign out', async ({ authedPage: page }) => {
    await page.goto('/dashboard/developers');
    await page.locator('header button[aria-haspopup="menu"]').click();

    const menu = page.getByRole('menu');
    await expect(menu).toBeVisible();
    const barBox = await page.getByRole('navigation', { name: 'Primary' }).boundingBox();
    if (!barBox) throw new Error('bottom tab bar has no box');
    const size = await menu.evaluate((el) => ({
      top: el.getBoundingClientRect().top,
      scrollHeight: el.scrollHeight,
      clientHeight: el.clientHeight,
    }));
    // At its natural height the menu would reach under the bar, so this viewport exercises the cap.
    expect(size.top + size.scrollHeight, `menu bottom at its natural height (bar top ${barBox.y})`).toBeGreaterThan(barBox.y);

    await expectClearOfBottomBar(page, menu);
    expect(size.scrollHeight, 'menu content height against its capped height').toBeGreaterThan(size.clientHeight);

    // The last item scrolls into view inside the menu, clear of the bar.
    const signOut = menu.getByRole('menuitem', { name: 'Sign out' });
    await signOut.scrollIntoViewIfNeeded();
    expect(await menu.evaluate((el) => el.scrollTop), 'menu scroll offset after reaching Sign out').toBeGreaterThan(0);
    await expectClearOfBottomBar(page, signOut);
  });
});

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
