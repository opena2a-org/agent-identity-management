// The "-frontend" hostname rule and the default API host (defaultApiHost, below), in
// one place for their two readers. The client turns the dashboard's hostname into its
// backend's at runtime (getApiUrl in lib/api.ts), and the root layout lists that
// backend in the page's connect-src (lib/runtime-config.ts). Both read the pattern
// below, so the client never derives a backend for a hostname whose page lists no
// derived source.
//
// A hostname takes part when it is made of a-z, 0-9, "." and "-" only and has a
// "-frontend" with 1 to 253 characters before it and at most 253 after it. Its
// backend is the same hostname with "-backend" in place of the first such
// "-frontend". No port: the client drops it too.
//
// The character set stays inside what a Content-Security-Policy host source may
// contain, and it keeps policy syntax in a Host header (";", a space, a quote) out of
// the page's policy. The prefix is lazy so it stops at the first "-frontend" it
// reaches. 253 is the length limit of a host name; with both halves bounded, the
// time to reject a Host does not grow with the square of its length.

// Unanchored, with named groups: FRONTEND_HOSTNAME anchors it, and derivedBackendHost
// reads the groups.
export const FRONTEND_HOST =
  "(?<frontendHostPrefix>[a-z0-9.-]{1,253}?)-frontend(?<frontendHostSuffix>[a-z0-9.-]{0,253})";

const FRONTEND_HOSTNAME = new RegExp(`^${FRONTEND_HOST}$`);

// The backend host for a dashboard hostname, or null when the rule does not apply.
export function derivedBackendHost(hostname: string): string | null {
  const groups = FRONTEND_HOSTNAME.exec(hostname)?.groups;
  return groups ? `${groups.frontendHostPrefix}-backend${groups.frontendHostSuffix}` : null;
}

// The port the API listens on when APP_PORT is not set (the backend's
// internal/config/config.go).
export const DEFAULT_API_PORT = "8080";

const POLICY_HOSTNAME = /^[a-z0-9.-]{1,253}$/;

// The host the client calls when no URL is named anywhere: not by the page, not by the
// build, and not by the "-frontend" rule above. It is the dashboard's hostname with
// DEFAULT_API_PORT, and the root layout lists it in the page's connect-src for the same
// hostnames. Null for a hostname with a character outside a-z, 0-9, "." and "-" (an
// IPv6 address, or policy syntax in a Host header), which the policy does not list; the
// client then calls the dashboard's own origin, which the policy's 'self' covers.
export function defaultApiHost(hostname: string): string | null {
  return POLICY_HOSTNAME.test(hostname) ? `${hostname}:${DEFAULT_API_PORT}` : null;
}
