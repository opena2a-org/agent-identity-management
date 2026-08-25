/**
 * The path to follow after sign-in. Only a same-origin path is accepted; a full URL, a
 * protocol-relative URL, a backslash escape or an undecodable value lands on the dashboard.
 */
export function safeReturnUrl(raw: string | null): string {
  if (!raw) return "/dashboard";
  let value: string;
  try {
    value = decodeURIComponent(raw);
  } catch {
    return "/dashboard";
  }
  return value.startsWith("/") && !value.startsWith("//") && !value.startsWith("/\\") ? value : "/dashboard";
}
