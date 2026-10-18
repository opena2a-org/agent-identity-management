import { describe, expect, it } from "vitest";
import { FRONTEND_HOST, derivedBackendHost } from "./backend-host";

describe("derivedBackendHost", () => {
  it("replaces the first -frontend of a hostname with -backend", () => {
    expect(derivedBackendHost("aim-frontend.blue-1.eastus.azurecontainerapps.io")).toBe(
      "aim-backend.blue-1.eastus.azurecontainerapps.io",
    );
    expect(derivedBackendHost("aim-dev-frontend.example.com")).toBe("aim-dev-backend.example.com");
    expect(derivedBackendHost("a-frontend-frontend.example.com")).toBe("a-backend-frontend.example.com");
    // A host with no domain part, as on a container network.
    expect(derivedBackendHost("aim-frontend")).toBe("aim-backend");
    // A -frontend with nothing before it is not the one replaced.
    expect(derivedBackendHost("-frontend-frontend.example.com")).toBe("-frontend-backend.example.com");
  });

  it("derives nothing for a hostname the rule does not cover", () => {
    for (const hostname of [
      "dashboard.example.com",
      "localhost",
      // Nothing before -frontend.
      "-frontend.example.com",
      "-frontend",
      // A character outside a-z, 0-9, "." and "-".
      "my_app-frontend.test",
      "aim-frontend.exa_mple.com",
      "AIM-frontend.example.com",
      "aim-frontend.example.com:3000",
      "x-frontend.example.com; script-src *",
      "",
    ]) {
      expect(derivedBackendHost(hostname), hostname).toBeNull();
    }
  });

  it("bounds each half to a host name's length", () => {
    expect(derivedBackendHost(`${"a".repeat(253)}-frontend`)).toBe(`${"a".repeat(253)}-backend`);
    expect(derivedBackendHost(`${"a".repeat(254)}-frontend`)).toBeNull();
    expect(derivedBackendHost(`a-frontend${"b".repeat(253)}`)).toBe(`a-backend${"b".repeat(253)}`);
    expect(derivedBackendHost(`a-frontend${"b".repeat(254)}`)).toBeNull();
  });

  // Anchored the way derivedBackendHost anchors it. With "+?" and "*"
  // for the two bounds this input takes about 3 seconds at this size: each "-frontend"
  // is tried in turn and the rest of the input rescanned for it.
  it("rejects a long Host of repeated -frontend without rescanning it for each one", () => {
    const pattern = new RegExp(`^${FRONTEND_HOST}$`);
    const host = `a${"-frontend".repeat(Math.floor((256 * 1024) / "-frontend".length))}_`;

    const started = performance.now();
    expect(pattern.test(host)).toBe(false);
    expect(performance.now() - started).toBeLessThan(500);
  });
});
