// Content-Security-Policy for every dashboard page.
//
// The session bearer lives in localStorage, so any script running on the dashboard
// origin can read it. The policy narrows what such a script can reach: script loads
// only from the dashboard origin; fetch, XMLHttpRequest, WebSocket, EventSource and
// beacon requests go only to the dashboard origin and the API origin the client
// resolves; and it allows no plugins, no <base> pointing at another origin, no form
// post to another origin and no framing. It does not close every way out: img-src
// allows any https: origin, so an image request can still carry data to one.
//
// The policy comes in two parts, because next.config.ts sets the header when the
// dashboard is built and the API origin is read when a page is rendered
// (lib/runtime-config.ts). The header carries every directive, with a connect-src that
// allows the dashboard origin and any http: or https: URL. The root layout
// (app/layout.tsx) writes a second policy into each page as a meta element: connect-src
// with the dashboard origin and the origins the browser may call the API at. Each
// policy a page carries is enforced on its own, so a request has to pass both.
// frame-ancestors has no effect in a meta element, which is one reason the header
// keeps every other directive.
//
// script-src keeps 'unsafe-inline': Next.js writes its bootstrap and flight data as
// inline scripts, next-themes sets the theme with one and the root layout hands the
// browser its API URL with one, and a header set in next.config.ts cannot carry a
// per-request nonce. img-src allows https: because a registration request shows the
// profile picture its identity provider returned, from that provider's host.

export const CSP_HEADER = "Content-Security-Policy";

// connect-src in the header: the dashboard origin, which also covers the WebSocket
// next dev opens to it, and any http: or https: URL. The page's policy narrows it.
const HEADER_CONNECT_SOURCES = ["'self'", "http:", "https:"];

export interface HeaderPolicyOptions {
  // next dev needs 'unsafe-eval': React rebuilds server error stacks with eval.
  development?: boolean;
}

export interface ConnectSourcePolicyOptions {
  // The URL the browser calls the API at.
  apiUrl?: string;
  // Sources the page may connect to beyond the dashboard and the API origin.
  extraConnectSources?: string[];
}

function originOf(url: string | undefined): string | null {
  if (!url) return null;
  try {
    const { protocol, origin } = new URL(url);
    return protocol === "http:" || protocol === "https:" ? origin : null;
  } catch {
    // A relative API URL resolves against the dashboard origin, which 'self' covers.
    return null;
  }
}

// The policy the header carries.
export function headerContentSecurityPolicy({ development = false }: HeaderPolicyOptions = {}): string {
  const directives: Array<[string, string[]]> = [
    ["default-src", ["'self'"]],
    ["script-src", ["'self'", "'unsafe-inline'", ...(development ? ["'unsafe-eval'"] : [])]],
    ["style-src", ["'self'", "'unsafe-inline'"]],
    ["img-src", ["'self'", "data:", "blob:", "https:"]],
    ["font-src", ["'self'"]],
    ["connect-src", HEADER_CONNECT_SOURCES],
    ["object-src", ["'none'"]],
    ["base-uri", ["'self'"]],
    ["form-action", ["'self'"]],
    ["frame-ancestors", ["'none'"]],
  ];
  return directives.map(([name, sources]) => `${name} ${sources.join(" ")}`).join("; ");
}

// The policy a page carries in its meta element: connect-src with the dashboard origin,
// the API URL's origin when it is an absolute http: or https: URL, and
// extraConnectSources.
export function connectSourcePolicy({ apiUrl, extraConnectSources = [] }: ConnectSourcePolicyOptions = {}): string {
  const apiOrigin = originOf(apiUrl);
  return ["connect-src", "'self'", ...(apiOrigin ? [apiOrigin] : []), ...extraConnectSources].join(" ");
}

// The header rule for next.config.ts: the same policy on every path, whatever the Host.
// Next.js sets a rule's headers on what it serves itself: a page, a static file, a
// not-found answer. It does not set them on a path next.config.ts rewrites to the
// backend, nor on a redirect from redirects() or for a trailing slash, so those
// responses do not carry this policy.
// The source is "/(.*)". The built server sets the header on "/" with "/:path*" as
// well; the servedHeaders helper in next-config.test.ts matches a source strictly,
// and under strict matching "/:path*" does not match "/".
export function dashboardHeaderRules({ development = false }: HeaderPolicyOptions = {}) {
  return [
    {
      source: "/(.*)",
      headers: [{ key: CSP_HEADER, value: headerContentSecurityPolicy({ development }) }],
    },
  ];
}
