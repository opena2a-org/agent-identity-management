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
 *
 * `sanitizeTerminalLine` is the single-line variant, for a server-supplied
 * value printed inside one line of the CLI's own text (`init`): it replaces
 * newlines, tabs and the bidi formatting characters as well.
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

/** The bidirectional formatting characters: ALM, LRM, RLM, U+202A-U+202E and U+2066-U+2069. */
const BIDI_FORMATTING = new Set([
  '\u061c', '\u200e', '\u200f',
  '\u202a', '\u202b', '\u202c', '\u202d', '\u202e',
  '\u2066', '\u2067', '\u2068', '\u2069',
]);
const REPLACEMENT_CHARACTER = '�';

/**
 * Whether `ch` (one code point, or one lone surrogate) cannot be carried by a
 * single printed line: all of C0 (newline, carriage return and tab included),
 * DEL, C1, the bidirectional formatting characters, and a lone surrogate,
 * which has no UTF-8 form.
 */
function cannotBeInALine(ch: string): boolean {
  const code = ch.charCodeAt(0);
  if (code <= 0x1f || (code >= 0x7f && code <= 0x9f)) return true;
  if (ch.length === 1 && code >= 0xd800 && code <= 0xdfff) return true;
  return BIDI_FORMATTING.has(ch);
}

/**
 * Server-supplied text as one terminal line, for a field that is printed
 * inside a sentence (a server's error message, a redirect address). Unlike
 * `sanitizeTerminalText`, each character `cannotBeInALine` names is replaced with U+FFFD,
 * so the text cannot start a new line, move the cursor or reorder how the rest
 * of the line reads, and the replacement shows where it was. Text over
 * `maxChars` characters is cut with an explicit marker, never inside a
 * surrogate pair.
 */
export function sanitizeTerminalLine(raw: string, maxChars: number = TERMINAL_TEXT_MAX_CHARS): string {
  let cleaned = '';
  let count = 0;
  for (const ch of raw) {
    if (count === maxChars) return cleaned + TRUNCATION_MARKER;
    cleaned += cannotBeInALine(ch) ? REPLACEMENT_CHARACTER : ch;
    count++;
  }
  return cleaned;
}
