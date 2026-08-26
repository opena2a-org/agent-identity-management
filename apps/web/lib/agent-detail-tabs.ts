/**
 * Agent detail tab groups. The page once showed nine flat tabs (MCPs,
 * Capabilities, Violations, Identity & Signing, API Keys, Tags, Recent
 * activity, Trust Score, Details); they are grouped by the question a reader
 * brings to the page. Every pre-grouping tab name is still accepted in
 * `?tab=` and resolves to its group plus the section to scroll to, so saved
 * links keep landing on the content they named.
 */
export const AGENT_DETAIL_TAB_GROUPS = [
  { value: "overview", label: "Overview", sections: ["details", "trust", "activity"] },
  { value: "connections", label: "Connections", sections: ["mcp-servers", "capabilities"] },
  { value: "security", label: "Security", sections: ["violations", "key-vault"] },
  { value: "access", label: "Access", sections: ["api-keys", "tags"] },
] as const;

export type AgentDetailTabGroup = (typeof AGENT_DETAIL_TAB_GROUPS)[number]["value"];
export type AgentDetailSection = (typeof AGENT_DETAIL_TAB_GROUPS)[number]["sections"][number];

export const DEFAULT_AGENT_DETAIL_TAB: AgentDetailTabGroup = "overview";

/** The nine `?tab=` values accepted before grouping, mapped to the section that now holds each. */
export const LEGACY_AGENT_DETAIL_TABS: Readonly<Record<string, AgentDetailSection>> = {
  connections: "mcp-servers",
  capabilities: "capabilities",
  violations: "violations",
  "key-vault": "key-vault",
  "api-keys": "api-keys",
  tags: "tags",
  activity: "activity",
  trust: "trust",
  details: "details",
};

const GROUPS = new Set<string>(AGENT_DETAIL_TAB_GROUPS.map((g) => g.value));
const SECTION_GROUP = new Map<string, AgentDetailTabGroup>(
  AGENT_DETAIL_TAB_GROUPS.flatMap((g) => g.sections.map((s) => [s, g.value] as const))
);

export interface ResolvedAgentDetailTab {
  group: AgentDetailTabGroup;
  /** Set when the value named a section rather than a group; the page scrolls to it. */
  section: AgentDetailSection | null;
}

/** Resolves a `?tab=` value: a pre-grouping tab name, a group name, a section name, or anything else (-> default). */
export function resolveAgentDetailTab(value: string | null | undefined): ResolvedAgentDetailTab {
  if (!value) return { group: DEFAULT_AGENT_DETAIL_TAB, section: null };
  const legacy = Object.prototype.hasOwnProperty.call(LEGACY_AGENT_DETAIL_TABS, value) ? LEGACY_AGENT_DETAIL_TABS[value] : undefined;
  if (legacy) return { group: SECTION_GROUP.get(legacy) as AgentDetailTabGroup, section: legacy };
  if (GROUPS.has(value)) return { group: value as AgentDetailTabGroup, section: null };
  const group = SECTION_GROUP.get(value);
  if (group) return { group, section: value as AgentDetailSection };
  return { group: DEFAULT_AGENT_DETAIL_TAB, section: null };
}

/** DOM id of a section's wrapper, the target of the deep-link scroll. */
export function agentDetailSectionId(section: AgentDetailSection): string {
  return `agent-section-${section}`;
}
