/**
 * Escapes a value for interpolation into an HTML document string. Used by the
 * ABOM print export, which writes server- and agent-supplied names into a new
 * window with document.write: every data value goes through this first.
 */
export function escapeHtml(value: unknown): string {
  if (value === null || value === undefined) return "";
  return String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}
