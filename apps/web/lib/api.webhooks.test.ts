import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { readFileSync } from "fs";
import path from "path";
import { fileURLToPath } from "url";
import { api } from "@/lib/api";

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() } }));

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");

// The JSON names of the server's webhook create body,
// application.CreateWebhookRequest. An update takes them without secret.
function createRequestFields(): Set<string> {
  const source = readFileSync(path.join(repoRoot, "apps/backend/internal/application/webhook_service.go"), "utf8");
  const struct = source.split("type CreateWebhookRequest struct {")[1].split("\n}")[0];
  return new Set(Array.from(struct.matchAll(/json:"([A-Za-z]+)/g), (match) => match[1]));
}

function answer(status: number, body: unknown) {
  const fetchMock = vi.fn<typeof fetch>(async () =>
    new Response(status === 204 ? null : JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

const sent = (fetchMock: ReturnType<typeof answer>) => {
  const [url, init] = fetchMock.mock.calls[0];
  return {
    path: new URL(String(url), "http://localhost").pathname,
    method: (init as RequestInit).method ?? "GET",
    body: (init as RequestInit).body ? JSON.parse(String((init as RequestInit).body)) : undefined,
  };
};

const id = "0c6f3c1e-8f0e-4a63-9a43-2f1d8b7e5a10";

describe("api: webhook methods against the server's routes", () => {
  beforeEach(() => {
    api.clearToken();
    localStorage.clear();
    localStorage.setItem("auth_token", "a1");
  });
  afterEach(() => vi.unstubAllGlobals());

  it("reads the create body's fields from the server's request struct", () => {
    expect(createRequestFields()).toEqual(new Set(["name", "url", "events", "isActive", "secret"]));
  });

  it("creates with POST /api/v1/webhooks and a body the server takes, and returns the secret", async () => {
    const fetchMock = answer(201, { id, secret: "s".repeat(64) });
    const created = await api.createWebhook({
      name: "receiver",
      url: "https://hooks.example.com/aim",
      events: ["agent.created"],
      secret: "c".repeat(40),
    });
    const request = sent(fetchMock);
    expect(request).toMatchObject({ path: "/api/v1/webhooks", method: "POST" });
    const fields = createRequestFields();
    for (const key of Object.keys(request.body)) expect(fields, key).toContain(key);
    expect(created.secret).toBe("s".repeat(64));
  });

  it("updates with PUT /api/v1/webhooks/:id and a body without secret", async () => {
    const fetchMock = answer(200, { id });
    await api.updateWebhook(id, {
      name: "receiver",
      url: "https://hooks.example.com/aim",
      events: ["agent.created"],
      isActive: false,
    });
    const request = sent(fetchMock);
    expect(request).toMatchObject({ path: `/api/v1/webhooks/${id}`, method: "PUT" });
    const fields = createRequestFields();
    fields.delete("secret");
    for (const key of Object.keys(request.body)) expect(fields, key).toContain(key);
  });

  it("rotates with POST /api/v1/webhooks/:id/rotate-secret and returns the new secret", async () => {
    const fetchMock = answer(200, { id, secret: "n".repeat(64) });
    const rotation = await api.rotateWebhookSecret(id);
    expect(sent(fetchMock)).toMatchObject({ path: `/api/v1/webhooks/${id}/rotate-secret`, method: "POST" });
    expect(rotation).toEqual({ id, secret: "n".repeat(64) });
  });

  it("reads, lists, tests and deletes on the server's paths", async () => {
    let fetchMock = answer(200, { id });
    await api.getWebhook(id);
    expect(sent(fetchMock)).toMatchObject({ path: `/api/v1/webhooks/${id}`, method: "GET" });

    fetchMock = answer(200, { webhooks: [], total: 0 });
    expect(await api.listWebhooks()).toEqual([]);
    expect(sent(fetchMock)).toMatchObject({ path: "/api/v1/webhooks", method: "GET" });

    fetchMock = answer(200, { success: true, message: "ok", responseCode: 204 });
    await api.testWebhook(id);
    expect(sent(fetchMock)).toMatchObject({ path: `/api/v1/webhooks/${id}/test`, method: "POST" });

    fetchMock = answer(204, null);
    await api.deleteWebhook(id);
    expect(sent(fetchMock)).toMatchObject({ path: `/api/v1/webhooks/${id}`, method: "DELETE" });
  });
});
