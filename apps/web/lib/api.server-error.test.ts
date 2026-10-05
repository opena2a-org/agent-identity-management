import { readFileSync } from "node:fs";
import path from "node:path";
import { afterEach, describe, expect, it, vi } from "vitest";
import { api, type ApiRequestError } from "@/lib/api";
import { SERVER_ERROR_MESSAGE, getErrorMessage } from "@/lib/error-messages";

// A 5xx is the server's failure. The client shows one fixed line for it,
// never the body's text: an older server put a Go error there (a
// foreign-key violation naming its table), and a proxy answers with HTML.

const repositoryFailure =
  'failed to delete agent: pq: update or delete on table "agents" violates foreign key constraint "fk_verification_events_agent"';

function answerWith(body: string, status: number, contentType = "application/json") {
  vi.stubGlobal(
    "fetch",
    vi.fn(
      async () =>
        new Response(body, { status, headers: { "Content-Type": contentType } })
    )
  );
}

async function failureOf(call: () => Promise<unknown>): Promise<ApiRequestError> {
  try {
    await call();
  } catch (err) {
    return err as ApiRequestError;
  }
  throw new Error("the request did not fail");
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("a 5xx answer", () => {
  it("shows the fixed line, not the server's error text", async () => {
    answerWith(JSON.stringify({ error: repositoryFailure }), 500);
    const err = await failureOf(() => api.getCurrentUser());
    expect(err.message).toBe(SERVER_ERROR_MESSAGE);
    expect(err.status).toBe(500);
  });

  it("shows the fixed line when the body is not JSON", async () => {
    answerWith("<html><body>502 Bad Gateway</body></html>", 502, "text/html");
    const err = await failureOf(() => api.getCurrentUser());
    expect(err.message).toBe(SERVER_ERROR_MESSAGE);
    expect(err.status).toBe(502);
  });

  it("shows the fixed line on the requests that read the response themselves", async () => {
    answerWith(JSON.stringify({ error: repositoryFailure }), 500);
    const changePassword = await failureOf(() =>
      api.changePassword({ email: "a@example.com", currentPassword: "x", newPassword: "y" })
    );
    expect(changePassword.message).toBe(SERVER_ERROR_MESSAGE);

    answerWith("Service Unavailable", 503, "text/plain");
    const exportAuditLogs = await failureOf(() => api.exportAuditLogs());
    expect(exportAuditLogs.message).toBe(SERVER_ERROR_MESSAGE);
    expect(exportAuditLogs.status).toBe(503);
  });

  it("keeps a refusal's own line when the refusal names its reason", async () => {
    answerWith(
      JSON.stringify({ error: "No administrator is configured", reasonCode: "noAdministrators" }),
      503
    );
    const err = await failureOf(() => api.getCurrentUser());
    expect(err.message).toBe("No administrator is configured");
    expect(err.code).toBe("noAdministrators");
  });

  it("leaves a 4xx answer's text alone", async () => {
    answerWith(JSON.stringify({ error: "Invalid agent ID" }), 400);
    expect((await failureOf(() => api.getCurrentUser())).message).toBe("Invalid agent ID");
  });
});

describe("getErrorMessage", () => {
  it.each([500, 501, 502, 503, 504])("returns the fixed line for %i", (status) => {
    const err = Object.assign(new Error(repositoryFailure), { status });
    expect(getErrorMessage(err)).toBe(SERVER_ERROR_MESSAGE);
  });
});

describe("SERVER_ERROR_MESSAGE", () => {
  it("is the line the API declares", () => {
    const source = readFileSync(
      path.resolve(__dirname, "../../backend/internal/interfaces/http/handlers/server_error.go"),
      "utf8"
    );
    expect(source.match(/const ServerErrorMessage = "([^"]*)"/)?.[1]).toBe(SERVER_ERROR_MESSAGE);
  });
});
