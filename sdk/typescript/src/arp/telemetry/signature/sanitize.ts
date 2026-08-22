/**
 * Terminal-text sanitisation for server-supplied strings (the TypeScript half of
 * the class filed as aim #384).
 *
 * A registry error message flows into CLI output (`opt-out`, `purge`, enrollment
 * failures). Unsanitised, it could carry ANSI escapes that rewrite what the
 * operator sees, or be long enough to scroll the real message away. C0 control
 * bytes other than newline and tab, DEL, and the C1 range are stripped; text over
 * the cap is truncated with an explicit marker rather than silently.
 *
 * Applied where the value ENTERS a result object, not at each print site, so a
 * new print site cannot miss it. Unicode bidi controls are deliberately NOT
 * stripped: legitimate RTL text may carry them.
 */

/** Longest server-supplied string allowed into terminal-bound result fields. */
export const TERMINAL_TEXT_MAX_CHARS = 500;
const TRUNCATION_MARKER = ' [truncated]';

export function sanitizeTerminalText(raw: string): string {
  let cleaned = '';
  for (const ch of raw) {
    const isC0 = ch < ' ' && ch !== '\n' && ch !== '\t';
    const isDelOrC1 = ch >= '\x7f' && ch <= '\x9f';
    if (!isC0 && !isDelOrC1) cleaned += ch;
  }
  if (cleaned.length > TERMINAL_TEXT_MAX_CHARS) {
    cleaned = cleaned.slice(0, TERMINAL_TEXT_MAX_CHARS) + TRUNCATION_MARKER;
  }
  return cleaned;
}
