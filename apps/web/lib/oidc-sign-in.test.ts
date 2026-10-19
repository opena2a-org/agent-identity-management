import { afterEach, describe, expect, it } from "vitest";
import type { ApiRequestError } from "@/lib/api";
import { SIGN_IN_NEUTRAL_MESSAGE } from "@/lib/sign-in-failure";
import {
  OIDC_PROVIDER_UNREACHABLE_MESSAGE,
  OIDC_SIGN_IN_STORAGE_KEY,
  OIDC_SIGN_IN_UNAVAILABLE_MESSAGE,
  oidcCallbackFailure,
  oidcStartFailure,
  organizationIdParam,
  providerAuthorizationUrl,
  providerErrorMessage,
  rememberOidcSignIn,
  signInPageHref,
  takeOidcSignIn,
} from "@/lib/oidc-sign-in";

const ORG = "2f1c9a4e-8b7d-4c3a-9e1f-0a2b3c4d5e6f";

function refusal(status: number, message: string, code?: string): ApiRequestError {
  const err = new Error(message) as ApiRequestError;
  err.status = status;
  if (code) err.code = code;
  return err;
}

afterEach(() => {
  sessionStorage.clear();
});

describe("organizationIdParam", () => {
  it("takes a UUID, in lower case, and nothing else", () => {
    expect(organizationIdParam(ORG)).toBe(ORG);
    expect(organizationIdParam(` ${ORG.toUpperCase()} `)).toBe(ORG);
    expect(organizationIdParam(null)).toBeNull();
    expect(organizationIdParam("")).toBeNull();
    expect(organizationIdParam("acme")).toBeNull();
    expect(organizationIdParam(`${ORG}/../login`)).toBeNull();
    expect(organizationIdParam(`{${ORG}}`)).toBeNull();
  });
});

describe("providerAuthorizationUrl", () => {
  it("passes an https URL and refuses every other address", () => {
    expect(providerAuthorizationUrl("https://idp.example.test/authorize?state=s")).toBe(
      "https://idp.example.test/authorize?state=s"
    );
    expect(providerAuthorizationUrl("http://idp.example.test/authorize")).toBeNull();
    expect(providerAuthorizationUrl("javascript:void(0)")).toBeNull();
    expect(providerAuthorizationUrl("/authorize")).toBeNull();
    expect(providerAuthorizationUrl("")).toBeNull();
    expect(providerAuthorizationUrl(undefined)).toBeNull();
  });
});

describe("the sign-in this tab started", () => {
  it("is read once: the first read removes it", () => {
    rememberOidcSignIn({ organizationId: ORG, flow: `${ORG}.sealed-flow`, returnUrl: "/dashboard/agents" });
    expect(takeOidcSignIn()).toEqual({ organizationId: ORG, flow: `${ORG}.sealed-flow`, returnUrl: "/dashboard/agents" });
    expect(sessionStorage.getItem(OIDC_SIGN_IN_STORAGE_KEY)).toBeNull();
    expect(takeOidcSignIn()).toBeNull();
  });

  it("is none when the tab holds nothing readable, and leaves nothing behind", () => {
    expect(takeOidcSignIn()).toBeNull();
    for (const stored of [
      "not json",
      "null",
      JSON.stringify({ organizationId: ORG }),
      JSON.stringify({ organizationId: ORG, flow: "" }),
      JSON.stringify({ organizationId: "acme", flow: "f" }),
      JSON.stringify({ flow: "f" }),
    ]) {
      sessionStorage.setItem(OIDC_SIGN_IN_STORAGE_KEY, stored);
      expect(takeOidcSignIn(), stored).toBeNull();
      expect(sessionStorage.getItem(OIDC_SIGN_IN_STORAGE_KEY), stored).toBeNull();
    }
  });

  it("returns to a same-origin path only", () => {
    for (const returnUrl of ["//evil.example", "https://evil.example/", "/\\evil.example", 42]) {
      sessionStorage.setItem(OIDC_SIGN_IN_STORAGE_KEY, JSON.stringify({ organizationId: ORG, flow: "f", returnUrl }));
      expect(takeOidcSignIn()?.returnUrl, String(returnUrl)).toBe("/dashboard");
    }
  });
});

