import type { ApiRequestError } from "@/lib/api";
import { safeReturnUrl } from "@/lib/return-url";
import { signInFailure } from "@/lib/sign-in-failure";

// Sign-in through an organization's OpenID Connect provider, the dashboard's half.
//
// The sign-in page starts it from an organization's sign-in link
// (/auth/login?organization=<id>): the server answers the provider's authorization URL
// and a flow, and the page keeps the flow in this tab's sessionStorage before it sends
// the browser to the provider. The provider redirects the browser to /auth/oidc/callback
// with a code and a state, and that page sends them to the server with the flow. The
// page removes the flow from storage as it reads it, and a tab that holds no flow sends
// nothing, so the code and state of a callback address alone complete no sign-in.

/** The sessionStorage key of the sign-in this tab started. */
export const OIDC_SIGN_IN_STORAGE_KEY = "oidc_sign_in";

/** A sign-in this tab started and has not completed. */
export interface PendingOidcSignIn {
  organizationId: string;
  /** The flow the server answered at the start, sent back with the provider's code and state. */
  flow: string;
  /** The same-origin path to follow after the sign-in. */
  returnUrl: string;
}

const ORGANIZATION_ID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** The organization id of a sign-in link, in lower case, or null when the value is not a UUID. */
export function organizationIdParam(raw: string | null): string | null {
  const value = (raw ?? "").trim();
  return ORGANIZATION_ID.test(value) ? value.toLowerCase() : null;
}

/** The authorization URL the browser may be sent to: an https URL, or null for anything else. */
export function providerAuthorizationUrl(raw: unknown): string | null {
  if (typeof raw !== "string") return null;
  try {
    const url = new URL(raw);
    return url.protocol === "https:" ? url.href : null;
  } catch {
    return null;
  }
}

/** Keeps a started sign-in for this tab's callback page, in place of any earlier one. */
export function rememberOidcSignIn(pending: PendingOidcSignIn): void {
  sessionStorage.setItem(OIDC_SIGN_IN_STORAGE_KEY, JSON.stringify(pending));
}

/**
 * The sign-in this tab started, removed from storage as it is read, so its flow is sent
 * once at most. Null when the tab holds none, or holds something that is not one.
 */
export function takeOidcSignIn(): PendingOidcSignIn | null {
  let raw: string | null;
  try {
    raw = sessionStorage.getItem(OIDC_SIGN_IN_STORAGE_KEY);
    sessionStorage.removeItem(OIDC_SIGN_IN_STORAGE_KEY);
  } catch {
    return null;
  }
  if (!raw) return null;
  let stored: Record<string, unknown> | null;
  try {
    stored = JSON.parse(raw);
  } catch {
    return null;
  }
  if (!stored || typeof stored !== "object") return null;
  const organizationId = organizationIdParam(typeof stored.organizationId === "string" ? stored.organizationId : null);
  const flow = typeof stored.flow === "string" ? stored.flow : "";
  if (!organizationId || !flow) return null;
  return {
    organizationId,
    flow,
    returnUrl: safeReturnUrl(typeof stored.returnUrl === "string" ? stored.returnUrl : null),
  };
}

/** The sign-in page, with the organization's sign-in button when the organization is known. */
export function signInPageHref(organizationId: string | null, returnUrl = "/dashboard"): string {
  const params = new URLSearchParams();
  if (organizationId) params.set("organization", organizationId);
  if (returnUrl !== "/dashboard") params.set("returnUrl", returnUrl);
  const query = params.toString();
  return query ? `/auth/login?${query}` : "/auth/login";
}

export const OIDC_SIGN_IN_UNAVAILABLE_MESSAGE =
  "Sign-in through an identity provider is not available for this organization. Sign in with your email and password, or ask your administrator for your organization's sign-in link.";

export const OIDC_PROVIDER_UNREACHABLE_MESSAGE =
  "Your organization's identity provider could not be reached. Try again in a few minutes.";

export const OIDC_SIGN_IN_NOT_STARTED_MESSAGE =
  "This browser tab has no sign-in waiting for this answer: the sign-in was started in another tab or browser, or it already completed. Start the sign-in again.";

