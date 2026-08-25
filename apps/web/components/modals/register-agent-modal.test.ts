import { describe, expect, it } from "vitest";
import { registrationDefaults } from "@/components/modals/register-agent-modal";

/**
 * Regression for the silent-forever-pending defect (CPO B2, 2026-08-24): the backend's
 * shouldAutoVerifyAgent refuses agents whose name, displayName or description is empty, so
 * the browser payload must always carry all three, byte-identical to the SDK's defaults
 * (sdk/python/aim_sdk/client.py).
 */
describe("registrationDefaults", () => {
  it("fills displayName, description and version for a minimal form", () => {
    const p = registrationDefaults({ name: "my-first-agent", displayName: "", description: "", version: "" });
    expect(p.name).toBe("my-first-agent");
    expect(p.displayName).toBe("my-first-agent");
    expect(p.description).toBe("Agent my-first-agent registered via AIM SDK");
    expect(p.version).toBe("1.0.0");
    for (const v of Object.values(p)) expect(v).not.toBe("");
  });

  it("keeps what the user typed", () => {
    const p = registrationDefaults({ name: "billing", displayName: "Billing bot", description: "Reconciles invoices", version: "2.1.0" });
    expect(p).toEqual({ name: "billing", displayName: "Billing bot", description: "Reconciles invoices", version: "2.1.0" });
  });
});