describe("signInPageHref", () => {
  it("links the organization's sign-in page and keeps a return path other than the dashboard", () => {
    expect(signInPageHref(null)).toBe("/auth/login");
    expect(signInPageHref(ORG)).toBe(`/auth/login?organization=${ORG}`);
    expect(signInPageHref(ORG, "/dashboard")).toBe(`/auth/login?organization=${ORG}`);
    expect(signInPageHref(ORG, "/dashboard/agents?tab=keys")).toBe(
      `/auth/login?organization=${ORG}&returnUrl=%2Fdashboard%2Fagents%3Ftab%3Dkeys`
    );
  });
});

describe("what a failed start shows", () => {
  it("names an organization without provider sign-in, an unreachable provider, and otherwise the server's words", () => {
    expect(oidcStartFailure(refusal(404, "The organization has no OIDC sign-in"))).toBe(OIDC_SIGN_IN_UNAVAILABLE_MESSAGE);
    expect(oidcStartFailure(refusal(502, "The identity provider could not be reached"))).toBe(
      OIDC_PROVIDER_UNREACHABLE_MESSAGE
    );
    expect(oidcStartFailure(refusal(429, "Too many requests"))).toBe("Too many requests");
    expect(oidcStartFailure(new TypeError("Failed to fetch"))).toBe(SIGN_IN_NEUTRAL_MESSAGE);
  });
});

describe("what a refused callback shows", () => {
  it("gives each refusal code the server answers its own line", () => {
    const codes = [
      "flow_invalid",
      "flow_expired",
      "state_mismatch",
      "provider_changed",
      "organization_inactive",
      "code_refused",
      "id_token_invalid",
      "email_not_verified",
      "no_user",
      "domain_not_provisioned",
      "email_in_other_organization",
      "ambiguous_user",
      "account_inactive",
    ];
    const server = "Sign-in through the identity provider was refused";
    for (const code of codes) {
      const message = oidcCallbackFailure(refusal(401, server, code));
      expect(message, code).not.toBe(server);
      expect(message, code).not.toBe(SIGN_IN_NEUTRAL_MESSAGE);
    }
    expect(oidcCallbackFailure(refusal(401, server, "no_user"))).toBe(
      "No account of this organization has your email address. Ask your administrator to add you."
    );
  });

  it("keeps the server's words for a code it does not know, and the neutral line when nothing answered", () => {
    expect(oidcCallbackFailure(refusal(401, "Sign-in through the identity provider was refused", "__proto__"))).toBe(
      "Sign-in through the identity provider was refused"
    );
    expect(oidcCallbackFailure(refusal(400, "code, state and flow are required"))).toBe("code, state and flow are required");
    expect(oidcCallbackFailure(refusal(502, "The identity provider could not be reached"))).toBe(
      OIDC_PROVIDER_UNREACHABLE_MESSAGE
    );
    expect(oidcCallbackFailure(new TypeError("Failed to fetch"))).toBe(SIGN_IN_NEUTRAL_MESSAGE);
  });
});

describe("what a provider's error redirect shows", () => {
  it("is a fixed line per error, never text from the address", () => {
    expect(providerErrorMessage("access_denied")).toBe(
      "The sign-in was cancelled or refused at your identity provider. Start the sign-in again."
    );
    const fallback = providerErrorMessage("invalid_scope");
    expect(fallback).toMatch(/^Your identity provider did not complete the sign-in\./);
    expect(providerErrorMessage("Call 555-0100 to unlock your account")).toBe(fallback);
    expect(providerErrorMessage("constructor")).toBe(fallback);
  });
});
