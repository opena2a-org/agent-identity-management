import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { mkdtempSync, rmSync, writeFileSync, readFileSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';

/**
 * docs/testing/sdk-publish-gate.md describes the 36 environment-gated
 * integration cases the publish/CI zero-skip gate allowlists. The doc
 * used to say every one of them is an
 * `it.skipIf(!backendAvailable || !credentialsAvailable)` case; 7 of the
 * 36 gate on a single predicate (6 on `!backendAvailable`, 1 on
 * `!credentialsAvailable`), so the doc now carries a per-file table of
 * the two groups.
 *
 * These tests derive the two groups from the three allowlisted files by
 * classifying every `it.skipIf(<predicate>)(` line into exactly three
 * predicate forms and refuse a doc whose table cells differ from the
 * derived counts. A predicate outside the three forms is refused with a
 * message naming the file and line, never binned into either group.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');
const DOC_PATH = join(REPO_ROOT, 'docs', 'testing', 'sdk-publish-gate.md');

const INTEGRATION_FILES = [
  'sdk/typescript/src/a2a/A2AClient.integration.test.ts',
  'sdk/typescript/src/client/AIMClient.integration.test.ts',
  'sdk/typescript/src/auth/oauth.integration.test.ts',
];

const BOTH_PREDICATE = '!backendAvailable || !credentialsAvailable';
const ONE_PREDICATES = ['!backendAvailable', '!credentialsAvailable'] as const;
const PREDICATE_FORMS = [BOTH_PREDICATE, ...ONE_PREDICATES];

interface SkipIfGroups {
  both: number;
  one: number;
  total: number;
  /** Per single-predicate form: how many cases gate on exactly that one. */
  onePerPredicate: Record<(typeof ONE_PREDICATES)[number], number>;
}

/**
 * Extract the predicate of every `it.skipIf(<predicate>)(` line in the
 * source, tracking parentheses so a predicate that carries its own call
 * parentheses is captured whole. Returns 1-based line numbers.
 */
function skipIfPredicates(source: string): { line: number; predicate: string }[] {
  const out: { line: number; predicate: string }[] = [];
  const lines = source.split('\n');
  const marker = 'it.skipIf(';
  for (let i = 0; i < lines.length; i++) {
    const text = lines[i];
    let from = 0;
    for (;;) {
      const start = text.indexOf(marker, from);
      if (start < 0) break;
      let depth = 1;
      let j = start + marker.length;
      for (; j < text.length && depth > 0; j++) {
        if (text[j] === '(') depth++;
        else if (text[j] === ')') depth--;
      }
      // `j` now sits just past the closing paren of skipIf(...).
      if (depth === 0 && text[j] === '(') {
        out.push({ line: i + 1, predicate: text.slice(start + marker.length, j - 1).trim() });
      }
      from = start + marker.length;
    }
  }
  return out;
}

/**
 * Classify every it.skipIf(<predicate>)( line of `source` into the both
 * group or the one group. Throws, naming `label` and the line, on any
 * predicate that is none of the three known forms.
 */
function classifySkipIfCases(label: string, source: string): SkipIfGroups {
  const groups: SkipIfGroups = {
    both: 0,
    one: 0,
    total: 0,
    onePerPredicate: { '!backendAvailable': 0, '!credentialsAvailable': 0 },
  };
  for (const { line, predicate } of skipIfPredicates(source)) {
    if (predicate === BOTH_PREDICATE) {
      groups.both++;
    } else if (predicate === ONE_PREDICATES[0] || predicate === ONE_PREDICATES[1]) {
      groups.one++;
      groups.onePerPredicate[predicate]++;
    } else {
      throw new Error(
        `${label}:${line}: it.skipIf predicate "${predicate}" is none of the known forms ` +
          PREDICATE_FORMS.map((p) => `"${p}"`).join(', '),
      );
    }
    groups.total++;
  }
  return groups;
}

function deriveGroups(): Map<string, SkipIfGroups> {
  const derived = new Map<string, SkipIfGroups>();
  for (const rel of INTEGRATION_FILES) {
    derived.set(rel, classifySkipIfCases(rel, readFileSync(join(REPO_ROOT, rel), 'utf-8')));
  }
  return derived;
}

/** Split a markdown table row into its trimmed cells. */
function tableCells(line: string): string[] {
  const trimmed = line.trim();
  if (!trimmed.startsWith('|')) return [];
  return trimmed
    .slice(1, trimmed.endsWith('|') ? -1 : undefined)
    .split('|')
    .map((c) => c.trim());
}

/** The leading integer of a table cell such as `3 (2 on ...)`. */
function leadingInt(cell: string, what: string): number {
  const m = /^(\d+)(?:\s|$)/.exec(cell);
  expect(m, `${what}: cell "${cell}" must begin with an integer`).not.toBeNull();
  return Number(m![1]);
}

