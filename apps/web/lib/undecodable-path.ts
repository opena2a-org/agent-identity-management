/**
 * Dashboard paths that hold a percent-escape decodeURIComponent cannot decode: a "%" not
 * followed by two hex digits, or escapes that are not the UTF-8 encoding of a character, as
 * in /dashboard/agents/%E0%A4%A.
 *
 * Next.js 16.3.6 decodes the dynamic segment of a route (the [id] of /dashboard/agents/[id])
 * while it resolves the route, and throws a DecodeError when the segment does not decode. To
 * answer that error it looks for the Pages Router's /_error page, which this App Router build
 * does not have, so without the rule below /dashboard/agents/%E0%A4%A, /dashboard/agents/%,
 * /dashboard/agents/%ff and /dashboard/mcp/%E0%A4%A are answered with a plain-text 500 and no
 * Content-Security-Policy header.
 *
 * next.config.ts returns undecodablePathRewrites() from rewrites() as beforeFiles, which
 * Next.js applies before it resolves a route. The rule sends such a path to
 * UNDECODABLE_PATH_DESTINATION, a path no page serves, so the dashboard answers it with its
 * not-found page and a 404, as it answers /dashboard/no-such-page. Every dynamic segment of
 * the dashboard is under /dashboard/ (next-config.test.ts checks it). The rule's source
 * starts with /dashboard/, so it matches none of the paths next.config.ts relays to the
 * backend (/api/*, /health, /health/*, /.well-known/agent.json, /.well-known/aip).
 *
 * Kept free of imports: next.config.ts loads this file at build time, outside the app's
 * module aliases.
 */

const HEX = "[0-9A-Fa-f]";

// A UTF-8 continuation byte, 80 to BF.
const TAIL = `%[89ABab]${HEX}`;

// One escaped character decodeURIComponent decodes: a byte from 00 to 7F, or the two to four
// bytes of a character in UTF-8 as RFC 3629 section 4 lists them, which leaves out an overlong
// form, a surrogate and a code point above U+10FFFF.
const DECODABLE_ESCAPE = [
  `%[0-7]${HEX}`, // 00-7F
  `%[Cc][2-9A-Fa-f]${TAIL}`, // C2-CF
  `%[Dd]${HEX}${TAIL}`, // D0-DF
  `%[Ee]0%[ABab]${HEX}${TAIL}`, // E0, then A0-BF
  `%[Ee][1-9A-Ca-c]${TAIL}${TAIL}`, // E1-EC
  `%[Ee][Dd]%[89]${HEX}${TAIL}`, // ED, then 80-9F
  `%[Ee][EeFf]${TAIL}${TAIL}`, // EE-EF
  `%[Ff]0%[9ABab]${HEX}${TAIL}${TAIL}`, // F0, then 90-BF
  `%[Ff][1-3]${TAIL}${TAIL}${TAIL}`, // F1-F3
  `%[Ff]4%8${HEX}${TAIL}${TAIL}`, // F4, then 80-8F
].join("|");

/**
 * A path-to-regexp pattern for text that decodeURIComponent cannot decode: the lookahead
 * fails when the text is made only of characters other than "%" and escapes that decode.
 * At any position at most one alternative can match, because the first escaped byte
 * selects the alternative.
 */
export const UNDECODABLE_TEXT = `(?!(?:[^%]|${DECODABLE_ESCAPE})*$).*`;

/** The path an undecodable dashboard path is rewritten to. No page serves it. */
export const UNDECODABLE_PATH_DESTINATION = "/dashboard/undecodable-path";

/**
 * The rule next.config.ts returns from rewrites() as beforeFiles. The pattern is an unnamed
 * group, so Next.js adds nothing from the matched path to the rewritten request's query.
 */
export function undecodablePathRewrites() {
  return [{ source: `/dashboard/(${UNDECODABLE_TEXT})`, destination: UNDECODABLE_PATH_DESTINATION }];
}
