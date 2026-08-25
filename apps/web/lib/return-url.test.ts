import { describe, expect, it } from "vitest";
import { safeReturnUrl } from "@/lib/return-url";

describe("safeReturnUrl", () => {
  it("follows a same-origin path", () => {
    expect(safeReturnUrl("/dashboard/agents")).toBe("/dashboard/agents");
    expect(safeReturnUrl("%2Fadmin%2Fregistrations")).toBe("/admin/registrations");
  });

  it("refuses anything that could leave the origin", () => {
    for (const bad of ["https://evil.example/", "//evil.example/x", "/\\evil.example", "javascript:alert(1)", "%ZZ", ""]) {
      expect(safeReturnUrl(bad), bad).toBe("/dashboard");
    }
    expect(safeReturnUrl(null)).toBe("/dashboard");
  });
});
