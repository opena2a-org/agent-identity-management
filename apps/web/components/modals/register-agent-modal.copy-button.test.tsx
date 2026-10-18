import { existsSync, readFileSync } from "fs";
import path from "path";
import { chromium, type Browser } from "@playwright/test";
import postcss, { type AcceptedPlugin } from "postcss";
import tailwindcss from "tailwindcss";
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import tailwindConfig from "../../tailwind.config.js";

// After an agent is registered with a capability, the register-agent dialog's manual
// integration step shows a cURL command that verifies the capability, and a button that
// copies it. The button has a row of its own above the command. The first test reads
// that from the markup. The second lays the dialog out in Chromium with the app's
// stylesheet, at 1280x800 and at 375x812, and lists the command's characters drawn where
// the button is, before and after the button is pressed.

const AGENT_ID = "11111111-2222-4333-8444-555555555555";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  Reflect.deleteProperty(navigator, "clipboard");
});

// Registers an agent with the capability db:read and opens the manual integration step.
// Returns the command's pre and the button that copies it.
async function openVerificationStep(): Promise<{ command: HTMLElement; copy: HTMLElement }> {
  vi.resetModules();
  vi.stubGlobal("__RUNTIME_CONFIG__", { apiUrl: "http://localhost:8080" });
  vi.spyOn(console, "log").mockImplementation(() => {});
  // The manual step reads the agent's keys itself.
  vi.stubGlobal(
    "fetch",
    vi.fn(
      async () =>
        new Response(JSON.stringify({ publicKey: "FAKE-PUBLIC-KEY", privateKey: "FAKE-PRIVATE-KEY" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
    ),
  );
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
        key: "FAKE-KEY-1234",
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
  fireEvent.click(await screen.findByRole("button", { name: /Manual integration/ }));

  const command = await screen.findByText(/curl -X POST/);
  const copy = within(command.closest("div") as HTMLElement).getByRole("button", { name: /Copy/ });
  return { command, copy };
}

describe("the register-agent dialog's verification command", () => {
  it("has its Copy button in a row above the command, in the flow of the block", async () => {
    const { command, copy } = await openVerificationStep();
    const block = command.parentElement as HTMLElement;

    expect(block.contains(copy), "the button is in the command's block").toBe(true);
    expect(copy.compareDocumentPosition(command) & Node.DOCUMENT_POSITION_FOLLOWING, "the command follows the button").toBeTruthy();
    // A box positioned absolute or fixed leaves the flow, so the command's lines can run
    // under it.
    for (let box: HTMLElement | null = copy; box && box !== block; box = box.parentElement) {
      expect(box.className, "the button and the row it sits in stay in the flow").not.toMatch(/(^|\s)(absolute|fixed)(\s|$)/);
    }
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

// Runs in the page. The characters of the command drawn where the Copy button is, read
// one at a time from the laid-out text, and how many characters were laid out at all.
function textUnderCopyButton(): { covered: string; laidOut: number; buttonWidth: number } {
  const command = document.querySelector("[data-measure=command]") as HTMLElement;
  const button = (document.querySelector("[data-measure=copy]") as HTMLElement).getBoundingClientRect();
  const walker = document.createTreeWalker(command, NodeFilter.SHOW_TEXT);
  const range = document.createRange();
  let covered = "";
  let laidOut = 0;
  for (let node = walker.nextNode() as Text | null; node; node = walker.nextNode() as Text | null) {
    for (let i = 0; i < node.length; i++) {
      range.setStart(node, i);
      range.setEnd(node, i + 1);
      const boxes = Array.from(range.getClientRects()).filter((r) => r.width > 0);
      if (boxes.length > 0) laidOut++;
      if (boxes.some((r) => r.left < button.right && r.right > button.left && r.top < button.bottom && r.bottom > button.top)) {
        covered += node.data[i];
      }
    }
  }
  return { covered, laidOut, buttonWidth: button.width };
}

// Runs where a Chromium build for this Playwright is installed (`npx playwright install
// chromium`), as tests/ux-sweep.test.ts does.
const browserInstalled = existsSync(chromium.executablePath());

describe.skipIf(!browserInstalled)("the register-agent dialog's verification command in Chromium", () => {
  let browser: Browser;
  beforeAll(async () => {
    browser = await chromium.launch();
  }, 60_000);
  afterAll(async () => {
    await browser?.close();
  });

  it("draws no character of the command under the Copy button, at 1280x800 and at 375x812", async () => {
    const { command, copy } = await openVerificationStep();
    command.setAttribute("data-measure", "command");
    copy.setAttribute("data-measure", "copy");
    // The markup React rendered, before the button is pressed and after, when it reads Copied.
    const states: Record<string, string> = { Copy: document.body.innerHTML };
    fireEvent.click(copy);
    await waitFor(() => expect(copy.textContent).toContain("Copied"));
    states.Copied = document.body.innerHTML;
    const css = await stylesheetFor(Object.values(states).join("\n"));

    const covered: Record<string, string> = {};
    for (const [width, height] of [
      [1280, 800],
      [375, 812],
    ]) {
      for (const [label, markup] of Object.entries(states)) {
        const page = await browser.newPage({ viewport: { width, height } });
        await page.setContent(
          `<!doctype html><html><head><meta name="viewport" content="width=device-width, initial-scale=1"><style>${css}</style></head><body class="font-sans">${markup}</body></html>`,
        );
        const measured = await page.evaluate(textUnderCopyButton);
        await page.close();
        // The measurement saw the command and the button.
        expect(measured.laidOut).toBeGreaterThan(100);
        expect(measured.buttonWidth).toBeGreaterThan(0);
        covered[`${width}x${height}, button reads ${label}`] = measured.covered;
      }
    }

    expect(covered).toEqual({
      "1280x800, button reads Copy": "",
      "1280x800, button reads Copied": "",
      "375x812, button reads Copy": "",
      "375x812, button reads Copied": "",
    });
  }, 60_000);
});
