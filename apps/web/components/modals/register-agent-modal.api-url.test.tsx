import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";

// After an agent is registered with a capability, the register-agent dialog's manual
// integration step shows a cURL command that verifies the capability, and its copy
// button copies it. The command names the API the client signs in to
// (lib/api-base-url.ts), which here is the URL the page hands the browser in
// window.__RUNTIME_CONFIG__: a second stack's API on port 8081, and the API's public
// origin behind a reverse proxy. It is not the dashboard's own origin.

const AGENT_ID = "agent-1";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
  vi.restoreAllMocks();
  Reflect.deleteProperty(navigator, "clipboard");
});

// The verification command the dialog shows, and the text its copy button copies, with
// the dashboard open at pageUrl and the page handing the browser apiUrl. The modules are
// loaded afresh: the client keeps the address it resolves first.
async function verificationCommand(pageUrl: string, apiUrl: string): Promise<{ shown: string; copied: string }> {
  vi.resetModules();
  vi.stubGlobal("location", new URL(pageUrl));
  vi.stubGlobal("__RUNTIME_CONFIG__", { apiUrl });
  vi.spyOn(console, "log").mockImplementation(() => {});
  // The manual step reads the agent's keys itself.
  vi.stubGlobal(
    "fetch",
    vi.fn(
      async (_url: string | URL | Request) =>
        new Response(JSON.stringify({ publicKey: "FAKE-PUBLIC-KEY", privateKey: "FAKE-PRIVATE-KEY" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
    ),
  );
  const writeText = vi.fn(async (_text: string) => {});
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });

  // The real client, so its address comes from getApiUrl in lib/api.ts. api is a Proxy
  // whose set trap writes onto the client instance, which these modules created afresh.
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
        createdAt: "2026-10-17T00:00:00Z",
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

  const block = (await screen.findByText(/curl -X POST/)).closest("div") as HTMLElement;
  fireEvent.click(within(block).getByRole("button", { name: /Copy/ }));
  await waitFor(() => expect(writeText).toHaveBeenCalledTimes(1));
  return { shown: block.textContent ?? "", copied: writeText.mock.calls[0][0] };
}

describe("the register-agent dialog's verification command", () => {
  it.each([
    ["a second stack on one host", "http://localhost:3001/dashboard/agents", "http://localhost:8081"],
    ["a dashboard behind a reverse proxy", "https://aim.example.com/dashboard/agents", "https://api.example.com"],
  ])("names the API the client resolved, on %s", async (_case, pageUrl, apiUrl) => {
    const { shown, copied } = await verificationCommand(pageUrl, apiUrl);
    const command = `curl -X POST "${apiUrl}/api/v1/agents/${AGENT_ID}/verify-capability"`;

    expect(shown).toContain(command);
    expect(copied).toContain(command);
    for (const text of [shown, copied]) {
      expect(text).toContain(`"capability": "db:read"`);
      expect(text).not.toContain(`${new URL(pageUrl).origin}/api/`);
      expect(text).not.toContain("localhost:8080");
    }
  });
});
