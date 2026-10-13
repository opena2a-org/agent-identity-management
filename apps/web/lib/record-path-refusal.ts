import type { ApiRequestError } from "@/lib/api";

// The reasonCode of a change the server refused because its audit record could not be
// written (503). The change is made only together with its record, so nothing changed.
export const RECORD_UNAVAILABLE_CODE = "recordUnavailable";

/** What a dialog or form shows, in its own alert, when the record path refused its action. */
export interface RecordPathRefusal {
  title: string;
  description: string;
}

// When to try again, from the refusal's Retry-After. Without one, no time is promised.
export function retryAfterText(seconds: number | undefined): string {
  if (seconds === undefined) return "Try again in a few minutes.";
  if (seconds <= 0) return "Try again now.";
  if (seconds < 60) return `Try again in ${seconds} ${seconds === 1 ? "second" : "seconds"}.`;
  const minutes = Math.ceil(seconds / 60);
  return `Try again in ${minutes} ${minutes === 1 ? "minute" : "minutes"}.`;
}

/**
 * The refusal to show when error is the record path's refusal, or null for any other
 * error. action is the verb phrase of the action, as in "AIM did not <action>":
 * "delete this agent". The text is the dashboard's own, never the server's.
 */
export function recordPathRefusal(error: unknown, action: string): RecordPathRefusal | null {
  const { code, retryAfterSeconds } = (error ?? {}) as ApiRequestError;
  if (code !== RECORD_UNAVAILABLE_CODE) return null;
  return {
    title: "Nothing was changed",
    description:
      `AIM did not ${action}: it writes an audit record of this change together with the change, ` +
      `and the record could not be written. Nothing you entered caused this. ` +
      `${retryAfterText(retryAfterSeconds)} ` +
      `If it fails again, an administrator can find the cause in the AIM server log.`,
  };
}
