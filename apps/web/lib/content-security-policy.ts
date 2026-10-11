// Content-Security-Policy for every dashboard page, served from next.config.ts.
//
// The session bearer lives in localStorage, so any script running on the dashboard
// origin can read it. The policy bounds that: script loads only from the dashboard
// origin, the page connects only to the dashboard origin and the API origin the
// client resolves, and it allows no plugins, no <base> rewrite, no cross-origin form
// post and no framing by another site.
//
// script-src keeps 'unsafe-inline': Next.js writes its bootstrap and flight data as
// inline scripts and next-themes sets the theme with one, and a header set here
// cannot carry a per-request nonce for statically rendered pages. img-src allows
// https: because a registration request shows the profile picture its identity
// provider returned, from that provider's host.

export const CSP_HEADER = "Content-Security-Policy";

// The client turns a hostname containing "-frontend" into its backend by replacing
// the first "-frontend" with "-backend" (getApiUrl in lib/api.ts), at runtime. The
// header rule captures the same two halves of the request's Host, and Next.js
// substitutes the named groups into the value. The prefix is lazy so it stops at the
// first "-frontend", as String.replace does. No port: the client drops it too.
const FRONTEND_HOST =
  "(?<frontendHostPrefix>[a-z0-9.-]+?)-frontend(?<frontendHostSuffix>[a-z0-9.-]*)";
const DERIVED_BACKEND_SOURCE = ":frontendHostPrefix-backend:frontendHostSuffix";

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

// The header rules for next.config.ts. Every path gets the policy; a Host that the
// client would turn into a backend host gets the same policy with that host added to
// connect-src. Next.js applies matching rules in order, so the second one wins there.
// The source is "/(.*)", not "/:path*": with Next's strict matching the latter does
// not match "/".
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
