import { expect, type Locator, type Page } from '@playwright/test';

/** Phone viewports the bottom tab bar is checked at. */
export const PHONE_VIEWPORTS = [
  { width: 375, height: 812 },
  { width: 375, height: 667 },
] as const;

/** Desktop viewport: the bottom tab bar is hidden and its height token is 0. */
export const DESKTOP_VIEWPORT = { width: 1280, height: 800 } as const;

type Box = { x: number; y: number; width: number; height: number };

const intersects = (a: Box, b: Box) =>
  a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height;

/**
 * The shared overlay check. For every element matched by `items`: it is visible, its box
 * lies inside the viewport, its box does not intersect the fixed bottom tab bar when the
 * bar is shown (below lg), and the topmost element at its center and near each edge is the
 * item itself (nothing, the bar included, is painted over it). Any popover, menu, drawer
 * or dialog can pass its items.
 */
export async function expectClearOfBottomBar(page: Page, items: Locator) {
  const bar = page.getByRole('navigation', { name: 'Primary' });
  const viewport = page.viewportSize();
  if (!viewport) throw new Error('page has no viewport');
  const barShown = await bar.isVisible();

  // The suite runs against `next dev`, whose dev-tools indicator sits in the bottom-left
  // corner over the sidebar foot. It does not exist in a production build.
  await page.addStyleTag({ content: 'nextjs-portal { display: none !important; }' });

  const count = await items.count();
  expect(count, 'overlay items to check').toBeGreaterThan(0);
  // Measure where an opening transition (the drawer slides in) comes to rest.
  await page.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));

  for (let i = 0; i < count; i++) {
    const item = items.nth(i);
    await expect(item).toBeVisible();
    const box = await item.boundingBox();
    if (!box) throw new Error(`item ${i} has no box`);
    const label = `${(await item.innerText()).trim().split('\n')[0] || `item ${i}`} at ${viewport.width}x${viewport.height}`;

    expect(box.x, `${label}: left edge in viewport`).toBeGreaterThanOrEqual(0);
    expect(box.y, `${label}: top edge in viewport`).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width, `${label}: right edge in viewport`).toBeLessThanOrEqual(viewport.width);
    expect(box.y + box.height, `${label}: bottom edge in viewport`).toBeLessThanOrEqual(viewport.height);
    if (barShown) {
      const barBox = await bar.boundingBox();
      if (!barBox) throw new Error('bottom tab bar has no box');
      expect(intersects(box, barBox), `${label}: clear of the bottom tab bar (bar top ${barBox.y}, item bottom ${box.y + box.height})`).toBe(false);
    }

    const covered = await item.evaluate((el) => {
      const r = el.getBoundingClientRect();
      // Center plus four points a fifth of the way in from each edge: inside rounded
      // corners (hit testing follows border-radius), yet near enough to each edge that a
      // half-covered item is caught.
      const at = (fx: number, fy: number) => [r.left + r.width * fx, r.top + r.height * fy];
      const points = [at(0.5, 0.5), at(0.2, 0.5), at(0.8, 0.5), at(0.5, 0.2), at(0.5, 0.8)];
      return points
        .map(([x, y]) => ({ x, y, top: document.elementFromPoint(x, y) }))
        .filter(({ top }) => !top || !(top === el || el.contains(top)))
        .map(({ x, y, top }) => {
          const name = top ? `${top.tagName.toLowerCase()}${top.getAttribute('aria-label') ? `[${top.getAttribute('aria-label')}]` : ''}.${(top.getAttribute('class') || '').split(' ').slice(0, 4).join('.')}` : 'nothing';
          return `${Math.round(x)},${Math.round(y)} under ${name}`;
        });
    });
    expect(covered, `${label}: points painted over by another element`).toEqual([]);
  }
}

/**
 * For modal overlays (a drawer or dialog with a backdrop) rendered inside the dashboard
 * shell: the backdrop paints over the bottom tab bar, so the overlay stacks above the bar
 * rather than being held beneath it by an ancestor's stacking context.
 */
export async function expectBottomBarCovered(page: Page) {
  const bar = page.getByRole('navigation', { name: 'Primary' });
  await page.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'));
  const onTop = await bar.evaluate((el) => {
    const r = el.getBoundingClientRect();
    const top = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
    return top && el.contains(top) ? `the bar (${top.tagName.toLowerCase()})` : 'an overlay';
  });
  expect(onTop, 'topmost element at the bottom tab bar while a modal overlay is open').toBe('an overlay');
}
