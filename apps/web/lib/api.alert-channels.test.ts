import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { readFileSync } from "fs";
import path from "path";
import { fileURLToPath } from "url";
import { api } from "@/lib/api";
import { SERVER_ERROR_MESSAGE } from "@/lib/error-messages";

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() } }));

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");

// The JSON names of the server's create and update body,
// application.AlertChannelRequest.
function requestFields(): Set<string> {
  const source = readFileSync(
    path.join(repoRoot, "apps/backend/internal/application/alert_channel_service.go"),
    "utf8",
  );
  const struct = source.split("type AlertChannelRequest struct {")[1].split("\n}")[0];
  return new Set(Array.from(struct.matchAll(/json:"([A-Za-z]+)/g), (match) => match[1]));
}

// The routes main.go mounts on the /alert-channels group, as "METHOD /path".
function mountedRoutes(): Set<string> {
  const source = readFileSync(path.join(repoRoot, "apps/backend/cmd/server/main.go"), "utf8");
  return new Set(
    Array.from(
      source.matchAll(/alertChannels\.(Get|Post|Put|Delete)\("([^"]*)"/g),
      (match) => `${match[1].toUpperCase()} /api/v1/alert-channels${match[2] === "/" ? "" : match[2]}`,
    ),
  );
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

const id = "3f1c2b7a-5d4e-4c3b-9a8f-1e2d3c4b5a69";
const route = (request: { method: string; path: string }) =>
  `${request.method} ${request.path.replace(id, ":id")}`;

describe("api: alert channel methods against the server's routes", () => {
  beforeEach(() => {
    api.clearToken();
    localStorage.clear();
    localStorage.setItem("auth_token", "a1");
  });
  afterEach(() => vi.unstubAllGlobals());

  it("reads the body's fields from the server's request struct and the routes from main.go", () => {
    expect(requestFields()).toEqual(new Set(["name", "kind", "config", "minSeverity", "isActive"]));
    expect(mountedRoutes()).toEqual(
      new Set([
        "POST /api/v1/alert-channels",
        "GET /api/v1/alert-channels",
        "GET /api/v1/alert-channels/:id",
        "PUT /api/v1/alert-channels/:id",
        "DELETE /api/v1/alert-channels/:id",
        "POST /api/v1/alert-channels/:id/test",
      ]),
    );
  });

  it("creates with POST /api/v1/alert-channels and a body the server takes", async () => {
    const fetchMock = answer(201, { id });
    await api.createAlertChannel({
      name: "Security on-call",
      kind: "slack",
      // The Slack incoming webhook form, put together at run time so that no file holds one.
      config: { webhookUrl: ["https://hooks.slack.com/services/", "T000", "/B000/", "XXXX"].join("") },
      minSeverity: "warning",
    });
    const request = sent(fetchMock);
    expect(mountedRoutes()).toContain(route(request));
    expect(request).toMatchObject({ path: "/api/v1/alert-channels", method: "POST" });
    const fields = requestFields();
    for (const key of Object.keys(request.body)) expect(fields, key).toContain(key);
  });

  it("updates with PUT /api/v1/alert-channels/:id and sends no config when none is given", async () => {
    const fetchMock = answer(200, { id });
    await api.updateAlertChannel(id, { name: "Security on-call", minSeverity: "high" });
    const request = sent(fetchMock);
    expect(mountedRoutes()).toContain(route(request));
    expect(request).toMatchObject({ path: `/api/v1/alert-channels/${id}`, method: "PUT" });
    expect(request.body).toEqual({ name: "Security on-call", minSeverity: "high" });
  });

  it("lists, reads, tests and deletes on the server's paths", async () => {
    let fetchMock = answer(200, { alertChannels: [], total: 0 });
    expect(await api.listAlertChannels()).toEqual({ alertChannels: [], total: 0 });
    expect(mountedRoutes()).toContain(route(sent(fetchMock)));

    fetchMock = answer(200, { id });
    await api.getAlertChannel(id);
    expect(mountedRoutes()).toContain(route(sent(fetchMock)));
    expect(sent(fetchMock)).toMatchObject({ path: `/api/v1/alert-channels/${id}`, method: "GET" });

    fetchMock = answer(200, { delivered: true, statusCode: 200 });
    expect(await api.testAlertChannel(id)).toEqual({ delivered: true, statusCode: 200 });
    expect(mountedRoutes()).toContain(route(sent(fetchMock)));
    expect(sent(fetchMock)).toMatchObject({ path: `/api/v1/alert-channels/${id}/test`, method: "POST" });
    expect(sent(fetchMock).body).toBeUndefined();

    fetchMock = answer(204, null);
    await api.deleteAlertChannel(id);
    expect(mountedRoutes()).toContain(route(sent(fetchMock)));
    expect(sent(fetchMock)).toMatchObject({ path: `/api/v1/alert-channels/${id}`, method: "DELETE" });
  });

  it("returns the 502 body of a test the destination did not accept", async () => {
    answer(502, { delivered: false, statusCode: 404, error: "status 404" });
    expect(await api.testAlertChannel(id)).toEqual({
      delivered: false,
      statusCode: 404,
      error: "status 404",
    });

    answer(502, { delivered: false, statusCode: null, error: "timeout" });
    expect(await api.testAlertChannel(id)).toEqual({ delivered: false, statusCode: null, error: "timeout" });
  });

  it("keeps a 502 without that body, and any other 5xx, as the server error line", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>(async () => new Response("<html>Bad Gateway</html>", { status: 502 })),
    );
    await expect(api.testAlertChannel(id)).rejects.toMatchObject({ message: SERVER_ERROR_MESSAGE, status: 502 });

    answer(500, { error: "database is down" });
    await expect(api.testAlertChannel(id)).rejects.toMatchObject({ message: SERVER_ERROR_MESSAGE, status: 500 });
  });

  it("throws a 4xx refusal with the server's message and status", async () => {
    answer(404, { error: "not found" });
    await expect(api.testAlertChannel(id)).rejects.toMatchObject({ message: "not found", status: 404 });

    answer(403, { error: "Insufficient permissions" });
    await expect(api.testAlertChannel(id)).rejects.toMatchObject({ status: 403 });
  });
});
