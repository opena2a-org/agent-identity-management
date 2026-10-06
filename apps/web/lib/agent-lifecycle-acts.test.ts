import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { agentLifecycleActs } from "./agent-lifecycle-acts";

describe("agentLifecycleActs", () => {
  it("offers no act on a revoked agent", () => {
    expect(agentLifecycleActs("revoked")).toEqual({ verify: null, suspend: false, reactivate: false });
  });

  it("offers Reactivate only for a suspended agent", () => {
    for (const status of ["pending", "verified", "revoked", "deactivated", "", undefined]) {
      expect(agentLifecycleActs(status).reactivate).toBe(false);
    }
    expect(agentLifecycleActs("suspended")).toEqual({ verify: null, suspend: false, reactivate: true });
  });

  it("offers Suspend only for a pending or verified agent", () => {
    expect(agentLifecycleActs("pending").suspend).toBe(true);
    expect(agentLifecycleActs("verified").suspend).toBe(true);
    for (const status of ["suspended", "revoked", "deactivated", undefined]) {
      expect(agentLifecycleActs(status).suspend).toBe(false);
    }
  });

  it("offers Verify for a pending agent and shows a verified agent as done", () => {
    expect(agentLifecycleActs("pending").verify).toBe("offered");
    expect(agentLifecycleActs("verified").verify).toBe("done");
    for (const status of ["suspended", "revoked", "Verified", undefined]) {
      expect(agentLifecycleActs(status).verify).toBeNull();
    }
  });
});

// The agent page renders each lifecycle button from agentLifecycleActs, not from its own
// status comparison: before, Suspend rendered for every status but `suspended`, revoked
// included, and Verify for every status.
describe("agent page lifecycle buttons", () => {
  const page = readFileSync(path.resolve(__dirname, "../app/dashboard/agents/[id]/page.tsx"), "utf8");

  it("gates Verify, Suspend and Reactivate on the table", () => {
    // A verified agent's "done" verify is a status badge, so the button needs "offered".
    expect(page.includes(`canManage && lifecycleActs.verify === "offered" && (`), "verify button gated on lifecycleActs").toBe(true);
    for (const act of ["suspend", "reactivate"]) {
      expect(page.includes(`canManage && lifecycleActs.${act} && (`), `${act} button gated on lifecycleActs`).toBe(true);
    }
  });

  it("compares no status to decide a lifecycle button", () => {
    expect(page.match(/canManage && agent\.status[^&]*/g) ?? []).toEqual([]);
  });
});
