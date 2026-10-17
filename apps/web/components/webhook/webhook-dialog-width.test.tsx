import { existsSync, readFileSync } from "fs";
import path from "path";
import { chromium, type Browser } from "@playwright/test";
import postcss, { type AcceptedPlugin } from "postcss";
import tailwindcss from "tailwindcss";
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import tailwindConfig from "../../tailwind.config.js";

const api = vi.hoisted(() => ({ getWebhook: vi.fn(), createWebhook: vi.fn(), rotateWebhookSecret: vi.fn() }));
vi.mock("@/lib/api", () => ({ api }));

import { WebhookCreateModal } from "./webhook-create-modal";
import { WebhookDetailModal } from "./webhook-detail-modal";

// The webhook dialogs as the app renders them, measured in Chromium at a 375x812
// phone viewport with the app's own stylesheet. The dialog is a one-column grid;
// an auto column grows to its widest unbreakable descendant (the payload example's
// longest line), and every row, header and footer included, then grows with it.

const webhook = {
  id: "0c6f3c1e-8f0e-4a63-9a43-2f1d8b7e5a10",
  organizationId: "7d9f2b4a-1c3e-4f5a-8b6c-9d0e1f2a3b4c",
  name: "receiver",
  url: "https://hooks.example.com/aim",
  events: ["agent.created", "agent.verified", "trust_score.changed"],
  isActive: true,
  createdAt: "2026-10-16T00:00:00Z",
  successCount: 3,
  failureCount: 1,
};

// The app's stylesheet, generated for exactly the markup under test.
async function stylesheetFor(html: string): Promise<string> {
  const globals = readFileSync(path.join(__dirname, "../../app/globals.css"), "utf8");
  // Tailwind is installed at the workspace root with its own postcss copy, whose
  // plugin type this package's postcss does not recognise; the plugin is the same.
  const plugin = tailwindcss({ ...tailwindConfig, content: [{ raw: html, extension: "html" }] });
  const result = await postcss([plugin as unknown as AcceptedPlugin]).process(globals, { from: undefined });
  return result.css;
}

const browserInstalled = existsSync(chromium.executablePath());

describe.skipIf(!browserInstalled)("webhook dialogs at 375x812", () => {
  let browser: Browser;
  beforeAll(async () => {
    browser = await chromium.launch();
  }, 60_000);
  afterAll(async () => {
    await browser?.close();
  });
  afterEach(cleanup);

  async function measure(dialog: HTMLElement) {
    const html = dialog.outerHTML;
    const css = await stylesheetFor(html);
    const page = await browser.newPage({ viewport: { width: 375, height: 812 } });
    await page.setContent(
      `<!doctype html><html><head><meta name="viewport" content="width=device-width, initial-scale=1"><style>${css}</style></head><body class="font-sans">${html}</body></html>`,
    );
    const result = await page.evaluate(() => {
      const el = document.querySelector('[role="dialog"]') as HTMLElement;
      const right = (selector: string) =>
        Math.max(...Array.from(el.querySelectorAll(selector), (n) => n.getBoundingClientRect().right));
      // The descendant reaching furthest right, named in a failure.
      let widest = "";
      let widestRight = 0;
      for (const n of Array.from(el.querySelectorAll("*"))) {
        const r = n.getBoundingClientRect().right;
        if (r > widestRight) {
          widestRight = r;
          widest = `${n.tagName.toLowerCase()}.${n.getAttribute("class") ?? ""} ${(n.textContent ?? "").slice(0, 40)}`;
        }
      }
      return {
        widest: `${widest} (right edge ${Math.round(widestRight)}px)`,
        scrollWidth: el.scrollWidth,
        clientWidth: el.clientWidth,
        buttonsRight: right("button"),
        headingRight: right("h2"),
      };
    });
    await page.close();
    return result;
  }

  it("fits the create dialog, its title and its buttons inside the screen", async () => {
    render(<WebhookCreateModal isOpen onClose={vi.fn()} onSuccess={vi.fn()} />);
    const measured = await measure(screen.getByRole("dialog"));

    expect(measured.scrollWidth, measured.widest).toBeLessThanOrEqual(measured.clientWidth);
    expect(measured.buttonsRight).toBeLessThanOrEqual(375);
    expect(measured.headingRight).toBeLessThanOrEqual(375);
  }, 60_000);

  it("fits the detail dialog, its title and its buttons inside the screen", async () => {
    api.getWebhook.mockReset().mockResolvedValue(webhook);
    render(<WebhookDetailModal isOpen webhookId={webhook.id} onClose={vi.fn()} onRefresh={vi.fn()} />);
    await screen.findByText(webhook.url);
    const measured = await measure(screen.getByRole("dialog"));

    expect(measured.scrollWidth, measured.widest).toBeLessThanOrEqual(measured.clientWidth);
    expect(measured.buttonsRight).toBeLessThanOrEqual(375);
    expect(measured.headingRight).toBeLessThanOrEqual(375);
  }, 60_000);
});
