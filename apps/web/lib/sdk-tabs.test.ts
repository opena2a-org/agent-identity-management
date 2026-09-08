import { describe, it, expect } from "vitest";
import { SDK_TABS } from "./sdk-tabs";

const ORIGIN = "https://aim.example.test";

describe("SDK_TABS", () => {
  it("offers Python, TypeScript and Java exactly once each", () => {
    expect(SDK_TABS.map((t) => t.key)).toEqual(["python", "typescript", "java"]);
    expect(new Set(SDK_TABS.map((t) => t.label)).size).toBe(3);
  });

  it("gives every tab an install line, an example and an https docs link", () => {
    for (const t of SDK_TABS) {
      expect(t.install("").trim().length, t.key).toBeGreaterThan(0);
      expect(t.install(ORIGIN).trim().length, t.key).toBeGreaterThan(0);
      expect(t.code("").trim().length, t.key).toBeGreaterThan(0);
      expect(t.docsHref, t.key).toMatch(/^https:\/\//);
      expect(t.docsLabel.trim().length, t.key).toBeGreaterThan(0);
    }
  });

  it("signs the Python login in against the deployment origin when one is known", () => {
    const python = SDK_TABS.find((t) => t.key === "python")!;
    expect(python.install(ORIGIN)).toContain(`aim-sdk login --url ${ORIGIN}`);
    expect(python.install("")).toContain("aim-sdk login");
    expect(python.install("")).not.toContain("--url");
  });

  it("points the TypeScript client at the deployment origin, or the hosted API by default", () => {
    const ts = SDK_TABS.find((t) => t.key === "typescript")!;
    expect(ts.code(ORIGIN)).toContain(`baseUrl: "${ORIGIN}"`);
    expect(ts.code("")).toContain('baseUrl: "https://api.aim.opena2a.org"');
    expect(ts.code(ORIGIN)).not.toContain("api.aim.opena2a.org");
  });

  it("never leaks an unfilled template into a rendered command", () => {
    for (const t of SDK_TABS) {
      for (const origin of ["", ORIGIN]) {
        expect(t.install(origin), t.key).not.toMatch(/\$\{|undefined|null/);
        expect(t.code(origin), t.key).not.toMatch(/\$\{|undefined/);
      }
    }
  });
});
