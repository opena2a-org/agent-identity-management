/**
 * The one classification of a failed verification, shared by every HTTP
 * integration.
 *
 * Before 1.2.0 the express middleware, the fastify preHandler and fastify's
 * `verifyRoute` each carried their own copy of the same `catch` ladder. All
 * three checked for `ActionDeniedError` and `AuthenticationError` and nothing
 * else — but a server-side denial does not arrive as either.
 * `parseAPIError` maps HTTP 403 to `AuthorizationError` (see `../exceptions`),
 * which matched neither branch, so a real denial from AIM fell through to
 * `next(error)` / `throw` and surfaced to the caller as an opaque 500 with no
 * denial reason.
 *
 * Keeping the ladder in one function is what makes the three integrations agree
 * by construction rather than by three people remembering to edit three files.
 */

import {
  ActionDeniedError,
  AuthenticationError,
  AuthorizationError,
} from '../exceptions';

export interface VerificationFailureResponse {
  status: number;
  body: Record<string, unknown>;
}

/**
 * Classify an error thrown by `client.verifyAction`.
 *
 * Returns the status and body to send, or `null` when the error is not a
 * verification outcome and should be re-thrown / passed to the host framework's
 * error handler unchanged.
 *
 * Note what this does NOT do: it never returns a "proceed" result. A failed
 * verification never continues to the route handler, in any integration.
 */
export function classifyVerificationFailure(
  error: unknown
): VerificationFailureResponse | null {
  // AIM answered and refused the action.
  if (error instanceof ActionDeniedError) {
    return {
      status: 403,
      body: {
        error: 'Action denied',
        action: error.action,
        reason: error.reason,
        trustScore: error.trustScore,
      },
    };
  }

  // A 403 on the wire is the server refusing the action. It is the same
  // outcome as above reached by a different route, and it must not be reported
  // as an authentication problem: telling a caller their credentials are bad
  // when policy refused them sends them to the wrong fix. This is the branch
  // that did not exist before 1.2.0.
  if (error instanceof AuthorizationError) {
    return {
      status: 403,
      body: {
        error: 'Action denied',
        reason: error.message,
      },
    };
  }

  if (error instanceof AuthenticationError) {
    return {
      status: 401,
      body: {
        error: 'Authentication required',
        message: error.message,
      },
    };
  }

  return null;
}
