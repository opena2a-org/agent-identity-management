import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { spawnSync } from 'child_process';
import { mkdtempSync, rmSync, writeFileSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';

/**
 * The per-file skip bound on the publish/CI zero-skip gate's allowlist
 * (scripts/check-vitest-report.mjs).
 *
 * Before the bound existed, an --allow-skips entry excused any number of
 * "skipped" assertions inside its file: a 22nd skipIf case added to
 * src/a2a/A2AClient.integration.test.ts passed the gate, and the only
 * witness was the skipped count in the log. Every entry now carries the
 * number of skips it tolerates (--allow-skips=<file>:<bound>); a count
 * above the bound refuses naming the file and the delta, and a bare
 * entry is refused before the report is judged.
 */

const CHECKER = join(__dirname, '..', 'scripts', 'check-vitest-report.mjs');

/**
 * The SDK root the fixtures' absolute report names sit under; the checker
 * matches an entry only against a name made relative to its root, so
 * every run here passes it as --root.
 */
const FIXTURE_ROOT = '/repo/sdk/typescript';

const A2A = 'src/a2a/A2AClient.integration.test.ts';
const CLIENT = 'src/client/AIMClient.integration.test.ts';
const OAUTH = 'src/auth/oauth.integration.test.ts';

const BOUND_FLAGS = [
  `--allow-skips=${A2A}:21`,
  `--allow-skips=${CLIENT}:13`,
  `--allow-skips=${OAUTH}:2`,
];

let workDir: string;

beforeAll(() => {
  workDir = mkdtempSync(join(tmpdir(), 'check-vitest-report-bounds-'));
});

afterAll(() => {
  rmSync(workDir, { recursive: true, force: true });
});

interface AssertionSpec {
  file: string;
  status: string;
  n: number;
}

/** Same vitest-JSON-reporter shape as check-vitest-report.test.ts builds. */
function buildReport(specs: AssertionSpec[]): object {
  const byFile = new Map<string, { status: string; n: number }[]>();
  for (const s of specs) {
    const list = byFile.get(s.file) ?? [];
    list.push({ status: s.status, n: s.n });
    byFile.set(s.file, list);
  }
  const testResults = [...byFile.entries()].map(([file, groups]) => ({
    name: join(FIXTURE_ROOT, file),
    status: 'passed',
    startTime: 0,
    endTime: 1,
    message: '',
    assertionResults: groups.flatMap((g) =>
      Array.from({ length: g.n }, (_, i) => ({
        ancestorTitles: ['suite'],
        fullName: `suite case ${g.status} ${i}`,
        title: `case ${g.status} ${i}`,
        status: g.status,
        failureMessages: [],
      })),
    ),
  }));
  const total = specs.reduce((acc, s) => acc + s.n, 0);
  return {
    numTotalTestSuites: testResults.length,
    numPassedTestSuites: testResults.length,
    numFailedTestSuites: 0,
    numPendingTestSuites: 0,
    numTotalTests: total,
    numPassedTests: total,
    numFailedTests: 0,
    numPendingTests: 0,
    numTodoTests: 0,
    startTime: 0,
    success: true,
    testResults,
  };
}

let fixtureCount = 0;

function writeReport(report: object): string {
  const p = join(workDir, `report-${fixtureCount++}.json`);
  writeFileSync(p, JSON.stringify(report));
  return p;
}

function runChecker(args: string[]) {
  const res = spawnSync('node', [CHECKER, ...args, `--root=${FIXTURE_ROOT}`], {
    encoding: 'utf-8',
  });
  return {
    status: res.status,
    stdout: res.stdout ?? '',
    stderr: res.stderr ?? '',
    output: `${res.stdout ?? ''}${res.stderr ?? ''}`,
  };
}

/** The measured population (21/13/2) with `a2aSkips` in the A2A file. */
function populationReport(a2aSkips: number): object {
  return buildReport([
    { file: A2A, status: 'skipped', n: a2aSkips },
    { file: CLIENT, status: 'skipped', n: 13 },
    { file: OAUTH, status: 'skipped', n: 2 },
    { file: 'src/version.test.ts', status: 'passed', n: 1158 },
  ]);
}

describe('check-vitest-report allowlist: every entry binds its file to a skip bound', () => {
  it('AIM-19.AC1 tolerates skips in an allowlisted file only while their count is at or below the bound, and passes a count below it', () => {
    const atBound = runChecker([writeReport(populationReport(21)), ...BOUND_FLAGS]);
    expect(atBound.status, atBound.output).toBe(0);

    // The environment-gated cases execute instead of skipping when a
    // backend and credentials are reachable, so the count legitimately
    // falls below the bound.
    const belowBound = runChecker([writeReport(populationReport(20)), ...BOUND_FLAGS]);
    expect(belowBound.status, belowBound.output).toBe(0);
    expect(belowBound.stdout).toContain(`${A2A} 20/21`);

    const zeroSkips = runChecker([
      writeReport(buildReport([{ file: 'src/version.test.ts', status: 'passed', n: 3 }])),
      ...BOUND_FLAGS,
    ]);
    expect(zeroSkips.status, zeroSkips.output).toBe(0);
  });

  it('AIM-19.AC1 the ok line lists every entry as <file> <skipped>/<bound> when skips remain', () => {
    const res = runChecker([writeReport(populationReport(21)), ...BOUND_FLAGS]);
    expect(res.status, res.output).toBe(0);
    const okLine = res.stdout.split('\n').find((l) => l.startsWith('ok:')) ?? '';
    expect(okLine).toContain(`${A2A} 21/21`);
    expect(okLine).toContain(`${CLIENT} 13/13`);
    expect(okLine).toContain(`${OAUTH} 2/2`);
  });

  it('AIM-19.AC1 the per-status counts line keeps its text and precedes the verdict', () => {
    const res = runChecker([writeReport(populationReport(21)), ...BOUND_FLAGS]);
    expect(res.status, res.output).toBe(0);
    const lines = res.stdout.split('\n').filter((l) => l.length > 0);
    expect(lines[0]).toBe('tests passed=1158 failed=0 skipped=36 pending=0 todo=0');
    expect(lines[1]).toMatch(/^ok: /);

    const refused = runChecker([writeReport(populationReport(22)), ...BOUND_FLAGS]);
    expect(refused.stdout.split('\n')[0]).toBe(
      'tests passed=1158 failed=0 skipped=37 pending=0 todo=0',
    );
  });

  it('AIM-19.AC2 refuses 22 skips in src/a2a/A2AClient.integration.test.ts against its bound of 21, naming the file, the count, the bound and the delta', () => {
    // The planted fault: one skipIf case added to the A2A file. Before
    // the bound, this report exited 0 and only the counts line
    // (skipped=37) witnessed the growth.
    const res = runChecker([writeReport(populationReport(22)), ...BOUND_FLAGS]);
    expect(res.status, res.output).not.toBe(0);
    expect(res.stderr).toContain(A2A);
    expect(res.stderr).toMatch(/\b22\b/);
    expect(res.stderr).toMatch(/\b21\b/);
    expect(res.stderr).toContain('+1');
    // The other two files sit at their bounds and are not named as offenders.
    expect(res.stderr).not.toContain(CLIENT);
    expect(res.stderr).not.toContain(OAUTH);

    const identicalAtBound = runChecker([writeReport(populationReport(21)), ...BOUND_FLAGS]);
    expect(identicalAtBound.status, identicalAtBound.output).toBe(0);
  });

  it('AIM-19.AC2 an over-bound file is refused even when the other entries are below their bounds', () => {
    const report = buildReport([
      { file: A2A, status: 'skipped', n: 5 },
      { file: CLIENT, status: 'skipped', n: 13 },
      { file: OAUTH, status: 'skipped', n: 3 },
      { file: 'src/version.test.ts', status: 'passed', n: 1000 },
    ]);
    const res = runChecker([writeReport(report), ...BOUND_FLAGS]);
    expect(res.status, res.output).not.toBe(0);
    expect(res.stderr).toContain(OAUTH);
    expect(res.stderr).toMatch(/\b3\b/);
    expect(res.stderr).toMatch(/\b2\b/);
    expect(res.stderr).toContain('+1');
    expect(res.stderr).not.toContain(A2A);
  });

  for (const entry of [
    `--allow-skips=${A2A}`,
    `--allow-skips=${A2A}:many`,
    `--allow-skips=${A2A}:-1`,
  ]) {
    it(`AIM-19.AC3 refuses ${entry} with exit 2, quoting the entry, before any verdict on the report`, () => {
      const res = runChecker([writeReport(populationReport(21)), entry]);
      expect(res.status, res.output).toBe(2);
      expect(res.stderr).toContain(entry.slice('--allow-skips='.length));
      // No counts line, no verdict: the report was never judged.
      expect(res.stdout).toBe('');
      expect(res.stderr).not.toMatch(/^ok:/m);
      expect(res.stderr).not.toContain('tests passed=');
    });
  }

  it('AIM-19.AC3 refuses a bare entry even when the report would otherwise pass, and even before an unreadable report is opened', () => {
    const passing = writeReport(
      buildReport([{ file: 'src/version.test.ts', status: 'passed', n: 3 }]),
    );
    const res = runChecker([passing, `--allow-skips=${A2A}`]);
    expect(res.status, res.output).toBe(2);
    expect(res.stderr).toContain(A2A);

    const missing = runChecker([join(workDir, 'does-not-exist.json'), `--allow-skips=${A2A}:+1`]);
    expect(missing.status, missing.output).toBe(2);
    expect(missing.stderr).toContain(`${A2A}:+1`);
    expect(missing.stderr).not.toContain('unreadable vitest report');
  });
});
