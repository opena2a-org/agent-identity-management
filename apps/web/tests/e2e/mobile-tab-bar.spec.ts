import { test, expect } from './fixtures/aim-test-stack';
import { PHONE_VIEWPORTS, TABLET_VIEWPORT, intersects, type Box } from './mobile-overlay';

// The bottom tab bar's Secure action is a raised button that extends above the bar. For a
// role that can register agents: every part of each tab is drawn inside that tab's own link
// or button, the page's content, scrolled to its end, ends above that raised button and the
// bar, nothing is painted over the button, and the navigation drawer ends above it.
for (const viewport of [...PHONE_VIEWPORTS, TABLET_VIEWPORT]) {
  test.describe(`the raised Secure action at ${viewport.width}x${viewport.height}`, () => {
    test.use({ viewport, hasTouch: true, isMobile: true });

    const boxOf = async (name: string, locator: { boundingBox(): Promise<Box | null> }) => {
      const box = await locator.boundingBox();
      if (!box) throw new Error(`${name} has no box`);
      return box;
    };

    test('every part of each tab is drawn inside its own link or button', async ({ authedPage: page }) => {
      await page.goto('/dashboard/developers');
      const bar = page.getByRole('navigation', { name: 'Primary' });
      await expect(bar.getByRole('link', { name: 'Secure' }).locator('span').first()).toBeVisible();

      const outside = await bar.evaluate((nav) => {
        // Half a pixel absorbs sub-pixel rounding; the raise this guards against is 14px.
        const slack = 0.5;
        const box = (r: DOMRect) => `[${[r.x, r.y, r.width, r.height].map((n) => Math.round(n)).join(', ')}]`;
        return [...nav.querySelectorAll('a, button')].flatMap((control) => {
          const c = control.getBoundingClientRect();
          return [...control.querySelectorAll('*')]
            .map((part) => ({ part, p: part.getBoundingClientRect() }))
            .filter(({ p }) => p.left < c.left - slack || p.right > c.right + slack || p.top < c.top - slack || p.bottom > c.bottom + slack)
            .map(({ part, p }) => `${control.textContent}: <${part.tagName.toLowerCase()}> ${box(p)} outside ${box(c)}`);
        });
      });
      expect(outside, 'parts of a tab drawn outside its link or button').toEqual([]);
    });

    test('content scrolled to its end ends above the raised button and the bar', async ({ authedPage: page }) => {
      await page.goto('/dashboard/developers');
      const bar = page.getByRole('navigation', { name: 'Primary' });
      const raised = bar.getByRole('link', { name: 'Secure' }).locator('span').first();
      const main = page.locator('main');
      await expect(raised).toBeVisible();
      await expect(main.getByRole('heading').first()).toBeVisible();

      const scrolls = await page.evaluate(() => document.documentElement.scrollHeight > window.innerHeight);
      expect(scrolls, 'the page scrolls, so content can pass beneath the bar').toBe(true);
      await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
      await page.waitForFunction(
        () => Math.ceil(window.scrollY + window.innerHeight) >= document.documentElement.scrollHeight,
      );

      const raisedBox = await boxOf('the raised Secure button', raised);
      const barBox = await boxOf('the bottom tab bar', bar);
      const mainBox = await boxOf('main', main);
      const contentBottom = mainBox.y + mainBox.height;
      expect(raisedBox.y, 'the Secure button rises above the bar').toBeLessThan(barBox.y);
      expect(contentBottom, `content bottom ${contentBottom} ends above the raised button top ${raisedBox.y}`).toBeLessThanOrEqual(raisedBox.y);

      const covered = await raised.evaluate((el) => {
        const r = el.getBoundingClientRect();
        // Center and a point a fifth of the way down: the part that rises above the bar.
        return [0.5, 0.2]
          .map((fy) => document.elementFromPoint(r.left + r.width / 2, r.top + r.height * fy))
          .filter((top) => !top || !(top === el || el.contains(top)))
          .map((top) => (top ? `${top.tagName.toLowerCase()}.${top.getAttribute('class') ?? ''}` : 'nothing'));
      });
      expect(covered, 'elements painted over the raised Secure button').toEqual([]);
    });

    test('the navigation drawer ends above the raised button', async ({ authedPage: page }) => {
      await page.goto('/dashboard/developers');
      const bar = page.getByRole('navigation', { name: 'Primary' });
      const raised = bar.getByRole('link', { name: 'Secure' }).locator('span').first();
      await expect(raised).toBeVisible();
      await bar.getByRole('button', { name: 'Open menu' }).click();

      const drawer = page.locator('aside[aria-hidden="false"]');
      await expect(drawer.getByRole('button', { name: 'Sign out' })).toBeVisible();
      await page.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));

      const drawerBox = await boxOf('the navigation drawer', drawer);
      const raisedBox = await boxOf('the raised Secure button', raised);
      const drawerBottom = drawerBox.y + drawerBox.height;
      expect(drawerBottom, `drawer bottom ${drawerBottom} ends above the raised button top ${raisedBox.y}`).toBeLessThanOrEqual(raisedBox.y);
      expect(intersects(drawerBox, raisedBox), 'the drawer and the raised button overlap').toBe(false);
    });
  });
}
