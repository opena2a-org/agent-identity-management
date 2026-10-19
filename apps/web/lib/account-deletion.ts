import type { ApiRequestError } from "@/lib/api";
import { retryAfterText } from "@/lib/record-path-refusal";

// The page where a signed-in user asks for the deletion of their own account.
export const DELETE_ACCOUNT_PATH = "/auth/delete-account";

// POST /api/v1/auth/delete-account answers a password it refuses with 401 and this
// code, the same for a wrong password, a locked account and an account without a
// password. A 401 without it refuses the session.
export const INVALID_PASSWORD_CODE = "invalidPassword";

// The 409 code that refuses the organization's only active admin while the
// organization has other users who are not deactivated.
export const LAST_ACTIVE_ADMIN_CODE = "lastActiveAdmin";

export const INVALID_PASSWORD_MESSAGE =
  "The password was not accepted. Enter the password you sign in with.";

export const LAST_ACTIVE_ADMIN_MESSAGE =
  "You are the only active admin of your organization, and it has other users. " +
  "Make another user an admin on the Users page first, then ask again.";

export const SESSION_ENDED_MESSAGE =
  "Your session has ended. Sign in again to ask for the deletion of your account.";

// Shown when the server sent no words of its own: the request never reached it, or it
// answered without a message.
export const ACCOUNT_DELETION_NEUTRAL_MESSAGE =
  "The request did not complete. Try again, and contact your administrator if it keeps happening.";

// The texts request() in lib/api.ts falls back to when a refusal carries no message:
// they are the client's words, not the server's.
const CLIENT_FALLBACK_MESSAGE = /^(HTTP \d{3}|Request failed)$/;

/** What the deletion form shows when the request is refused or does not complete. */
export interface AccountDeletionFailure {
  message: string;
  /** The refusal is about the password, so the input is marked invalid. */
  marksPassword: boolean;
  /** The refusal is resolved on the Users page, so the alert links there. */
  linksUsers: boolean;
}

/**
 * The failure to show for error. signedIn is whether a session is still stored
 * after the request: the client clears it when the server refused the session and
 * a refresh could not replace it.
 */
export function accountDeletionFailure(error: unknown, signedIn: boolean): AccountDeletionFailure {
  const failure = (message: string, marks: Partial<AccountDeletionFailure> = {}) => ({
    message,
    marksPassword: false,
    linksUsers: false,
    ...marks,
  });
  if (!signedIn) return failure(SESSION_ENDED_MESSAGE);

  const { status, code, retryAfterSeconds } = (error ?? {}) as ApiRequestError;
  if (code === INVALID_PASSWORD_CODE) return failure(INVALID_PASSWORD_MESSAGE, { marksPassword: true });
  if (code === LAST_ACTIVE_ADMIN_CODE) return failure(LAST_ACTIVE_ADMIN_MESSAGE, { linksUsers: true });
  if (status === 429) return failure(`Too many requests. ${retryAfterText(retryAfterSeconds)}`);

  // A 5xx arrives as the client's server error line (SERVER_ERROR_MESSAGE), and any
  // other refusal with the server's own words, which are shown as they are.
  const message = error instanceof Error ? error.message.trim() : "";
  if (typeof status !== "number" || !message || CLIENT_FALLBACK_MESSAGE.test(message)) {
    return failure(ACCOUNT_DELETION_NEUTRAL_MESSAGE);
  }
  return failure(message);
}
