/**
 * Capability violations as the dashboard reads them.
 *
 * The backend serializes domain.CapabilityViolation with snake_case JSON tags
 * (apps/backend/internal/domain/capability.go: agent_id, attempted_capability,
 * registered_capabilities, trust_score_impact, is_blocked, source_ip, request_metadata,
 * created_at) while every consumer in this app reads camelCase. The API client maps the
 * wire shape here, once, so consumers see the declared shape whichever casing arrives.
 */
export interface SecurityViolation {
  id: string;
  agentId: string;
  agentName?: string;
  attemptedCapability: string;
  registeredCapabilities: string[];
  severity: string;
  trustScoreImpact: number;
  isBlocked: boolean;
  sourceIp: string;
  requestMetadata: Record<string, unknown>;
  createdAt: string;
}

type Raw = Record<string, unknown>;

function pick<T>(raw: Raw, camel: string, snake: string): T | undefined {
  return (raw[camel] !== undefined ? raw[camel] : raw[snake]) as T | undefined;
}

/** Registered capabilities arrive as an object keyed by capability name, or as a list. */
function capabilityNames(value: unknown): string[] {
  if (Array.isArray(value)) return value.map(String);
  if (value && typeof value === "object") return Object.keys(value as Record<string, unknown>);
  return [];
}

export function normalizeViolation(raw: Raw): SecurityViolation {
  return {
    id: String(pick<string>(raw, "id", "id") ?? ""),
    agentId: String(pick<string>(raw, "agentId", "agent_id") ?? ""),
    agentName: pick<string>(raw, "agentName", "agent_name") ?? undefined,
    attemptedCapability: String(pick<string>(raw, "attemptedCapability", "attempted_capability") ?? ""),
    registeredCapabilities: capabilityNames(pick(raw, "registeredCapabilities", "registered_capabilities")),
    severity: String(pick<string>(raw, "severity", "severity") ?? ""),
    trustScoreImpact: Number(pick<number>(raw, "trustScoreImpact", "trust_score_impact") ?? 0),
    isBlocked: Boolean(pick<boolean>(raw, "isBlocked", "is_blocked") ?? false),
    sourceIp: String(pick<string>(raw, "sourceIp", "source_ip") ?? ""),
    requestMetadata: (pick<Record<string, unknown>>(raw, "requestMetadata", "request_metadata") ?? {}) as Record<string, unknown>,
    createdAt: String(pick<string>(raw, "createdAt", "created_at") ?? ""),
  };
}
