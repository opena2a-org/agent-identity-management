import { runInNewContext } from "node:vm";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

// next build compiles next/font calls; here the call only has to return a class name.
vi.mock("next/font/google", () => ({ Inter: () => ({ variable: "font-inter" }) }));

// The Host header of the request the layout renders.
const request = vi.hoisted(() => ({ host: "localhost:3001" as string | null }));
vi.mock("next/headers", () => ({
  headers: async () => new Headers(request.host ? { host: request.host } : {}),
}));

import RootLayout from "./layout";

// The page the root layout renders, parsed back into a document.
async function renderedPage(): Promise<Document> {
  const html = renderToStaticMarkup(await RootLayout({ children: <main /> }));
  return new DOMParser().parseFromString(html, "text/html");
}

function connectPolicy(page: Document): string | null | undefined {
  return page.querySelector('head > meta[http-equiv="Content-Security-Policy"]')?.getAttribute("content");
}

// What the inline scripts in the page's head leave in window.__RUNTIME_CONFIG__.
function runtimeConfig(page: Document): unknown {
  const window: { __RUNTIME_CONFIG__?: unknown } = {};
  for (const script of page.querySelectorAll("head > script:not([src])")) {
    runInNewContext(script.textContent ?? "", { window });
  }
  return window.__RUNTIME_CONFIG__;
}

afterEach(() => {
  vi.unstubAllEnvs();
  request.host = "localhost:3001";
});

describe("the root layout", () => {
  it("hands the browser the server's NEXT_PUBLIC_API_URL and lists its origin in the page's connect-src", async () => {
    vi.stubEnv("NEXT_PUBLIC_API_URL", "http://localhost:8081");
    const page = await renderedPage();

    expect(runtimeConfig(page)).toEqual({ apiUrl: "http://localhost:8081" });
    expect(connectPolicy(page)).toBe("connect-src 'self' http://localhost:8081");
  });

  it("hands the browser no URL when the server's environment names none, and lists what the client derives", async () => {
    vi.stubEnv("NEXT_PUBLIC_API_URL", undefined);
    request.host = "aim-frontend.example.com";
    const page = await renderedPage();

    expect(runtimeConfig(page)).toBeUndefined();
    expect(connectPolicy(page)).toBe("connect-src 'self' http://localhost:8080 aim-backend.example.com");
  });
});
