/**
 * The fields an audit list row shows for a record from GET /api/v1/admin/audit-logs:
 * actor, action, target and absolute time. The route also sends ipAddress, userAgent
 * and metadata; no audit list renders them, so they are not part of this type.
 */
export interface AuditRecord {
  id: string;
  action: string;
  resourceType: string;
  resourceId: string;
  timestamp: string;
  userId?: string | null;
  userName?: string;
  agentId?: string | null;
  agentName?: string;
}

export type AuditActor =
  | { kind: "user"; id: string; label: string }
  | { kind: "agent"; id: string; label: string }
  | { kind: "none"; label: string };

/** Records per page on the audit page; the route's own default is 100. */
export const AUDIT_PAGE_SIZE = 50;

/** Shown when a record carries neither a user nor an agent. */
export const NO_ACTOR_LABEL = "No user or agent recorded";

const NIL_UUID = "00000000-0000-0000-0000-000000000000";

const ACRONYMS = new Set(["api", "mcp", "sdk", "jit", "a2a"]);

function present(id: string | null | undefined): id is string {
  return typeof id === "string" && id !== "" && id !== NIL_UUID;
}

function shortId(id: string): string {
  return id.slice(0, 8);
}

/**
 * Who a record says acted, from the members the route sends. A user act names the
 * user, an agent act names the agent; a record with neither says so rather than
 * attributing it to "system".
 */
export function auditActor(record: AuditRecord): AuditActor {
  if (present(record.userId)) {
    return { kind: "user", id: record.userId, label: record.userName?.trim() || `User ${shortId(record.userId)}` };
  }
  if (present(record.agentId)) {
    return { kind: "agent", id: record.agentId, label: record.agentName?.trim() || `Agent ${shortId(record.agentId)}` };
  }
  return { kind: "none", label: NO_ACTOR_LABEL };
}

function humanize(value: string): string {
  const words = value
    .split(/[_\s]+/)
    .filter(Boolean)
    .map((w) => (ACRONYMS.has(w.toLowerCase()) ? w.toUpperCase() : w.toLowerCase()));
  if (words.length === 0) return value;
  const text = words.join(" ");
  return text.charAt(0).toUpperCase() + text.slice(1);
}

/** "refresh_token_reuse" -> "Refresh token reuse". */
export function auditActionLabel(action: string): string {
  return humanize(action);
}

/** "api_key" -> "API key", "mcp_server" -> "MCP server". */
export function auditTargetLabel(resourceType: string): string {
  return humanize(resourceType);
}

const pad = (n: number) => String(n).padStart(2, "0");

/** An absolute time in UTC, "2026-10-01 13:18:21 UTC"; an unparseable value is returned as sent. */
export function formatAuditTime(timestamp: string): string {
  const d = new Date(timestamp);
  if (Number.isNaN(d.getTime())) return timestamp;
  return (
    `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())} ` +
    `${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}:${pad(d.getUTCSeconds())} UTC`
  );
}

/** A copy ordered newest first; records with an unparseable time keep their place at the end. */
export function newestFirst(records: AuditRecord[]): AuditRecord[] {
  const time = (r: AuditRecord) => {
    const t = new Date(r.timestamp).getTime();
    return Number.isNaN(t) ? -Infinity : t;
  };
  return records
    .map((record, index) => ({ record, index }))
    .sort((a, b) => time(b.record) - time(a.record) || a.index - b.index)
    .map(({ record }) => record);
}
