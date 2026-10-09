import { test, expect } from '@playwright/test';
import { DESKTOP_VIEWPORT, PHONE_VIEWPORTS, expectClearOfBottomBar } from './mobile-overlay';

// The shared overlay check is called on pages whose shell arrives after navigation has
// settled: the dashboard's route gate shows a placeholder until it decides, and only then
// renders the sidebar and the bottom tab bar. These cases serve a page that does the same,
// with no backend, and call the check while the placeholder is still showing.
const SHELL_DELAY_MS = 750;

const BAR_HEIGHT = 84;

/** A page that shows the route gate's placeholder, then renders `shell` after a delay. */
const pageRenderingLate = (shell: string) => `<!doctype html>
<meta name="viewport" content="width=device-width, initial-scale=1">
<body style="margin: 0">
  <p>Verifying authentication...</p>
  <script>
    setTimeout(() => { document.body.innerHTML = ${JSON.stringify(shell)}; }, ${SHELL_DELAY_MS});
  </script>
</body>`;

const bottomBar = (display: 'flex' | 'none') =>
  `<nav aria-label="Primary" style="display: ${display}; position: fixed; left: 0; right: 0; bottom: 0; height: ${BAR_HEIGHT}px; z-index: 30; background: #ccc">Overview</nav>`;

test.describe(`the overlay check at ${DESKTOP_VIEWPORT.width}x${DESKTOP_VIEWPORT.height}`, () => {
  test.use({ viewport: DESKTOP_VIEWPORT });

  test('waits for items that render after the page loads before it counts them', async ({ page }) => {
    await page.setContent(
      pageRenderingLate(`<aside><span>admin@example.test</span><button type="button">Sign out</button></aside>${bottomBar('none')}`),
    );

    const sidebar = page.locator('aside');
    await expectClearOfBottomBar(
      page,
      sidebar.getByText('admin@example.test', { exact: true }).or(sidebar.getByRole('button', { name: 'Sign out' })),
    );
  });
});

const phone = PHONE_VIEWPORTS[0];

test.describe(`the overlay check at ${phone.width}x${phone.height}`, () => {
  test.use({ viewport: phone, hasTouch: true, isMobile: true });

  test('holds an item to the bottom tab bar when both render after the page loads', async ({ page }) => {
    // The item sits inside the bar's area and is painted above it, so the only finding
    // against it is the overlap with the bar, which the check reads once the bar is there.
    await page.setContent(
      pageRenderingLate(
        `${bottomBar('flex')}<button type="button" style="position: fixed; left: 16px; bottom: 16px; z-index: 50">Sign out</button>`,
      ),
    );

    await expect(expectClearOfBottomBar(page, page.getByRole('button', { name: 'Sign out' }))).rejects.toThrow(
      /clear of the bottom tab bar/,
    );
  });
});
