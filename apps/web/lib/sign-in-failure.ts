import type { ApiRequestError } from "@/lib/api";

/** What the sign-in form shows when the server refuses or the request does not complete. */
export interface SignInFailure {
  message: string;
  /** The refusal names both the email and the password, so both inputs are marked invalid. */
  marksCredentials: boolean;
}

// The server's line for a wrong email or password. It names both fields.
export const INVALID_CREDENTIALS_MESSAGE = "Invalid email or password";

// Shown when the server sent no words of its own: the request never reached it, or it
// answered without a message. It does not say the credentials were wrong.
export const SIGN_IN_NEUTRAL_MESSAGE =
  "The sign-in request did not complete. Try again, and contact your administrator if it keeps happening.";

// The texts request() in lib/api.ts falls back to when a refusal carries no message:
// they are the client's words, not the server's.
const CLIENT_FALLBACK_MESSAGE = /^(HTTP \d{3}|Request failed)$/;

export function signInFailure(error: unknown): SignInFailure {
  const status = (error as ApiRequestError | null)?.status;
  const message = error instanceof Error ? error.message.trim() : "";
  if (typeof status !== "number" || !message || CLIENT_FALLBACK_MESSAGE.test(message)) {
    return { message: SIGN_IN_NEUTRAL_MESSAGE, marksCredentials: false };
  }
  return {
    message,
    marksCredentials: status === 401 && message === INVALID_CREDENTIALS_MESSAGE,
  };
}
