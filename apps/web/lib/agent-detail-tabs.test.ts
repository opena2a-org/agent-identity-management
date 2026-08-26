import { describe, expect, it } from "vitest";
import {
  AGENT_DETAIL_TAB_GROUPS,
  DEFAULT_AGENT_DETAIL_TAB,
  LEGACY_AGENT_DETAIL_TABS,
  agentDetailSectionId,
  resolveAgentDetailTab,
} from "./agent-detail-tabs";

/** The nine tab values the page accepted before grouping (page.tsx at c9941b1); each must still resolve. */
const LEGACY_TABS = ["connections", "capabilities", "violations", "key-vault", "api-keys", "tags", "activity", "trust", "details"];
const groupOf = (section: string) => AGENT_DETAIL_TAB_GROUPS.find((g) => (g.sections as readonly string[]).includes(section))?.value;

describe("agent detail tab groups", () => {
  it("shows at most four top-level groups", () => {
    expect(AGENT_DETAIL_TAB_GROUPS.length).toBeLessThanOrEqual(4);
    expect(AGENT_DETAIL_TAB_GROUPS.length).toBeGreaterThanOrEqual(3);
  });

  it("places every one of the nine former tabs in exactly one group", () => {
    expect(Object.keys(LEGACY_AGENT_DETAIL_TABS).sort()).toEqual([...LEGACY_TABS].sort());
    const placed = AGENT_DETAIL_TAB_GROUPS.flatMap((g) => [...g.sections]);
    expect(new Set(placed).size).toBe(placed.length);
    for (const legacy of LEGACY_TABS) expect(placed).toContain(LEGACY_AGENT_DETAIL_TABS[legacy]);
    expect(placed.length).toBe(LEGACY_TABS.length);
  });

  it("keeps group and section names disjoint so ?tab= is unambiguous", () => {
    const groups = AGENT_DETAIL_TAB_GROUPS.map((g) => g.value);
    for (const g of AGENT_DETAIL_TAB_GROUPS) for (const s of g.sections) expect(groups).not.toContain(s);
  });

  it("lands a legacy name that matches a group name inside that same group", () => {
    const groups = AGENT_DETAIL_TAB_GROUPS.map((g) => g.value);
    for (const legacy of LEGACY_TABS.filter((t) => (groups as string[]).includes(t))) {
      expect(groupOf(LEGACY_AGENT_DETAIL_TABS[legacy])).toBe(legacy);
    }
  });
});

describe("resolveAgentDetailTab", () => {
  it("resolves a group name to that group, with at most a section of its own", () => {
    for (const g of AGENT_DETAIL_TAB_GROUPS) {
      const r = resolveAgentDetailTab(g.value);
      expect(r.group).toBe(g.value);
      if (r.section !== null) expect(g.sections as readonly string[]).toContain(r.section);
    }
  });

  it("resolves every legacy tab to the group that now holds it, naming the section", () => {
    for (const legacy of LEGACY_TABS) {
      const section = LEGACY_AGENT_DETAIL_TABS[legacy];
      expect(resolveAgentDetailTab(legacy)).toEqual({ group: groupOf(section), section });
    }
  });

  it("resolves a section name to its group, naming the section", () => {
    for (const g of AGENT_DETAIL_TAB_GROUPS) {
      for (const s of g.sections) expect(resolveAgentDetailTab(s)).toEqual({ group: g.value, section: s });
    }
  });

  it("falls back to the default group for missing or unknown values", () => {
    for (const v of [null, undefined, "", "nope", "OVERVIEW"]) {
      expect(resolveAgentDetailTab(v)).toEqual({ group: DEFAULT_AGENT_DETAIL_TAB, section: null });
    }
  });

  it("defaults to overview, the at-a-glance group", () => {
    expect(DEFAULT_AGENT_DETAIL_TAB).toBe("overview");
  });

  it("derives a stable section id", () => {
    expect(agentDetailSectionId("key-vault")).toBe("agent-section-key-vault");
  });
});
