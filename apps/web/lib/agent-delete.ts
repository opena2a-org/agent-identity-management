// What the dashboard says about deleting an agent. Both delete entry points (the agent
// list and the agent page) render the same dialog, and every sentence it shows comes
// from here.

import {
  describeActionFailure,
  type ActionFailure,
  type ActionFailureContext,
} from "@/lib/action-outcome";

// What DELETE /api/v1/agents/:id reaches. Every per-agent table cascades from agents(id).
// The audit log, API call records, A2A security violations and MCP servers keep their rows
// (their reference to the agent is cleared or was never a foreign key).
export function agentDeleteRemovesText(agentName: string): string {
  return `Deleting "${agentName}" removes the agent and the records that belong to it, including its API keys, stored secrets, capabilities, trust and verification history, and the A2A tasks, messages and consent records it took part in.`;
}

export const AGENT_DELETE_KEPT_TEXT =
  "Audit log entries, API call records, A2A security violations and the MCP servers it registered are kept.";

export const AGENT_DELETE_IRREVERSIBLE_TEXT = "This cannot be undone.";

// A failed delete is described the way the agent page describes its other actions: one
// reason and one next step, chosen from the response's status, never from the server's
// text, which can be a database or proxy message.
export const AGENT_DELETE_FAILURE_CONTEXT: ActionFailureContext = {
  action: "delete this agent",
  missing: {
    reason: "This agent no longer exists in your organization.",
    link: { label: "Back to agents", href: "/dashboard/agents" },
  },
};

export function agentDeleteFailure(err: unknown): ActionFailure {
  return describeActionFailure(err, AGENT_DELETE_FAILURE_CONTEXT);
}

export function agentDeletedNotice(agentName: string): string {
  return `Agent "${agentName}" was deleted.`;
}

// A delete from the agent page navigates to the list, which then states the deletion.
// The name travels in sessionStorage rather than the URL, so a link cannot make the list
// claim an agent was deleted.
const DELETED_NOTICE_KEY = "aim.agentDeletedNotice";

export function rememberAgentDeleted(agentName: string): void {
  try {
    sessionStorage.setItem(DELETED_NOTICE_KEY, agentDeletedNotice(agentName));
  } catch {
    // Storage can be unavailable (private mode, quota); the list then shows no notice.
  }
}

export function takeAgentDeletedNotice(): string | null {
  try {
    const notice = sessionStorage.getItem(DELETED_NOTICE_KEY);
    sessionStorage.removeItem(DELETED_NOTICE_KEY);
    return notice;
  } catch {
    return null;
  }
}
