import type { ApiRequestError } from "@/lib/api";

/**
 * The result of a dashboard action, shown inline beside the control that caused it.
 *
 * A failure is one sentence of reason and one next step. The reason is chosen from
 * the response's stated `code` or its HTTP status, never from the server's error
 * text: that text can be an internal message (a database constraint, a stack frame)
 * that tells the reader nothing they can act on.
 */
export type ActionOutcome =
  | { kind: "success"; message: string }
  | {
      kind: "failure";
      reason: string;
      nextStep: string;
      /** A page that helps with the next step, when there is one. */
      link?: { label: string; href: string };
    };

export type ActionFailure = Extract<ActionOutcome, { kind: "failure" }>;

export interface ActionFailureContext {
  /** The verb phrase for the action, as in "AIM could not <action>": "delete this agent". */
  action: string;
  /** Where to go when the resource no longer exists. */
  missing?: { reason: string; link: { label: string; href: string } };
  /** Reasons for the stated `code` values the endpoint returns, keyed by code. */
  codes?: Record<string, Omit<ActionFailure, "kind">>;
}

export function actionSuccess(message: string): ActionOutcome {
  return { kind: "success", message };
}

export function describeActionFailure(error: unknown, context: ActionFailureContext): ActionFailure {
  const { status, code } = (error ?? {}) as ApiRequestError;

  if (code && context.codes && Object.prototype.hasOwnProperty.call(context.codes, code)) {
    return { kind: "failure", ...context.codes[code] };
  }
  if (status === 403) {
    return {
      kind: "failure",
      reason: `Your role does not allow you to ${context.action}.`,
      nextStep: "Ask an organization administrator or manager to do it, or to change your role.",
    };
  }
  if (status === 404 && context.missing) {
    return {
      kind: "failure",
      reason: context.missing.reason,
      nextStep: "Open the list to see the current state.",
      link: context.missing.link,
    };
  }
  // fetch rejects with a TypeError when the request never reached the server.
  if (status === undefined && error instanceof TypeError) {
    return {
      kind: "failure",
      reason: `AIM could not be reached, so it did not ${context.action}.`,
      nextStep: "Check your network connection, then try again.",
    };
  }
  return {
    kind: "failure",
    reason: `AIM could not ${context.action}.`,
    nextStep: "Try again. If it fails again, an administrator can find the cause in the AIM server log.",
  };
}
