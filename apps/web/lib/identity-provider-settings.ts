import type {
  ApiRequestError,
  OidcProvisionedRole,
  OidcSignInConfiguration,
  OidcSignInConfigurationInput,
} from "@/lib/api";
import { describeActionFailure, type ActionFailure } from "@/lib/action-outcome";
import { organizationIdParam, signInPageHref } from "@/lib/oidc-sign-in";

// The settings page of the organization's OpenID Connect provider
// (/dashboard/admin/identity-provider): the form it edits, the checks it makes before it
// saves, and the body it sends to PUT /api/v1/admin/oidc-sign-in. The server checks each
// rule again, and reads the issuer's configuration document, before it stores anything.

/** What the form holds; emailDomains is the text the administrator typed. */
export interface IdentityProviderForm {
  issuer: string;
  clientId: string;
  clientSecret: string;
  provisioningEnabled: boolean;
  provisioningRole: OidcProvisionedRole;
  emailDomains: string;
  requireProviderSignIn: boolean;
}

export type IdentityProviderField = "issuer" | "clientId" | "clientSecret" | "emailDomains";

export type IdentityProviderFormErrors = Partial<Record<IdentityProviderField, string>>;

/** The roles a provisioned account can have. The server never provisions an administrator. */
export const PROVISIONED_ROLES: { value: OidcProvisionedRole; label: string }[] = [
  { value: "viewer", label: "Viewer" },
  { value: "member", label: "Member" },
  { value: "manager", label: "Manager" },
];

/** The most email domains the server takes. */
export const MAX_EMAIL_DOMAINS = 20;

/** The form for the stored provider, or an empty one. The client secret is never filled in. */
export function identityProviderForm(cfg: OidcSignInConfiguration | null): IdentityProviderForm {
  const stored = cfg?.configured ? cfg : null;
  const provisioning = stored?.provisioning?.enabled ? stored.provisioning : null;
  return {
    issuer: stored?.issuer ?? "",
    clientId: stored?.clientId ?? "",
    clientSecret: "",
    provisioningEnabled: !!provisioning,
    provisioningRole: provisioning?.role ?? "viewer",
    emailDomains: (provisioning?.emailDomains ?? []).join(", "),
    requireProviderSignIn: !!stored?.requireProviderSignIn,
  };
}

/** The domains typed, split at commas, spaces and line breaks. */
export function emailDomainList(text: string): string[] {
  return text.split(/[\s,]+/).filter(Boolean);
}

/**
 * Whether a save may leave the client secret empty: a secret is stored, and the form names
 * the stored issuer and client id, the only case in which the server keeps it.
 */
export function keepsStoredClientSecret(cfg: OidcSignInConfiguration | null, form: IdentityProviderForm): boolean {
  return !!(
    cfg?.configured &&
    cfg.clientSecretSet &&
    form.issuer.trim() === cfg.issuer &&
    form.clientId.trim() === cfg.clientId
  );
}

function isHttpsUrl(value: string): boolean {
  try {
    return new URL(value).protocol === "https:";
  } catch {
    return false;
  }
}

/** The fields the form cannot be saved with, each with the line shown under it. */
export function identityProviderFormErrors(
  cfg: OidcSignInConfiguration | null,
  form: IdentityProviderForm
): IdentityProviderFormErrors {
  const errors: IdentityProviderFormErrors = {};
  const issuer = form.issuer.trim();
  if (!issuer) {
    errors.issuer = "Enter your provider's issuer URL.";
  } else if (!isHttpsUrl(issuer)) {
    errors.issuer = "The issuer must be an https URL, such as https://idp.example.com.";
  }
  if (!form.clientId.trim()) {
    errors.clientId = "Enter the client ID your provider registered for this deployment.";
  }
  if (!form.clientSecret.trim() && !keepsStoredClientSecret(cfg, form)) {
    errors.clientSecret =
      cfg?.configured && cfg.clientSecretSet
        ? "Enter the client secret. The stored one is kept only for the same issuer and client ID."
        : "Enter the client secret your provider issued.";
  }
  if (form.provisioningEnabled) {
    const domains = emailDomainList(form.emailDomains);
    if (domains.length === 0) {
      errors.emailDomains = "Enter at least one email domain, such as example.com.";
    } else if (domains.length > MAX_EMAIL_DOMAINS) {
      errors.emailDomains = `Enter at most ${MAX_EMAIL_DOMAINS} email domains.`;
    }
  }
  return errors;
}

/** The body of PUT /api/v1/admin/oidc-sign-in for the form. An empty secret keeps the stored one. */
export function identityProviderInput(form: IdentityProviderForm): OidcSignInConfigurationInput {
  return {
    issuer: form.issuer.trim(),
    clientId: form.clientId.trim(),
    clientSecret: form.clientSecret.trim(),
    provisioning: form.provisioningEnabled
      ? { enabled: true, role: form.provisioningRole, emailDomains: emailDomainList(form.emailDomains) }
      : { enabled: false },
    requireProviderSignIn: form.requireProviderSignIn,
  };
}

/** The organization's sign-in link on this dashboard, or null without an organization id. */
export function organizationSignInLink(origin: string, organizationId: string | null | undefined): string | null {
  const id = organizationIdParam(organizationId ?? null);
  return id ? `${origin}${signInPageHref(id)}` : null;
}

/**
 * Why a save failed. The route's 400 answers are its own fixed lines, each naming the
 * setting it refuses and what it takes, so they are shown as they are; any other failure
 * is described from its status alone.
 */
export function identityProviderSaveFailure(error: unknown): ActionFailure {
  const failure = error as ApiRequestError | null;
  const message = error instanceof Error ? error.message.trim() : "";
  if (failure?.status === 400 && message) {
    return { kind: "failure", reason: message, nextStep: "Correct the setting and save again." };
  }
  return describeActionFailure(error, { action: "save the identity provider" });
}