export const OIDC_PROVIDER_ANSWER_INCOMPLETE_MESSAGE =
  "The identity provider's answer carries no sign-in code. Start the sign-in again.";

export const OIDC_PROVIDER_ADDRESS_REFUSED_MESSAGE =
  "The server answered an identity provider address that is not an https URL, so the browser was not sent there. Contact your administrator.";

export const PROVIDER_SIGN_IN_REQUIRED_MESSAGE =
  'Your organization requires you to sign in through its identity provider. Choose "Sign in with your identity provider" to continue.';

/**
 * The organization whose identity provider a refused password sign-in sends the user to:
 * the organizationId, in lower case, of a 403 with the code provider_sign_in_required. Null
 * for any other failure, and for that refusal when it names no organization id.
 */
export function providerSignInRequiredOrganization(error: unknown): string | null {
  const failure = error as ApiRequestError | null;
  if (failure?.status !== 403 || failure.code !== "provider_sign_in_required") return null;
  return organizationIdParam(typeof failure.organizationId === "string" ? failure.organizationId : null);
}

// The server's reason for a refused callback (the code of its 401), in the words of the
// person signing in. docs/API.md lists the reasons.
const CALLBACK_REFUSALS: Record<string, string> = {
  flow_invalid: "This sign-in does not match the one this browser tab started. Start the sign-in again.",
  state_mismatch: "This sign-in does not match the one this browser tab started. Start the sign-in again.",
  flow_expired: "The sign-in expired before it completed. Start the sign-in again.",
  provider_changed:
    "Your organization's identity provider was changed or removed during the sign-in. Start the sign-in again.",
  organization_inactive: "Your organization is not active. Contact your administrator.",
  code_refused: "The identity provider did not accept the sign-in code. Start the sign-in again.",
  id_token_invalid: "The identity provider's answer did not pass verification. Contact your administrator.",
  email_not_verified:
    "The identity provider did not confirm a verified email address for you. Contact your administrator.",
  no_user: "No account of this organization has your email address. Ask your administrator to add you.",
  domain_not_provisioned:
    "Accounts are created at sign-in only for this organization's email domains, and your email address is not at one of them. Ask your administrator to add you.",
  email_in_other_organization:
    "An account of another organization has your email address, so no account was created in this one. Contact your administrator.",
  ambiguous_user:
    "More than one account of this organization has your email address. Contact your administrator.",
  account_inactive: "Your account is not active. Contact your administrator.",
};

// The provider's own refusal, the error of its redirect (RFC 6749, section 4.1.2.1). Its
// error_description is not shown: anyone can write one into a link to this page.
const PROVIDER_ERRORS: Record<string, string> = {
  access_denied: "The sign-in was cancelled or refused at your identity provider. Start the sign-in again.",
  server_error: "Your identity provider could not complete the sign-in. Try again in a few minutes.",
  temporarily_unavailable: "Your identity provider could not complete the sign-in. Try again in a few minutes.",
};

const PROVIDER_ERROR_FALLBACK =
  "Your identity provider did not complete the sign-in. Start it again, and contact your administrator if it keeps happening.";

/** What the callback page shows for the error the provider redirected with. */
export function providerErrorMessage(error: string): string {
  return Object.hasOwn(PROVIDER_ERRORS, error) ? PROVIDER_ERRORS[error] : PROVIDER_ERROR_FALLBACK;
}

/** What the sign-in page shows when the server does not start a sign-in. */
export function oidcStartFailure(error: unknown): string {
  const status = (error as ApiRequestError | null)?.status;
  if (status === 404) return OIDC_SIGN_IN_UNAVAILABLE_MESSAGE;
  if (status === 502) return OIDC_PROVIDER_UNREACHABLE_MESSAGE;
  return signInFailure(error).message;
}

/** What the callback page shows when the server does not complete the sign-in. */
export function oidcCallbackFailure(error: unknown): string {
  const failure = error as ApiRequestError | null;
  if (failure?.status === 401 && failure.code && Object.hasOwn(CALLBACK_REFUSALS, failure.code)) {
    return CALLBACK_REFUSALS[failure.code];
  }
  if (failure?.status === 502) return OIDC_PROVIDER_UNREACHABLE_MESSAGE;
  return signInFailure(error).message;
}
