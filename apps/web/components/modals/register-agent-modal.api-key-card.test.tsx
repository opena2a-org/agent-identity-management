import { existsSync, readFileSync } from "fs";
import path from "path";
import { chromium, type Browser } from "@playwright/test";
import postcss, { type AcceptedPlugin } from "postcss";
import tailwindcss from "tailwindcss";
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import tailwindConfig from "../../tailwind.config.js";

// After an agent is registered, the register-agent dialog shows a card titled "Your API key
// (for manual integration)" with the key field, a button that reveals the key and a button
// that copies it in one row. The row's buttons stay inside the card at a phone width. The
// first test reads from the markup that the key field may shrink below its intrinsic width:
// an input in a flex row keeps that width unless its minimum width is zero, and then pushes
// the buttons that follow it past the card. The second lays the dialog out in Chromium with
// the app's stylesheet, at 1280x800 and at 375x812, and measures how far each button is
// drawn past the card's content edge, before and after the copy button is pressed.

const AGENT_ID = "11111111-2222-4333-8444-555555555555";
const API_KEY = "FAKE-KEY-1234";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  Reflect.deleteProperty(navigator, "clipboard");
});

type ApiKeyCard = {
  card: HTMLElement;
  field: HTMLInputElement;
  reveal: HTMLElement;
  copy: HTMLElement;
};

// Registers an agent with the capability db:read and returns the API key card, its key
// field, the button that reveals the key and the button that copies it.
async function openApiKeyCard(): Promise<ApiKeyCard> {
  vi.resetModules();
  vi.stubGlobal("__RUNTIME_CONFIG__", { apiUrl: "http://localhost:8080" });
  vi.spyOn(console, "log").mockImplementation(() => {});
  Object.defineProperty(navigator, "clipboard", { value: { writeText: vi.fn(async () => {}) }, configurable: true });

  const { api } = await import("@/lib/api");
  Object.assign(api, {
    getCapabilityDefinitions: vi.fn(async () => ({ capabilities: [], reservedNamespaces: [] })),
    listMCPServers: vi.fn(async () => ({ mcpServers: [], total: 0 })),
    getDiscoveredMCPs: vi.fn(async () => ({ discovered: [], totalUnmapped: 0, totalAgents: 0, registeredServers: 0 })),
    createAgent: vi.fn(async () => ({
      id: AGENT_ID,
      name: "billing",
      displayName: "billing",
      apiKey: {
        key: API_KEY,
        id: "key-1",
        name: "default",
        prefix: "FAKE",
        expiresAt: null,
        createdAt: "2026-10-18T00:00:00Z",
      },
    })),
  });
  const { RegisterAgentModal } = await import("./register-agent-modal");

  render(<RegisterAgentModal isOpen onClose={() => {}} />);
  fireEvent.change(screen.getByPlaceholderText("e.g., claude-assistant"), { target: { value: "billing" } });
  // The custom capability field appears once the capability list has loaded.
  const capability = await screen.findByPlaceholderText("e.g., payment:process or email:send");
  fireEvent.change(capability, { target: { value: "db:read" } });
  fireEvent.click(within(capability.parentElement as HTMLElement).getByRole("button", { name: /^Add$/ }));
  fireEvent.click(screen.getByRole("button", { name: /Register agent/ }));

  const title = await screen.findByText("Your API key (for manual integration)");
  const field = screen.getByDisplayValue(API_KEY) as HTMLInputElement;
  // The card is the nearest box that holds both the title and the key field.
  let card = title.parentElement;
  while (card && !card.contains(field)) card = card.parentElement;
  if (!card) throw new Error("no box holds both the card's title and its key field");
  const row = field.parentElement as HTMLElement;
  const copy = within(row).getByRole("button", { name: /Copy key/ });
  const reveal = within(row)
    .getAllByRole("button")
    .find((button) => button !== copy);
  if (!reveal) throw new Error("the key row has no button that reveals the key");
  return { card, field, reveal, copy };
}

describe("the register-agent dialog's API key card", () => {
  it("lets the key field shrink below its intrinsic width, so the row's buttons fit beside it", async () => {
    const { card, field, reveal, copy } = await openApiKeyCard();
    const row = field.parentElement as HTMLElement;

    expect(card.contains(reveal) && card.contains(copy), "both buttons are in the card").toBe(true);
    expect(row.className, "the field and the buttons share a flex row").toMatch(/(^|\s)flex(\s|$)/);
    expect(field.className, "the field grows into the row's free space").toMatch(/(^|\s)flex-1(\s|$)/);
    expect(field.className, "the field may shrink below its intrinsic width").toMatch(/(^|\s)min-w-0(\s|$)/);
  });
});

