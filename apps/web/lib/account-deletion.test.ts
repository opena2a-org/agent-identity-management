import { describe, expect, it } from "vitest";
import type { ApiRequestError } from "@/lib/api";
import {
  ACCOUNT_DELETION_NEUTRAL_MESSAGE,
  INVALID_PASSWORD_CODE,
  INVALID_PASSWORD_MESSAGE,
  LAST_ACTIVE_ADMIN_CODE,
  LAST_ACTIVE_ADMIN_MESSAGE,
  SESSION_ENDED_MESSAGE,
  accountDeletionFailure,
} from "@/lib/account-deletion";
import { SERVER_ERROR_MESSAGE } from "@/lib/error-messages";

const refusal = (status: number | undefined, message: string, extra: Partial<ApiRequestError> = {}) =>
  Object.assign(new Error(message), { status, ...extra }) as ApiRequestError;

describe("accountDeletionFailure", () => {
  it("marks the password for a refused password, and links the Users page for the only active admin", () => {
    expect(accountDeletionFailure(refusal(401, "Invalid password", { code: INVALID_PASSWORD_CODE }), true)).toEqual({
      message: INVALID_PASSWORD_MESSAGE,
      marksPassword: true,
      linksUsers: false,
    });
    expect(accountDeletionFailure(refusal(409, "server words", { code: LAST_ACTIVE_ADMIN_CODE }), true)).toEqual({
      message: LAST_ACTIVE_ADMIN_MESSAGE,
      marksPassword: false,
      linksUsers: true,
    });
  });

  it("says the session has ended whenever none is stored after the request", () => {
    for (const error of [new Error("Unauthorized"), refusal(401, "Invalid password", { code: INVALID_PASSWORD_CODE })]) {
      expect(accountDeletionFailure(error, false).message).toBe(SESSION_ENDED_MESSAGE);
    }
  });

  it("says when to try again after a rate limit, from its Retry-After when it has one", () => {
    expect(accountDeletionFailure(refusal(429, "Rate limit exceeded.", { retryAfterSeconds: 30 }), true).message).toBe(
      "Too many requests. Try again in 30 seconds."
    );
    expect(accountDeletionFailure(refusal(429, "Rate limit exceeded."), true).message).toBe(
      "Too many requests. Try again in a few minutes."
    );
  });

  it("shows the server error line and the server's own words as they are", () => {
    expect(accountDeletionFailure(refusal(500, SERVER_ERROR_MESSAGE), true).message).toBe(SERVER_ERROR_MESSAGE);
    expect(accountDeletionFailure(refusal(400, "password is required"), true).message).toBe("password is required");
  });

  it("shows the neutral line when the server sent no words of its own", () => {
    for (const error of [new TypeError("Failed to fetch"), refusal(400, "HTTP 400"), refusal(403, "Request failed"), null]) {
      expect(accountDeletionFailure(error, true)).toEqual({
        message: ACCOUNT_DELETION_NEUTRAL_MESSAGE,
        marksPassword: false,
        linksUsers: false,
      });
    }
  });
});
