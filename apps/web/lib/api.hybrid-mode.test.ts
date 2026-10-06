import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "@/lib/api";

// The dashboard only ever turns hybrid mode off. The route reads the member
// `enable`; any other name would bind as false and hide a wrong request.

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("turnOffAgentHybridMode", () => {
  it("posts enable false to the agent's hybrid-mode route", async () => {
    const fetchMock = vi.fn(
      async () =>
        new Response(
          JSON.stringify({ agentId: "agent-1", hybridModeEnabled: false, message: "ok" }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        )
    );
    vi.stubGlobal("fetch", fetchMock);

    const result = await api.turnOffAgentHybridMode("agent-1");

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url.endsWith("/api/v1/agents/agent-1/hybrid-mode")).toBe(true);
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual({ enable: false });
    expect(result.hybridModeEnabled).toBe(false);
  });

  it("surfaces the server's refusal text", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(JSON.stringify({ error: "Insufficient permissions" }), {
            status: 403,
            headers: { "Content-Type": "application/json" },
          })
      )
    );

    await expect(api.turnOffAgentHybridMode("agent-1")).rejects.toThrow(
      "Insufficient permissions"
    );
  });
});
