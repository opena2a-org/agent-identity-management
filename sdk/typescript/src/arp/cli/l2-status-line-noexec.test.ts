/**
 * The CLI's L2 status line must ASK the coordinator rather than decide for itself.
 *
 * Two assertions, deliberately separate: that the call is there, and that the gate
 * is not restated in any spelling we have seen. The first does not depend on having
 * enumerated the spellings; the second is a known-shape list, not a proof.
 *
 * #457 replaced a status line that re-derived L2's predicate — `enabled !== false`,
 * then `enabled === true && adapter` — and printed "3-Layer (L0+L1+L2)" for installs
 * where L2 was not running. The fix routes the line through `describeL2Status`, which
 * settles the question by building the adapter.
 *
 * The independent review of that PR (F-1) found the fix itself unpinned: a mutant that
 * restates the predicate inline in `cli/index.ts` and leaves `describeL2Status` unused
 * SURVIVED the whole suite, because nothing exercises this file. So the property the PR
 * claims — that the line cannot drift from the gate — held only while someone kept the
 * call there.
 *
 * `cli/index.ts` invokes `main()` at module scope, so it cannot be imported to be
 * exercised. This reads it instead, the same shape as `cited-commands.test.ts`. A
 * source-reading check is worth only its positive controls, so both are asserted in
 * this same run: that the extractor really found the status line, and that the mutant
 * detector really fires on a planted mutant.
 */

import { describe, expect, it } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';

const CLI_SOURCE = readFileSync(join(__dirname, 'index.ts'), 'utf8');

/**
 * The body of `startGuard`, which is where the status line is printed.
 *
 * Scoped rather than whole-file on purpose: `cli/index.ts` legitimately tests
 * `intelligence?.enabled === false` elsewhere (the Guard public key, a different
 * decision with a deliberately different asymmetry), and a whole-file rule would
 * either flag that or have to carve it out by name.
 */
function startGuardBody(source: string): string {
  const open = source.indexOf('async function startGuard(');
  if (open === -1) return '';
  // Functions in this file are declared at column 0, so the next line that is exactly
  // a closing brace ends the body.
  const end = source.indexOf('\n}\n', open);
  return end === -1 ? source.slice(open) : source.slice(open, end);
}

/**
 * A restatement of L2's gate, in every spelling we have actually seen.
 *
 * The comparison forms were the first cut. They missed the TRUTHINESS spelling
 * (`ic && ic.enabled && ic.adapter`), which the delta review found surviving
 * this rule — the mutant was still killed, but by the `describeL2Status` call
 * assertion below rather than by the rule that names the property. A string
 * rule needs a rule per spelling, so the conjunction forms are pinned too.
 *
 * This is a list of known shapes, not a proof of the invariant. That is why the
 * call assertion is a separate test rather than a convenience: it is the half
 * that does not depend on having enumerated the spellings.
 */
function restatesL2Gate(body: string): boolean {
  return (
    /\benabled\s*(?:===|!==|==|!=)\s*(?:true|false)/.test(body) ||
    /\benabled\s*\?/.test(body) ||
    /\benabled\b\s*(?:&&|\|\|)/.test(body) ||
    /(?:&&|\|\|)\s*[\w.?]*\benabled\b/.test(body) ||
    /\badapter\b\s*(?:&&|\|\|)/.test(body) ||
    /(?:&&|\|\|)\s*[\w.?]*\badapter\b/.test(body) ||
    /\?\.adapter\s*(?:\?|&&|\|\|)/.test(body)
  );
}

describe('the CLI status line asks describeL2Status rather than re-deriving it', () => {
  const body = startGuardBody(CLI_SOURCE);

  it('positive control: the extractor found the real status line', () => {
    // If this fails the two assertions below are vacuous, so it is asserted first.
    expect(body).not.toBe('');
    expect(body).toContain('3-Layer (L0+L1+L2');
    expect(body).toContain('L0+L1 only (L2 not running');
  });

  it('positive control: the detector fires on a planted inline predicate', () => {
    // The exact mutant the review reported as surviving.
    const mutant = `async function startGuard() {
      const l2 = config.intelligence?.enabled === true ? config.intelligence.adapter : undefined;
      console.log(\`  Intelligence: \${l2 ? '3-Layer (L0+L1+L2' : 'L0+L1 only (L2 not running'}\`);
    `;
    expect(restatesL2Gate(mutant)).toBe(true);
    // The truthiness spelling, which the first cut of this rule missed.
    expect(restatesL2Gate('const on = ic && ic.enabled && ic.adapter;')).toBe(true);
    expect(restatesL2Gate("const l2 = describeL2Status(config.intelligence);")).toBe(false);
  });

  it('calls describeL2Status', () => {
    expect(body).toContain('describeL2Status(');
    expect(CLI_SOURCE).toMatch(/import\s*\{[^}]*\bdescribeL2Status\b[^}]*\}\s*from\s*'\.\.\/intelligence\/coordinator'/);
  });

  it('does not restate the gate inline', () => {
    expect(restatesL2Gate(body)).toBe(false);
  });
});
