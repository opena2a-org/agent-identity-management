import { runInNewContext } from "node:vm";
import { afterEach, describe, expect, it, vi } from "vitest";
import { derivedBackendHost } from "./backend-host";
import { pageApiConfig, runtimeApiUrl, runtimeConfigScript } from "./runtime-config";

// pageApiConfig reads the server's environment from its first argument. The URL the
// build read comes from process.env.NEXT_PUBLIC_API_URL, which next build replaces with
// that value and these tests stub.

type RuntimeConfig = { apiUrl?: string } | undefined;

// What a page's inline script leaves in window.__RUNTIME_CONFIG__.
function runScript(script: string): RuntimeConfig {
  const window: { __RUNTIME_CONFIG__?: RuntimeConfig } = {};
  runInNewContext(script, { window });
  return window.__RUNTIME_CONFIG__;
}

afterEach(() => {
  vi.unstubAllEnvs();
});

describe("runtimeApiUrl", () => {
  it("reads NEXT_PUBLIC_API_URL from the environment it is given, without surrounding white space", () => {
    expect(runtimeApiUrl({ NEXT_PUBLIC_API_URL: "http://localhost:8081" })).toBe("http://localhost:8081");
    expect(runtimeApiUrl({ NEXT_PUBLIC_API_URL: " https://api.example.com \n" })).toBe("https://api.example.com");
  });

  it("is undefined when the variable is unset or blank", () => {
    for (const env of [{}, { NEXT_PUBLIC_API_URL: "" }, { NEXT_PUBLIC_API_URL: "  " }, { API_URL: "http://localhost:8081" }]) {
      expect(runtimeApiUrl(env), JSON.stringify(env)).toBeUndefined();
    }
  });
});

describe("pageApiConfig", () => {
  it("hands the browser the server's URL and lists only its origin, whatever the Host", () => {
    vi.stubEnv("NEXT_PUBLIC_API_URL", "http://localhost:8080");

    for (const host of ["localhost:3001", "aim-frontend.example.com", null]) {
      expect(pageApiConfig({ NEXT_PUBLIC_API_URL: "http://localhost:8081" }, host), String(host)).toEqual({
        apiUrl: "http://localhost:8081",
        connectPolicy: "connect-src 'self' http://localhost:8081",
      });
    }
  });

  it("hands the browser no URL when the server's environment names none, and lists the build's", () => {
    vi.stubEnv("NEXT_PUBLIC_API_URL", "https://api.example.com/v1/");
    expect(pageApiConfig({}, "dashboard.example.com")).toEqual({
      connectPolicy: "connect-src 'self' https://api.example.com",
    });

    // A build that read no URL: getApiUrl's last source.
    vi.stubEnv("NEXT_PUBLIC_API_URL", undefined);
    expect(pageApiConfig({ NEXT_PUBLIC_API_URL: " " }, "localhost:3000")).toEqual({
      connectPolicy: "connect-src 'self' http://localhost:8080",
    });
  });

  it("lists no origin for a relative or non-HTTP URL, which 'self' covers or nothing reaches", () => {
    for (const apiUrl of ["/", "javascript:alert(1)", "not a url"]) {
      expect(pageApiConfig({ NEXT_PUBLIC_API_URL: apiUrl }, "localhost:3000"), apiUrl).toEqual({
        apiUrl,
        connectPolicy: "connect-src 'self'",
      });
    }
  });

  // getApiUrl in lib/api.ts derives a backend from the address bar only when the page
  // names no URL. Where it derives one the policy lists it; where it derives none the
  // policy lists nothing more than the build's origin.
  it("lists a derived backend for exactly the hosts the client derives one for", () => {
    vi.stubEnv("NEXT_PUBLIC_API_URL", "https://api.example.com");
    const base = "connect-src 'self' https://api.example.com";
    const hosts = [
      "aim-frontend.blue-1.eastus.azurecontainerapps.io",
      "a-frontend-frontend.example.com",
      "aim-frontend",
      "dashboard.example.com",
      "my_app-frontend.test",
      "-frontend.example.com",
      "x-frontend.example.com; script-src *",
    ];
    // Control: the table holds hosts of both kinds.
    expect(hosts.map(derivedBackendHost)).toEqual(expect.arrayContaining([null, "aim-backend"]));

    for (const host of hosts) {
      const derived = derivedBackendHost(host);
      expect(pageApiConfig({}, host), host).toEqual({ connectPolicy: derived ? `${base} ${derived}` : base });
    }
  });

  it("reads the Host as Next.js matches a header rule's host: without its port, in lower case", () => {
    vi.stubEnv("NEXT_PUBLIC_API_URL", "https://api.example.com");
    expect(pageApiConfig({}, "AIM-Frontend.example.com:3000").connectPolicy).toBe(
      "connect-src 'self' https://api.example.com aim-backend.example.com",
    );
    expect(pageApiConfig({}, null).connectPolicy).toBe("connect-src 'self' https://api.example.com");
  });
});

describe("runtimeConfigScript", () => {
  it("sets window.__RUNTIME_CONFIG__.apiUrl", () => {
    expect(runScript(runtimeConfigScript("http://localhost:8081"))).toEqual({ apiUrl: "http://localhost:8081" });
  });

  it("writes no < for the value to close the script element with, and keeps the value as given", () => {
    const value = 'https://x.example/</script><script>alert(1)</script><!--" ';
    const script = runtimeConfigScript(value);

    expect(script).not.toContain("<");
    expect(runScript(script)).toEqual({ apiUrl: value });
  });
});
