import { describe, it, expect } from 'vitest';
import { readdirSync, readFileSync } from 'fs';
import { join, relative } from 'path';
import * as ts from 'typescript';

/**
 * A test title in the SDK opens with what the test checks.
 *
 * Titles across the SDK's tests opened with a tracking identifier and an
 * acceptance-criterion number (`AIM-11.AC4 ...`, `CR-002: ...`) that resolve to
 * nothing in this repository, and the ARP pattern tests opened with the
 * pattern's identifier. A pattern identifier is still named, after the
 * behaviour: `detects path traversal (MCP-001)`.
 *
 * The scanner parses every source file under tests/, helpers as well as test
 * files, and every test file under src/, fixtures directories aside. It reads
 * the first argument of a call to describe, it or test, made by name, through a
 * modifier (`it.only`) or through a call that returns the test function
 * (`it.skipIf(cond)`, `describe.each(table)`), when that argument is a string
 * literal in any quote style or a template literal, whose text before the first
 * substitution is read. A title is reported when its text opens with a capital
 * letter, any further capitals or digits, a hyphen and a digit (`AIM-11`,
 * `CR-002`, `MCP-001`). Comments and the text of strings are never read as
 * calls.
 */

const TESTS_DIR = __dirname;
const SDK_ROOT = join(__dirname, '..');
const SOURCE_FILE = /\.[cm]?[jt]s$/;
const TEST_FILE = /\.test\.ts$/;
const TEST_FUNCTIONS = new Set(['describe', 'it', 'test']);
const OPENING_IDENTIFIER = /^[A-Z][A-Z0-9]*-[0-9]/;

/** Every file under `dir` whose name matches `pattern`, at any depth, fixtures directories aside. */
function sourceFiles(dir: string, pattern: RegExp): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      if (entry.name !== 'fixtures') out.push(...sourceFiles(path, pattern));
    } else if (pattern.test(entry.name)) {
      out.push(path);
    }
  }
  return out;
}

/** The name a call's callee starts from: `it` for `it`, `it.only` and `it.skipIf(cond)`. */
function rootName(callee: ts.Expression): string | null {
  if (ts.isIdentifier(callee)) return callee.text;
  if (ts.isPropertyAccessExpression(callee)) return rootName(callee.expression);
  if (ts.isCallExpression(callee)) return rootName(callee.expression);
  return null;
}

/** The text of a title argument: a string literal's text, or a template's text before its first substitution. */
function titleText(argument: ts.Expression | undefined): string | null {
  if (argument === undefined) return null;
  if (ts.isStringLiteral(argument) || ts.isNoSubstitutionTemplateLiteral(argument)) return argument.text;
  if (ts.isTemplateExpression(argument)) return argument.head.text;
  return null;
}

/** Every describe, it and test title in `source`, as `<line>: <title>`, and the ones that open with an identifier. */
function readTitles(fileName: string, source: string): { titles: string[]; opening: string[] } {
  const file = ts.createSourceFile(fileName, source, ts.ScriptTarget.Latest, true);
  const titles: string[] = [];
  const opening: string[] = [];
  const visit = (node: ts.Node): void => {
    if (ts.isCallExpression(node)) {
      const name = rootName(node.expression);
      const title = titleText(node.arguments[0]);
      if (name !== null && TEST_FUNCTIONS.has(name) && title !== null) {
        const { line } = file.getLineAndCharacterOfPosition(node.arguments[0].getStart(file));
        const entry = `${line + 1}: ${title}`;
        titles.push(entry);
        if (OPENING_IDENTIFIER.test(title)) opening.push(entry);
      }
    }
    ts.forEachChild(node, visit);
  };
  visit(file);
  return { titles, opening };
}

describe('test titles open with what the test checks', () => {
  it('no describe, it or test title under tests/ or in a test file under src/ opens with an identifier', () => {
    const files = [...sourceFiles(TESTS_DIR, SOURCE_FILE), ...sourceFiles(join(SDK_ROOT, 'src'), TEST_FILE)];
    const opening: string[] = [];
    const read = new Map<string, string[]>();
    for (const path of files) {
      const name = relative(SDK_ROOT, path);
      const found = readTitles(name, readFileSync(path, 'utf-8'));
      read.set(name, found.titles);
      opening.push(...found.opening.map((entry) => `${name}:${entry}`));
    }

    expect(opening, `titles that open with an identifier:\n${opening.join('\n')}`).toEqual([]);

    // The scan reaches titles in a helper under tests/, a test under tests/ and a test under src/.
    const titlesOf = (name: string): string[] => (read.get(name) ?? []).map((entry) => entry.slice(entry.indexOf(': ') + 2));
    expect(titlesOf(join('tests', 'arp', 'harness', 'dvaa-integration.ts'))).toContain('instruction override (PI-001)');
    expect(titlesOf(join('tests', 'arp', 'patterns', 'ai-threats.test.ts'))).toContain('detects instruction override (PI-001)');
    expect(titlesOf(join('src', 'client', 'AIMClient.validation.test.ts'))).toContain('timeout: -5 throws ConfigurationError');
  });

  it('reports a title that opens with an identifier in each call and quote form, and nothing else', () => {
    const source = [
      "describe('CR-002: fail-closed enforcement', () => {",
      "  it('AIM-11.AC4 a 429 yields RateLimitError', () => {});",
      '  it("AIM-20.AC1 the doc table totals", () => {});',
      '  it(`AIM-19.AC3 refuses ${entry} with exit 2`, () => {});',
      "  it.skipIf(!available)('AIM-06.AC4 the default timeout', () => {});",
      '  it(',
      "    'AIM-06.AC3 (a) a stalled POST is ended',",
      '    async () => {},',
      '  );',
      "  describe.each([1, 2])('MCP-001 case %s', () => {});",
      "  test.only('PI-1 a one-digit identifier', () => {});",
      "  it('detects path traversal (MCP-001)', () => {});",
      "  it('T0: defaultConfig() has L2 off', () => {});",
      '  // it(\'AIM-01.AC1 in a comment\', () => {});',
      "  const sample = \"it('AIM-02.AC1 in a string')\";",
      "  expect(sample).toBe('AIM-03.AC1');",
      "  expect(/AIM-04/.test('AIM-04.AC1')).toBe(true);",
      '});',
    ].join('\n');

    const { titles, opening } = readTitles('sample.test.ts', source);
    expect(opening).toEqual([
      '1: CR-002: fail-closed enforcement',
      '2: AIM-11.AC4 a 429 yields RateLimitError',
      '3: AIM-20.AC1 the doc table totals',
      '4: AIM-19.AC3 refuses ',
      '5: AIM-06.AC4 the default timeout',
      '7: AIM-06.AC3 (a) a stalled POST is ended',
      '10: MCP-001 case %s',
      '11: PI-1 a one-digit identifier',
    ]);
    expect(titles).toContain('12: detects path traversal (MCP-001)');
    expect(titles).toContain('13: T0: defaultConfig() has L2 off');
    expect(titles).toHaveLength(10);
  });
});
