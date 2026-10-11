// Content-Security-Policy for every dashboard page, served from next.config.ts.
//
// The session bearer lives in localStorage, so any script running on the dashboard
// origin can read it. The policy narrows what such a script can reach: script loads
// only from the dashboard origin; fetch, XMLHttpRequest, WebSocket, EventSource and
// beacon requests go only to the dashboard origin and the API origin the client
// resolves; and it allows no plugins, no <base> pointing at another origin, no form
// post to another origin and no framing. It does not close every way out: img-src
// allows any https: origin, so an image request can still carry data to one.
//
// script-src keeps 'unsafe-inline': Next.js writes its bootstrap and flight data as
// inline scripts and next-themes sets the theme with one, and a header set here
// cannot carry a per-request nonce for statically rendered pages. img-src allows
// https: because a registration request shows the profile picture its identity
// provider returned, from that provider's host.

import { DERIVED_BACKEND_SOURCE, FRONTEND_HOST } from "./backend-host";

export const CSP_HEADER = "Content-Security-Policy";

export interface ContentSecurityPolicyOptions {
  // NEXT_PUBLIC_API_URL as the client bundle reads it.
  apiUrl?: string;
  // Sources the page may connect to beyond the dashboard and the API origin.
  extraConnectSources?: string[];
  // next dev needs 'unsafe-eval': React rebuilds server error stacks with eval.
  development?: boolean;
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

export function contentSecurityPolicy({
  apiUrl,
  extraConnectSources = [],
  development = false,
}: ContentSecurityPolicyOptions = {}): string {
  const apiOrigin = originOf(apiUrl);
  const connect = ["'self'", ...(apiOrigin ? [apiOrigin] : []), ...extraConnectSources];
  const directives: Array<[string, string[]]> = [
    ["default-src", ["'self'"]],
    ["script-src", ["'self'", "'unsafe-inline'", ...(development ? ["'unsafe-eval'"] : [])]],
    ["style-src", ["'self'", "'unsafe-inline'"]],
    ["img-src", ["'self'", "data:", "blob:", "https:"]],
    ["font-src", ["'self'"]],
    ["connect-src", connect],
    ["object-src", ["'none'"]],
    ["base-uri", ["'self'"]],
    ["form-action", ["'self'"]],
    ["frame-ancestors", ["'none'"]],
  ];
  return directives.map(([name, sources]) => `${name} ${sources.join(" ")}`).join("; ");
}

// The header rules for next.config.ts. The first rule matches every path; a Host that
// the client turns into a backend host (lib/backend-host.ts) gets the same policy
// with that host added to connect-src. Next.js applies matching rules in order, so
// the second one wins there. The second rule reads the Host header the server
// receives, while the client reads the address bar: a reverse proxy in front of a
// "-frontend" deployment has to pass the public host name on.
// Next.js sets a rule's headers on what it serves itself: a page, a static file, a
// not-found answer. It does not set them on a path next.config.ts rewrites to the
// backend, nor on a redirect from redirects() or for a trailing slash, so those
// responses do not carry this policy.
// The source is "/(.*)". The built server sets the header on "/" with "/:path*" as
// well; the servedHeaders helper in next-config.test.ts matches a source strictly,
// and under strict matching "/:path*" does not match "/".
export function dashboardSecurityHeaders({
  apiUrl,
  development = false,
}: Omit<ContentSecurityPolicyOptions, "extraConnectSources"> = {}) {
  return [
    {
      source: "/(.*)",
      headers: [{ key: CSP_HEADER, value: contentSecurityPolicy({ apiUrl, development }) }],
    },
    {
      source: "/(.*)",
      has: [{ type: "host" as const, value: FRONTEND_HOST }],
      headers: [
        {
          key: CSP_HEADER,
          value: contentSecurityPolicy({
            apiUrl,
            development,
            extraConnectSources: [DERIVED_BACKEND_SOURCE],
          }),
        },
      ],
    },
  ];
}