// The app's stylesheet, generated for exactly the markup under test.
async function stylesheetFor(html: string): Promise<string> {
  const globals = readFileSync(path.join(__dirname, "../../app/globals.css"), "utf8");
  // Tailwind is installed at the workspace root with its own postcss copy, whose
  // plugin type this package's postcss does not recognise; the plugin is the same.
  const plugin = tailwindcss({ ...tailwindConfig, content: [{ raw: html, extension: "html" }] });
  const result = await postcss([plugin as unknown as AcceptedPlugin]).process(globals, { from: undefined });
  return result.css;
}

type CardMeasurement = {
  // CSS px by which each button's right edge passes the card's content edge; 0 when it fits.
  copyPast: number;
  revealPast: number;
  // CSS px by which the card's right edge passes the dialog panel's, and the panel's the viewport's.
  cardPastPanel: number;
  panelPastViewport: number;
  fieldWidth: number;
  copyWidth: number;
};

// Runs in the page. Where the card, its key field and its buttons are drawn.
function measureApiKeyCard(): CardMeasurement {
  const box = (name: string) => (document.querySelector(`[data-measure=${name}]`) as HTMLElement).getBoundingClientRect();
  const card = document.querySelector("[data-measure=card]") as HTMLElement;
  const cardStyle = getComputedStyle(card);
  const contentRight = card.getBoundingClientRect().right - parseFloat(cardStyle.paddingRight) - parseFloat(cardStyle.borderRightWidth);
  const past = (right: number, edge: number) => Math.max(0, Math.round((right - edge) * 100) / 100);
  return {
    copyPast: past(box("copy").right, contentRight),
    revealPast: past(box("reveal").right, contentRight),
    cardPastPanel: past(box("card").right, box("panel").right),
    panelPastViewport: past(box("panel").right, document.documentElement.clientWidth),
    fieldWidth: box("field").width,
    copyWidth: box("copy").width,
  };
}

// Runs where a Chromium build for this Playwright is installed (`npx playwright install
// chromium`), as tests/ux-sweep.test.ts does.
const browserInstalled = existsSync(chromium.executablePath());

describe.skipIf(!browserInstalled)("the register-agent dialog's API key card in Chromium", () => {
  let browser: Browser;
  beforeAll(async () => {
    browser = await chromium.launch();
  }, 60_000);
  afterAll(async () => {
    await browser?.close();
  });

  it("keeps the reveal and copy buttons inside the card, at 1280x800 and at 375x812", async () => {
    const { card, field, reveal, copy } = await openApiKeyCard();
    const panel = card.closest(".overlay-surface") as HTMLElement | null;
    if (!panel) throw new Error("the card is not in the dialog panel");
    panel.setAttribute("data-measure", "panel");
    card.setAttribute("data-measure", "card");
    field.setAttribute("data-measure", "field");
    reveal.setAttribute("data-measure", "reveal");
    copy.setAttribute("data-measure", "copy");
    // The markup React rendered, before the copy button is pressed and after, when it reads Copied.
    const states: Record<string, string> = { "Copy key": document.body.innerHTML };
    fireEvent.click(copy);
    await waitFor(() => expect(copy.textContent).toContain("Copied"));
    states.Copied = document.body.innerHTML;
    const css = await stylesheetFor(Object.values(states).join("\n"));

    const past: Record<string, { copy: number; reveal: number }> = {};
    for (const [width, height] of [
      [1280, 800],
      [375, 812],
    ]) {
      for (const [label, markup] of Object.entries(states)) {
        const page = await browser.newPage({ viewport: { width, height } });
        await page.setContent(
          `<!doctype html><html><head><meta name="viewport" content="width=device-width, initial-scale=1"><style>${css}</style></head><body class="font-sans">${markup}</body></html>`,
        );
        const measured = await page.evaluate(measureApiKeyCard);
        await page.close();
        // The measurement saw the field and the button, and the dialog fits the viewport.
        expect(measured.fieldWidth).toBeGreaterThan(0);
        expect(measured.copyWidth).toBeGreaterThan(0);
        expect(measured.panelPastViewport, `${width}x${height}: the panel fits the viewport`).toBe(0);
        expect(measured.cardPastPanel, `${width}x${height}: the card fits the panel`).toBe(0);
        past[`${width}x${height}, button reads ${label}`] = { copy: measured.copyPast, reveal: measured.revealPast };
      }
    }

    expect(past).toEqual({
      "1280x800, button reads Copy key": { copy: 0, reveal: 0 },
      "1280x800, button reads Copied": { copy: 0, reveal: 0 },
      "375x812, button reads Copy key": { copy: 0, reveal: 0 },
      "375x812, button reads Copied": { copy: 0, reveal: 0 },
    });
  }, 60_000);
});
