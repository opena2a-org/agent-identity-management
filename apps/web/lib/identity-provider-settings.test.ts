import { describe, expect, it } from "vitest";
import type { ApiRequestError, OidcSignInConfiguration } from "@/lib/api";
import {
  emailDomainList,
  identityProviderForm,
  identityProviderFormErrors,
  identityProviderInput,
  identityProviderSaveFailure,
  keepsStoredClientSecret,
  organizationSignInLink,
} from "@/lib/identity-provider-settings";

const ORG = "2f1c9a4e-8b7d-4c3a-9e1f-0a2b3c4d5e6f";
const stored: OidcSignInConfiguration = {
  configured: true,
  issuer: "https://idp.example.test",
  clientId: "aim-dashboard",
  clientSecretSet: true,
  provisioning: { enabled: true, role: "manager", emailDomains: ["example.com", "example.org"] },
  requireProviderSignIn: true,
  redirectUrl: "https://aim.example.test/auth/oidc/callback",
};

function refusal(status: number | undefined, message: string): ApiRequestError {
  const err = new Error(message) as ApiRequestError;
  err.status = status;
  return err;
}

describe("the identity provider form", () => {
  it("holds the stored provider, never its secret, and starts empty without one", () => {
    expect(identityProviderForm(stored)).toEqual({
      issuer: "https://idp.example.test",
      clientId: "aim-dashboard",
      clientSecret: "",
      provisioningEnabled: true,
      provisioningRole: "manager",
      emailDomains: "example.com, example.org",
      requireProviderSignIn: true,
    });
    const empty = {
      issuer: "",
      clientId: "",
      clientSecret: "",
      provisioningEnabled: false,
      provisioningRole: "viewer",
      emailDomains: "",
      requireProviderSignIn: false,
    };
    expect(identityProviderForm(null)).toEqual(empty);
    expect(identityProviderForm({ configured: false, redirectUrl: stored.redirectUrl })).toEqual(empty);
    expect(identityProviderForm({ ...stored, provisioning: { enabled: false } })).toMatchObject({
      provisioningEnabled: false,
      provisioningRole: "viewer",
      emailDomains: "",
    });
  });

  it("splits the domains at commas, spaces and line breaks", () => {
    expect(emailDomainList(" example.com,example.org\nexample.net  example.io, ")).toEqual([
      "example.com",
      "example.org",
      "example.net",
      "example.io",
    ]);
    expect(emailDomainList(" , ")).toEqual([]);
  });

  it("keeps the stored secret only for the stored issuer and client id", () => {
    const form = identityProviderForm(stored);
    expect(keepsStoredClientSecret(stored, form)).toBe(true);
    expect(keepsStoredClientSecret(stored, { ...form, issuer: " https://idp.example.test " })).toBe(true);
    expect(keepsStoredClientSecret(stored, { ...form, issuer: "https://idp.example.test/" })).toBe(false);
    expect(keepsStoredClientSecret(stored, { ...form, clientId: "another-client" })).toBe(false);
    expect(keepsStoredClientSecret({ ...stored, clientSecretSet: false }, form)).toBe(false);
    expect(keepsStoredClientSecret(null, form)).toBe(false);
  });

  it("sends the trimmed fields, and provisioning only when it is enabled", () => {
    const form = {
      ...identityProviderForm(stored),
      issuer: " https://idp.example.test ",
      clientId: " aim-dashboard ",
      clientSecret: " not-a-real-client-secret ",
    };
    expect(identityProviderInput(form)).toEqual({
      issuer: "https://idp.example.test",
      clientId: "aim-dashboard",
      clientSecret: "not-a-real-client-secret",
      provisioning: { enabled: true, role: "manager", emailDomains: ["example.com", "example.org"] },
      requireProviderSignIn: true,
    });
    expect(identityProviderInput({ ...form, provisioningEnabled: false }).provisioning).toEqual({ enabled: false });
  });

  it("names each field the form cannot be saved with", () => {
    const form = identityProviderForm(stored);
    expect(identityProviderFormErrors(stored, form)).toEqual({});
    expect(identityProviderFormErrors(stored, { ...form, issuer: "idp.example.test", clientId: " " })).toEqual({
      issuer: "The issuer must be an https URL, such as https://idp.example.com.",
      clientId: "Enter the client ID your provider registered for this deployment.",
      clientSecret: "Enter the client secret. The stored one is kept only for the same issuer and client ID.",
    });
    expect(identityProviderFormErrors(null, { ...form, emailDomains: " , " })).toEqual({
      clientSecret: "Enter the client secret your provider issued.",
      emailDomains: "Enter at least one email domain, such as example.com.",
    });
    expect(identityProviderFormErrors(stored, { ...form, provisioningEnabled: false, emailDomains: "" })).toEqual({});
  });
});

describe("the sign-in link", () => {
  it("is the organization's sign-in page on this dashboard, and null without an organization id", () => {
    expect(organizationSignInLink("https://aim.example.test", ORG.toUpperCase())).toBe(
      `https://aim.example.test/auth/login?organization=${ORG}`
    );
    expect(organizationSignInLink("https://aim.example.test", "acme")).toBeNull();
    expect(organizationSignInLink("https://aim.example.test", undefined)).toBeNull();
  });
});

describe("a failed save", () => {
  it("shows the route's refusal of a setting as it is", () => {
    expect(identityProviderSaveFailure(refusal(400, "clientId is required, at most 512 characters"))).toEqual({
      kind: "failure",
      reason: "clientId is required, at most 512 characters",
      nextStep: "Correct the setting and save again.",
    });
  });

  it("describes any other failure from its status, never from the server's words", () => {
    expect(identityProviderSaveFailure(refusal(500, "pq: duplicate key")).reason).toBe(
      "AIM could not save the identity provider."
    );
    expect(identityProviderSaveFailure(refusal(403, "Admin access required")).reason).toBe(
      "Your role does not allow you to save the identity provider."
    );
    expect(identityProviderSaveFailure(refusal(400, "  ")).reason).toBe("AIM could not save the identity provider.");
  });
});