interface GroupsRow {
  both: number;
  oneCell: string;
  one: number;
  total: number;
}

interface DocTables {
  /** The existing Cases table: file -> cases. */
  cases: Map<string, number>;
  /** The groups table: file -> both / one / total. */
  groups: Map<string, GroupsRow>;
  totals: GroupsRow | undefined;
}

function excludedSection(doc: string): string {
  const start = doc.indexOf('### Excluded');
  expect(start, 'the doc must carry an "### Excluded" subsection').toBeGreaterThanOrEqual(0);
  const rest = doc.slice(start + 3);
  const next = rest.search(/\n##+ /);
  return next < 0 ? doc.slice(start) : doc.slice(start, start + 3 + next);
}

function parseDocTables(section: string): DocTables {
  const cases = new Map<string, number>();
  const groups = new Map<string, GroupsRow>();
  let totals: GroupsRow | undefined;
  for (const line of section.split('\n')) {
    const cells = tableCells(line);
    if (cells.length === 0) continue;
    const first = cells[0].replace(/[`*]/g, '');
    if (cells.length === 2 && INTEGRATION_FILES.includes(first)) {
      cases.set(first, leadingInt(cells[1], `Cases row ${first}`));
    } else if (cells.length === 4 && INTEGRATION_FILES.includes(first)) {
      groups.set(first, {
        both: leadingInt(cells[1], `groups row ${first} both`),
        oneCell: cells[2],
        one: leadingInt(cells[2], `groups row ${first} one`),
        total: leadingInt(cells[3], `groups row ${first} total`),
      });
    } else if (cells.length === 4 && /^total$/i.test(first)) {
      totals = {
        both: leadingInt(cells[1], 'totals both'),
        oneCell: cells[2],
        one: leadingInt(cells[2], 'totals one'),
        total: leadingInt(cells[3], 'totals total'),
      };
    }
  }
  return { cases, groups, totals };
}

const doc = readFileSync(DOC_PATH, 'utf-8');
const section = excludedSection(doc);
const tables = parseDocTables(section);

describe('sdk-publish-gate.md names the two skipIf groups by file', () => {
  it('AIM-20.AC1 the Excluded subsection no longer claims every case gates on both predicates', () => {
    expect(section).not.toMatch(
      /Every one is an `it\.skipIf\(!backendAvailable \|\| !credentialsAvailable\)`/,
    );
  });

  it('AIM-20.AC1 the groups table carries both / one / total per file with the documented values, totals 29 / 7 / 36', () => {
    const expected: Record<string, [number, number, number]> = {
      'sdk/typescript/src/a2a/A2AClient.integration.test.ts': [19, 2, 21],
      'sdk/typescript/src/client/AIMClient.integration.test.ts': [10, 3, 13],
      'sdk/typescript/src/auth/oauth.integration.test.ts': [0, 2, 2],
    };
    for (const rel of INTEGRATION_FILES) {
      const row = tables.groups.get(rel);
      expect(row, `groups table row for ${rel}`).toBeDefined();
      expect([row!.both, row!.one, row!.total], rel).toEqual(expected[rel]);
    }
    expect(tables.totals, 'a totals row').toBeDefined();
    expect([tables.totals!.both, tables.totals!.one, tables.totals!.total]).toEqual([29, 7, 36]);
  });

  it('AIM-20.AC1 the one-predicate cell names the predicate(s) used', () => {
    const expected: Record<string, string[]> = {
      'sdk/typescript/src/a2a/A2AClient.integration.test.ts': ['!backendAvailable'],
      'sdk/typescript/src/client/AIMClient.integration.test.ts': [
        '!backendAvailable',
        '!credentialsAvailable',
      ],
      'sdk/typescript/src/auth/oauth.integration.test.ts': ['!backendAvailable'],
    };
    for (const rel of INTEGRATION_FILES) {
      const row = tables.groups.get(rel);
      expect(row, `groups table row for ${rel}`).toBeDefined();
      const cell = row!.oneCell;
      for (const predicate of ONE_PREDICATES) {
        if (expected[rel].includes(predicate)) {
          expect(cell, `${rel} one-predicate cell`).toContain(predicate);
        } else {
          expect(cell, `${rel} one-predicate cell`).not.toContain(predicate);
        }
      }
    }
  });

  it("AIM-20.AC1 each file's groups total equals its row in the existing Cases table (21, 13, 2)", () => {
    const expected: Record<string, number> = {
      'sdk/typescript/src/a2a/A2AClient.integration.test.ts': 21,
      'sdk/typescript/src/client/AIMClient.integration.test.ts': 13,
      'sdk/typescript/src/auth/oauth.integration.test.ts': 2,
    };
    for (const rel of INTEGRATION_FILES) {
      expect(tables.cases.get(rel), `Cases row for ${rel}`).toBe(expected[rel]);
      const row = tables.groups.get(rel);
      expect(row, `groups table row for ${rel}`).toBeDefined();
      expect(row!.total, `groups total for ${rel}`).toBe(expected[rel]);
    }
  });

  it('AIM-20.AC1 the prose states that a single-predicate case skips when only that one condition is missing on the runner', () => {
    expect(section).toMatch(
      /single-predicate case skips when only that one condition is missing on\s+the runner/,
    );
  });

  it('AIM-20.AC1 the doc carries no internal artifact name', () => {
    expect(doc).not.toMatch(/qgf/i);
    expect(doc).not.toMatch(/ledger/i);
    expect(doc).not.toMatch(/chief[-_ ]?tag/i);
    expect(doc).not.toMatch(/\btodo\//i);
    expect(doc).not.toMatch(/\bTODO-\d/);
  });
});

describe('the groups table is derived from the three files', () => {
  const derived = deriveGroups();

  it('AIM-20.AC2 the derived per-file both / one / total equal the doc table cells', () => {
    let both = 0;
    let one = 0;
    let total = 0;
    for (const rel of INTEGRATION_FILES) {
      const d = derived.get(rel)!;
      const row = tables.groups.get(rel);
      expect(row, `groups table row for ${rel}`).toBeDefined();
      expect([row!.both, row!.one, row!.total], rel).toEqual([d.both, d.one, d.total]);
      both += d.both;
      one += d.one;
      total += d.total;
    }
    expect(tables.totals, 'a totals row').toBeDefined();
    expect([tables.totals!.both, tables.totals!.one, tables.totals!.total]).toEqual([
      both,
      one,
      total,
    ]);
  });

  it('AIM-20.AC2 the derived numbers are 19/2/21, 10/3/13 and 0/2/2', () => {
    const d = INTEGRATION_FILES.map((rel) => {
      const g = derived.get(rel)!;
      return [g.both, g.one, g.total];
    });
    expect(d).toEqual([
      [19, 2, 21],
      [10, 3, 13],
      [0, 2, 2],
    ]);
  });

  it('AIM-20.AC2 the one-predicate cell of each row names exactly the single predicates the file uses', () => {
    for (const rel of INTEGRATION_FILES) {
      const row = tables.groups.get(rel);
      expect(row, `groups table row for ${rel}`).toBeDefined();
      const cell = row!.oneCell;
      for (const predicate of ONE_PREDICATES) {
        if (derived.get(rel)!.onePerPredicate[predicate] > 0) {
          expect(cell, `${rel} one-predicate cell`).toContain(predicate);
        } else {
          expect(cell, `${rel} one-predicate cell`).not.toContain(predicate);
        }
      }
    }
  });
});

describe('the classifier refuses a predicate outside the three forms', () => {
  let workDir: string;

  beforeAll(() => {
    workDir = mkdtempSync(join(tmpdir(), 'skipif-groups-'));
  });

  afterAll(() => {
    rmSync(workDir, { recursive: true, force: true });
  });

  it('AIM-20.AC3 a scratch copy of oauth.integration.test.ts with one planted it.skipIf(process.env.SKIP_ME)( case throws naming that line', () => {
    const rel = 'sdk/typescript/src/auth/oauth.integration.test.ts';
    const original = readFileSync(join(REPO_ROOT, rel), 'utf-8');
    const lines = original.split('\n');
    // Plant the foreign case right after the file's last it.skipIf line.
    const lastSkipIf = lines.reduce(
      (acc, line, i) => (line.includes('it.skipIf(') ? i : acc),
      -1,
    );
    expect(lastSkipIf, 'the oauth file carries an it.skipIf line').toBeGreaterThanOrEqual(0);
    const planted = [
      ...lines.slice(0, lastSkipIf + 1),
      "    it.skipIf(process.env.SKIP_ME)('planted', () => {});",
      ...lines.slice(lastSkipIf + 1),
    ].join('\n');
    const plantedLine = lastSkipIf + 2;
    const scratch = join(workDir, 'oauth.integration.test.ts');
    writeFileSync(scratch, planted);

    expect(() => classifySkipIfCases(scratch, readFileSync(scratch, 'utf-8'))).toThrow(
      new RegExp(`${scratch.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}:${plantedLine}:`),
    );
    expect(() => classifySkipIfCases(scratch, planted)).toThrow(/process\.env\.SKIP_ME/);
  });

  it('AIM-20.AC3 the unmodified three files classify without throwing', () => {
    for (const rel of INTEGRATION_FILES) {
      expect(() =>
        classifySkipIfCases(rel, readFileSync(join(REPO_ROOT, rel), 'utf-8')),
      ).not.toThrow();
    }
  });
});
