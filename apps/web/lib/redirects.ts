/**
 * Dashboard route moves: one row per old path, and the only place a moved page's redirect
 * is written. next.config.ts returns dashboardRedirects() from redirects(), so an old URL
 * (a bookmark, a link in an email, a URL an SDK printed) keeps working after its page moves.
 *
 * A row lands in the same commit as the move, never before it: the destination page must
 * exist and the old page must be gone, because a redirect runs before the filesystem and
 * would hide a page still served at the old path. lib/redirects.test.ts checks both against
 * app/, along with the row shape.
 *
 * Every row is permanent (308). Next.js passes the query string through a redirect, so a
 * ?tab= on the old path reaches the new page.
 *
 * Kept free of imports: next.config.ts loads this file at build time, outside the app's
 * module aliases.
 */
export interface RouteMove {
  /** The old path, as Next.js matches it: ":id" names a dynamic segment. */
  source: string;
  /** The page that replaced it. Uses only parameters named in source. */
  destination: string;
}

export const ROUTE_MOVES: readonly RouteMove[] = [
  { source: "/dashboard/admin/compliance", destination: "/dashboard/compliance" },
  { source: "/dashboard/api-keys", destination: "/dashboard/credentials" },
  { source: "/dashboard/sdk-tokens", destination: "/dashboard/credentials" },
];

/** The table in the shape next.config.ts returns from redirects(). */
export function dashboardRedirects() {
  return ROUTE_MOVES.map(({ source, destination }) => ({ source, destination, permanent: true }));
}
