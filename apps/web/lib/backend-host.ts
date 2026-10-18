// The "-frontend" hostname rule, in one place for its two readers. The client turns
// the dashboard's hostname into its backend's at runtime (getApiUrl in lib/api.ts),
// and the root layout lists that backend in the page's connect-src
// (lib/runtime-config.ts). Both read the pattern below, so the client never derives a
// backend for a hostname whose page lists no derived source.
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
