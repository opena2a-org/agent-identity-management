/**
 * The lifecycle acts the agent page offers for each agent status. It follows the server's
 * transition table: verify moves `pending` to `verified`, suspend moves `pending` or
 * `verified` to `suspended`, reactivate moves `suspended` to `verified`, and no act moves
 * an agent out of `revoked`. The server refuses any other pair with a 409, so a button for
 * it could only ever show a refusal.
 *
 * `verify` is "done" for a verified agent: the page shows a "Verified" status badge, not a button.
 * A status outside the four (the column has no constraint) offers no act.
 */
export interface AgentLifecycleActs {
  verify: "offered" | "done" | null;
  suspend: boolean;
  reactivate: boolean;
}

export function agentLifecycleActs(status: string | undefined): AgentLifecycleActs {
  switch (status) {
    case "pending":
      return { verify: "offered", suspend: true, reactivate: false };
    case "verified":
      return { verify: "done", suspend: true, reactivate: false };
    case "suspended":
      return { verify: null, suspend: false, reactivate: true };
    default:
      return { verify: null, suspend: false, reactivate: false };
  }
}
