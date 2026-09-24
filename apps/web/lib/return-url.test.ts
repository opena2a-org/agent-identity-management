import { describe, expect, it } from "vitest";
import { safeReturnUrl } from "@/lib/return-url";

// safeReturnUrl receives the value after searchParams.get, which has already decoded it once.
describe("safeReturnUrl", () => {
  it("follows a same-origin path, with its query and fragment", () => {
    expect(safeReturnUrl("/dashboard/agents")).toBe("/dashboard/agents");
    expect(safeReturnUrl("/admin/registrations")).toBe("/admin/registrations");
    expect(safeReturnUrl("/device?user_code=ABCD-1234")).toBe("/device?user_code=ABCD-1234");
    expect(safeReturnUrl("/agents?tab=keys#top")).toBe("/agents?tab=keys#top");
  });

  it("refuses anything that could leave the origin", () => {
    for (const bad of ["https://evil.example/", "//evil.example/x", "/\\evil.example", "javascript:alert(1)", "%ZZ", ""]) {
      expect(safeReturnUrl(bad), bad).toBe("/dashboard");
    }
    expect(safeReturnUrl(null)).toBe("/dashboard");
  });

  // Browsers strip tab, LF and CR while parsing a URL, so "/<TAB>/host" becomes "//host" and leaves the origin
  // (measured on AIM main f0ffcf55, unit 10014).
  it("refuses a control character anywhere in the value", () => {
    for (const bad of ["/\t/evil.example", "/\n/evil.example", "/\r/evil.example", "/\t\\evil.example", "/\u0000/evil.example", "/\u007f/evil.example", "/ok\t/x"]) {
      expect(safeReturnUrl(bad), JSON.stringify(bad)).toBe("/dashboard");
    }
  });

  // The page decodes once through searchParams.get; a second decode turned "%2F%09%2Fevil.example" into "/<TAB>/evil.example".
  it("decodes exactly once", () => {
    expect(safeReturnUrl("%2F%09%2Fevil.example")).toBe("/dashboard");
    expect(safeReturnUrl("%2Fadmin%2Fregistrations")).toBe("/dashboard");
    expect(safeReturnUrl("%2F%2Fevil.example")).toBe("/dashboard");
  });

  // Dot segments normalise to a leading "//" ("/..//evil.example" -> "//evil.example"); the returned path is checked
  // after normalisation, because pushing "//host" leaves the origin.
  it("refuses a path that normalises to a protocol-relative URL", () => {
    for (const bad of ["/..//evil.example", "/.//evil.example", "/%2e%2e//evil.example", "/a/..//evil.example", "/./\\evil.example", "/..\\/evil.example"]) {
      expect(safeReturnUrl(bad), JSON.stringify(bad)).toBe("/dashboard");
    }
    expect(safeReturnUrl("/a/../agents")).toBe("/agents");
  });
});
