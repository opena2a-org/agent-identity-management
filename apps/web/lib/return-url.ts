const BASE = "http://localhost";

/**
 * The path to follow after sign-in. Only a same-origin path is accepted; anything else lands on the dashboard.
 *
 * The value arrives already decoded once by searchParams.get and is not decoded again: a second decode let
 * "%2F%09%2Fevil.example" through. Any control character is refused, because browsers strip tab, LF and CR while
 * parsing, so "/<TAB>/evil.example" navigates to evil.example. The path is then resolved against a fixed base and
 * must keep that origin, and the normalised path is checked again; a prefix test alone is not a control.
 */
export function safeReturnUrl(raw: string | null): string {
  if (!raw) return "/dashboard";
  if (/[\u0000-\u001F\u007F]/.test(raw)) return "/dashboard";
  if (!isSingleSlashPath(raw)) return "/dashboard";
  let url: URL;
  try {
    url = new URL(raw, BASE);
  } catch {
    return "/dashboard";
  }
  if (url.origin !== BASE) return "/dashboard";
  // Dot segments can normalise to a leading "//" ("/..//x" -> "//x"), which a push would read as another host.
  const path = url.pathname + url.search + url.hash;
  return isSingleSlashPath(path) ? path : "/dashboard";
}

function isSingleSlashPath(value: string): boolean {
  return value.startsWith("/") && !value.startsWith("//") && !value.startsWith("/\\");
}
