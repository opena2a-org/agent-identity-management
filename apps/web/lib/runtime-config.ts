import { defaultApiHost, derivedBackendHost } from "./backend-host";
import { connectSourcePolicy } from "./content-security-policy";

// The API URL a dashboard page hands the browser, read when the page is rendered.
//
// next build replaces process.env.NEXT_PUBLIC_API_URL with the value it read, in the
// browser's code and in the server's, so that value is fixed in a prebuilt image. The
// root layout (app/layout.tsx) reads NEXT_PUBLIC_API_URL from the running server's
// environment each time it renders a page, through pageApiConfig below. When the
// variable is set, the page hands it to the browser as window.__RUNTIME_CONFIG__.apiUrl,
// which getApiUrl in lib/api.ts takes before its other sources, and the page's
// connect-src lists its origin. When it is not set, the page hands over no URL and
// getApiUrl resolves one from its other sources: the -backend host of a -frontend
// hostname (lib/backend-host.ts), else the URL the build read, else the API's default
// port on the dashboard's host (defaultApiHost in lib/backend-host.ts). connect-src
// then lists the build's origin, and the -backend host where one is derived; when the
// build read no URL and no -backend host is derived, it lists the default host.
//
// The layout derives both hosts from the Host header the server receives, without its
// port and in lower case, as Next.js matches a header rule's host. The browser derives
// them from the address bar, so a reverse proxy in front of the dashboard has to pass
// the public host name on in Host.

export const API_URL_VARIABLE = "NEXT_PUBLIC_API_URL";

type Environment = Readonly<Record<string, string | undefined>>;

// NEXT_PUBLIC_API_URL in env, or undefined when it is unset or blank. The name is
// looked up through a constant: next build replaces process.env.NEXT_PUBLIC_API_URL
// only where it is written out, so this lookup is left to the running server.
export function runtimeApiUrl(env: Environment): string | undefined {
  return env[API_URL_VARIABLE]?.trim() || undefined;
}

// The URL the build read, which getApiUrl takes when the page hands over none and the
// hostname derives no backend. Undefined when the build read none.
function builtApiUrl(): string | undefined {
  return process.env.NEXT_PUBLIC_API_URL || undefined;
}

export interface PageApiConfig {
  // Handed to the browser as window.__RUNTIME_CONFIG__.apiUrl; absent when the server's
  // environment names no URL.
  apiUrl?: string;
  // The policy the page carries in its meta element (lib/content-security-policy.ts).
  connectPolicy: string;
}

// What the root layout writes into a page, given the server's environment and the
// request's Host header.
export function pageApiConfig(env: Environment, host: string | null | undefined): PageApiConfig {
  const apiUrl = runtimeApiUrl(env);
  if (apiUrl) return { apiUrl, connectPolicy: connectSourcePolicy({ apiUrl }) };

  const hostname = host ? host.split(":", 1)[0].toLowerCase() : null;
  const derived = hostname ? derivedBackendHost(hostname) : null;
  const built = builtApiUrl();
  const defaultHost = hostname && !derived && !built ? defaultApiHost(hostname) : null;
  return {
    connectPolicy: connectSourcePolicy({
      apiUrl: built,
      extraConnectSources: [derived, defaultHost].filter((source): source is string => source !== null),
    }),
  };
}

// The inline script that hands apiUrl to the browser. JSON.stringify writes the value
// as a string literal, and each "<" in it becomes <, so the value cannot close the
// script element or open a comment inside it.
export function runtimeConfigScript(apiUrl: string): string {
  return `window.__RUNTIME_CONFIG__=${JSON.stringify({ apiUrl }).replace(/</g, "\\u003c")};`;
}
