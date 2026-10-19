import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api, type ApiRequestError } from "@/lib/api";

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() } }));

// The administration of the organization's OpenID Connect provider, and the organization a
// refused password sign-in names.

const ORG = "2f1c9a4e-8b7d-4c3a-9e1f-0a2b3c4d5e6f";

function answer(status: number, body?: unknown) {
  const fetchMock = vi.fn<typeof fetch>(
    async () =>
      new Response(body === undefined ? null : JSON.stringify(body), {
        status,
        headers: body === undefined ? {} : { "Content-Type": "application/json" },
      })
  );
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

beforeEach(() => {
  api.clearToken();
  localStorage.clear();
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe("the identity provider administration calls", () => {
  it("reads the provider with GET", async () => {
    const stored = { configured: false, redirectUrl: "https://aim.example.test/auth/oidc/callback" };
    const fetchMock = answer(200, stored);

    await expect(api.getOidcSignInConfiguration()).resolves.toEqual(stored);
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toMatch(/\/api\/v1\/admin\/oidc-sign-in$/);
    expect(init?.method ?? "GET").toBe("GET");
    expect(init?.credentials).toBe("omit");
  });

  it("stores it with PUT and the body as given, an empty client secret included", async () => {
    const body = {
      issuer: "https://idp.example.test",
      clientId: "aim-dashboard",
      clientSecret: "",
      provisioning: { enabled: false },
      requireProviderSignIn: true,
    };
    const fetchMock = answer(200, { configured: true, ...body, clientSecret: undefined, clientSecretSet: true });

    const saved = await api.saveOidcSignInConfiguration(body);
    expect(saved.clientSecretSet).toBe(true);
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toMatch(/\/api\/v1\/admin\/oidc-sign-in$/);
    expect(init?.method).toBe("PUT");
    expect(JSON.parse(String(init?.body))).toEqual(body);
  });

  it("removes it with DELETE", async () => {
    const fetchMock = answer(204);

    await expect(api.removeOidcSignInConfiguration()).resolves.toBeUndefined();
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toMatch(/\/api\/v1\/admin\/oidc-sign-in$/);
    expect(init?.method).toBe("DELETE");
  });
});

describe("a refusal that names an organization", () => {
  it("carries the organization of a password sign-in the provider requirement refuses", async () => {
    answer(403, {
      success: false,
      error: "Your organization requires sign-in through its identity provider. Sign in from your organization's sign-in link.",
      code: "provider_sign_in_required",
      organizationId: ORG,
    });

    const err = await api.loginWithPassword({ email: "member@example.test", password: "the-members-password" }).then(
      () => null,
      (e: ApiRequestError) => e
    );

    expect(err?.status).toBe(403);
    expect(err?.code).toBe("provider_sign_in_required");
    expect(err?.organizationId).toBe(ORG);
    expect(api.getToken()).toBeNull();
  });

  it("carries none when the body names none", async () => {
    answer(401, { success: false, error: "Invalid email or password" });

    const err = await api.loginWithPassword({ email: "member@example.test", password: "a-wrong-password" }).then(
      () => null,
      (e: ApiRequestError) => e
    );

    expect(err?.status).toBe(401);
    expect(err).not.toHaveProperty("organizationId");
  });
});
