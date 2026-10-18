import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render } from "@testing-library/react";

// The setup guide on the MCP server page writes cURL commands that name the API's
// address. It names the API the client signs in to (lib/api-base-url.ts), which here is
// the URL the page hands the browser in window.__RUNTIME_CONFIG__: a second stack's API
// on port 8081, and the API's public origin behind a reverse proxy. It is not port 8080
// on the dashboard's host, nor the dashboard's own origin.

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
  vi.restoreAllMocks();
});

// The guide's text with the dashboard open at pageUrl and the page handing the browser
// apiUrl. The modules are loaded afresh: the client keeps the address it resolves first.
async function guideText(pageUrl: string, apiUrl: string): Promise<string> {
  vi.resetModules();
  vi.stubGlobal("location", new URL(pageUrl));
  vi.stubGlobal("__RUNTIME_CONFIG__", { apiUrl });
  vi.spyOn(console, "log").mockImplementation(() => {});
  const { MCPSetupGuide } = await import("./mcp-setup-guide");
  const { container } = render(
    <MCPSetupGuide mcpServerId="mcp-1" mcpServerName="Files" mcpServerUrl="https://mcp.example.test" />,
  );
  return container.textContent ?? "";
}

describe("the MCP server setup guide", () => {
  it.each([
    ["a second stack on one host", "http://localhost:3001/dashboard/mcp/mcp-1", "http://localhost:8081"],
    ["a dashboard behind a reverse proxy", "https://aim.example.com/dashboard/mcp/mcp-1", "https://api.example.com"],
  ])("names the API the client resolved, on %s", async (_case, pageUrl, apiUrl) => {
    const text = await guideText(pageUrl, apiUrl);

    expect(text).toContain(`curl -X GET "${apiUrl}/api/v1/mcp-servers/mcp-1"`);
    expect(text).toContain(`curl -X GET "${apiUrl}/api/v1/mcp-servers/mcp-1/attestations"`);
    expect(text).not.toContain(`${new URL(pageUrl).origin}/api/`);
    expect(text).not.toContain("localhost:8080");
  });
});
