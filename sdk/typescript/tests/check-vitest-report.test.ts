import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { spawnSync } from 'child_process';
import { mkdirSync, mkdtempSync, rmSync, writeFileSync, readFileSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { load } from 'js-yaml';

/**
 * The publish/CI zero-skip refusal: scripts/check-vitest-report.mjs.
 *
 * The vitest JSON reporter's aggregate counters cannot see skipIf-skipped
 * cases: on the 1.3.1 release commit the default reporter printed
 * 1158 passed / 36 skipped while the JSON aggregate said 1194 total,
 * 1194 passed, 0 pending, 0 todo — so the inline
 * numPendingTests/numTodoTests expression the workflows used waved the
 * publish through over 36 tests that never ran. The committed checker
 * walks testResults[].assertionResults[].status instead and only ever
 * trusts the per-assertion statuses.
 */

const CHECKER = join(__dirname, '..', 'scripts', 'check-vitest-report.mjs');

/**
 * The SDK root the fixtures' absolute report names sit under. The checker
 * compares each name made relative to its root (--root, else its cwd)
 * exactly against the --allow-skips entries, so every invocation over a
 * fixture built here passes this root explicitly.
 */
const FIXTURE_ROOT = '/repo/sdk/typescript';

let workDir: string;

beforeAll(() => {
  workDir = mkdtempSync(join(tmpdir(), 'check-vitest-report-'));
});

afterAll(() => {
  rmSync(workDir, { recursive: true, force: true });
});

interface AssertionSpec {
  file: string;
  status: string;
  n: number;
}

/**
 * Build a vitest-JSON-reporter-shaped report. `aggregate` overrides let a
 * fixture lie in its aggregate counters exactly the way the real reporter
 * does (skipped assertions counted as passed, numPendingTests 0).
 */
function buildReport(
  specs: AssertionSpec[],
  aggregate?: Record<string, number>,
): object {
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
  const passed = specs
    .filter((s) => s.status === 'passed')
    .reduce((acc, s) => acc + s.n, 0);
  const failed = specs
    .filter((s) => s.status === 'failed')
    .reduce((acc, s) => acc + s.n, 0);
  return {
    numTotalTestSuites: testResults.length,
    numPassedTestSuites: testResults.length,
    numFailedTestSuites: 0,
    numPendingTestSuites: 0,
    numTotalTests: total,
    numPassedTests: passed,
    numFailedTests: failed,
    numPendingTests: 0,
    numTodoTests: 0,
    startTime: 0,
    success: failed === 0,
    testResults,
    ...aggregate,
  };
}

let fixtureCount = 0;

function writeReport(report: object): string {
  const p = join(workDir, `report-${fixtureCount++}.json`);
  writeFileSync(p, JSON.stringify(report));
  return p;
}

/**
 * Run the checker over a fixture. The fixture names resolve against
 * FIXTURE_ROOT, passed as --root unless the caller supplies its own
 * `root` (an explicit `null` omits --root to exercise the cwd default).
 */
function runChecker(
  args: string[],
  opts: { root?: string | null; cwd?: string } = {},
) {
  const root = opts.root === undefined ? FIXTURE_ROOT : opts.root;
  const rootArgs = root === null ? [] : [`--root=${root}`];
  const res = spawnSync('node', [CHECKER, ...args, ...rootArgs], {
    encoding: 'utf-8',
    cwd: opts.cwd,
  });
  return {
    status: res.status,
    stdout: res.stdout ?? '',
    stderr: res.stderr ?? '',
    output: `${res.stdout ?? ''}${res.stderr ?? ''}`,
  };
}

const INTEGRATION_FILES = [
  'src/a2a/A2AClient.integration.test.ts',
  'src/client/AIMClient.integration.test.ts',
  'src/auth/oauth.integration.test.ts',
];

describe('check-vitest-report walks per-assertion statuses', () => {
  it('AIM-10.AC1 refuses the measured 1.3.1 divergence: aggregate 1194/1194/0 pending while assertionResults carry 36 skipped', () => {
    // The fixture reproduces the shape measured on the 1.3.1 release
    // commit: default reporter 1158 passed / 36 skipped; JSON aggregate
    // 1194 total, 1194 passed, 0 pending, 0 todo. The aggregate-only
    // expression the workflows used exits 0 on this report.
    const report = buildReport(
      [
        { file: 'src/a2a/A2AClient.integration.test.ts', status: 'skipped', n: 21 },
        { file: 'src/client/AIMClient.integration.test.ts', status: 'skipped', n: 13 },
        { file: 'src/auth/oauth.integration.test.ts', status: 'skipped', n: 2 },
        { file: 'src/version.test.ts', status: 'passed', n: 1158 },
      ],
      {
        numTotalTests: 1194,
        numPassedTests: 1194,
        numFailedTests: 0,
        numPendingTests: 0,
        numTodoTests: 0,
      },
    );
    const res = runChecker([writeReport(report)]);
    expect(res.status, res.output).not.toBe(0);
    expect(res.output).toContain('skipped=36');
  });

  it('AIM-10.AC1 prints the per-status counts before the verdict', () => {
    const report = buildReport([
      { file: 'src/a.test.ts', status: 'passed', n: 3 },
      { file: 'src/b.test.ts', status: 'skipped', n: 2 },
    ]);
    const res = runChecker([writeReport(report)]);
    expect(res.status, res.output).not.toBe(0);
    const counts = res.stdout;
    expect(counts).toContain('passed=3');
    expect(counts).toContain('failed=0');
    expect(counts).toContain('skipped=2');
    expect(counts).toContain('pending=0');
    expect(counts).toContain('todo=0');
    // The counts line is on stdout; the refusal verdict follows on stderr.
    expect(res.stderr).toMatch(/refus/i);
  });

  it('AIM-10.AC1 refuses a report carrying a pending assertion', () => {
    const report = buildReport([
      { file: 'src/a.test.ts', status: 'passed', n: 5 },
      { file: 'src/b.test.ts', status: 'pending', n: 1 },
    ]);
    const res = runChecker([writeReport(report)]);
    expect(res.status, res.output).not.toBe(0);
    expect(res.stdout).toContain('pending=1');
  });

  it('AIM-10.AC1 refuses a report carrying a todo assertion among passes', () => {
    const report = buildReport([
      { file: 'src/a.test.ts', status: 'passed', n: 5 },
      { file: 'src/b.test.ts', status: 'todo', n: 1 },
    ]);
    const res = runChecker([writeReport(report)]);
    expect(res.status, res.output).not.toBe(0);
    expect(res.stdout).toContain('todo=1');
  });
});

describe('check-vitest-report fails closed on vacuous shapes', () => {
  it('AIM-10.AC3 refuses a report with numTotalTests 0', () => {
    const report = buildReport([], { numTotalTests: 0 });
    const res = runChecker([writeReport(report)]);
    expect(res.status, res.output).not.toBe(0);
    expect(res.stderr).toMatch(/no test ran/i);
  });

  it('AIM-10.AC3 refuses a report with an empty testResults array even when the aggregate claims tests ran', () => {
    const report = buildReport([], {
      numTotalTests: 1194,
      numPassedTests: 1194,
    });
    const res = runChecker([writeReport(report)]);
    expect(res.status, res.output).not.toBe(0);
    expect(res.stderr).toMatch(/no test ran/i);
  });

  it('AIM-10.AC3 refuses a report whose assertionResults carry only todo', () => {
    const report = buildReport([
      { file: 'src/a.test.ts', status: 'todo', n: 4 },
    ]);
    const res = runChecker([writeReport(report)]);
    expect(res.status, res.output).not.toBe(0);
    expect(res.stdout).toContain('todo=4');
  });

  it('AIM-10.AC3 refuses a missing report file with a distinct message', () => {
    const res = runChecker([join(workDir, 'does-not-exist.json')]);
    expect(res.status).not.toBe(0);
    expect(res.stderr).toContain('unreadable vitest report');
  });

  it('AIM-10.AC3 refuses a malformed report file with a distinct message', () => {
    const p = join(workDir, 'malformed.json');
    writeFileSync(p, '{ this is not json');
    const res = runChecker([p]);
    expect(res.status).not.toBe(0);
    expect(res.stderr).toContain('unreadable vitest report');
  });

  it('AIM-10.AC3 exits 0 on a fully-passed report', () => {
    const report = buildReport([
      { file: 'src/a.test.ts', status: 'passed', n: 7 },
      { file: 'src/b.test.ts', status: 'passed', n: 2 },
    ]);
    const res = runChecker([writeReport(report)]);
    expect(res.status, res.output).toBe(0);
  });
});

describe('check-vitest-report allowlist: exclusions are explicit and bounded', () => {
  const allowFlags = INTEGRATION_FILES.map((f) => `--allow-skips=${f}`);

  it('AIM-10.AC4 accepts skips confined to the allowlisted environment-gated integration files', () => {
    const report = buildReport([
      { file: 'src/a2a/A2AClient.integration.test.ts', status: 'skipped', n: 21 },
      { file: 'src/client/AIMClient.integration.test.ts', status: 'skipped', n: 13 },
      { file: 'src/auth/oauth.integration.test.ts', status: 'skipped', n: 2 },
      { file: 'src/version.test.ts', status: 'passed', n: 1158 },
    ]);
    const res = runChecker([writeReport(report), ...allowFlags]);
    expect(res.status, res.output).toBe(0);
    // The counts still print, so the gate log shows the excluded population.
    expect(res.stdout).toContain('skipped=36');
  });

  it('AIM-10.AC4 refuses a skip outside the allowlist and names the offending file', () => {
    const report = buildReport([
      { file: 'src/a2a/A2AClient.integration.test.ts', status: 'skipped', n: 21 },
      { file: 'src/telemetry/correlation.test.ts', status: 'skipped', n: 1 },
      { file: 'src/version.test.ts', status: 'passed', n: 1000 },
    ]);
    const res = runChecker([writeReport(report), ...allowFlags]);
    expect(res.status, res.output).not.toBe(0);
    expect(res.stderr).toContain('src/telemetry/correlation.test.ts');
  });

  it('AIM-10.AC4 the allowlist does not excuse failures inside allowlisted files', () => {
    const report = buildReport([
      { file: 'src/a2a/A2AClient.integration.test.ts', status: 'failed', n: 1 },
      { file: 'src/version.test.ts', status: 'passed', n: 1000 },
    ]);
    const res = runChecker([writeReport(report), ...allowFlags]);
    expect(res.status, res.output).not.toBe(0);
    expect(res.stdout).toContain('failed=1');
  });

  it('AIM-10.AC4 refuses a report whose only executed outcome is allowlisted skips (nothing ran green)', () => {
    const report = buildReport([
      { file: 'src/a2a/A2AClient.integration.test.ts', status: 'skipped', n: 21 },
    ]);
    const res = runChecker([writeReport(report), ...allowFlags]);
    expect(res.status, res.output).not.toBe(0);
    expect(res.stderr).toMatch(/no test ran/i);
  });
});

describe('check-vitest-report allowlist: an entry matches its SDK-root-relative path exactly', () => {
  const allowFlags = INTEGRATION_FILES.map((f) => `--allow-skips=${f}`);
  const NESTED = 'src/evil/src/a2a/A2AClient.integration.test.ts';
  const LISTED = 'src/a2a/A2AClient.integration.test.ts';

  interface ReportShape {
    testResults: { name: string }[];
  }

  /** The same report with one file's testResults[].name rewritten verbatim. */
  function renamed(report: object, from: string, to: string): object {
    const copy = JSON.parse(JSON.stringify(report)) as ReportShape;
    const hit = copy.testResults.find((r) => r.name === from);
    expect(hit, `fixture carries ${from}`).toBeDefined();
    hit!.name = to;
    return copy;
  }

  it('AIM-18.AC2 refuses a skipped case under a nested same-suffix path and names it; the same report with the file at the listed path exits 0', () => {
    const report = buildReport([
      { file: NESTED, status: 'skipped', n: 1 },
      { file: 'src/version.test.ts', status: 'passed', n: 10 },
    ]);
    const res = runChecker([writeReport(report), ...allowFlags]);
    expect(res.status, res.output).not.toBe(0);
    expect(res.stderr).toMatch(/outside the allowlist/);
    expect(res.stderr).toContain(NESTED);

    const moved = renamed(report, join(FIXTURE_ROOT, NESTED), join(FIXTURE_ROOT, LISTED));
    const ok = runChecker([writeReport(moved), ...allowFlags]);
    expect(ok.status, ok.output).toBe(0);
  });

  it('AIM-18.AC3 refuses an absolute report name outside the root that merely ends with an entry, and tolerates the bare relative entry', () => {
    const outside = `/elsewhere/${LISTED}`;
    const report = renamed(
      buildReport([
        { file: LISTED, status: 'skipped', n: 1 },
        { file: 'src/version.test.ts', status: 'passed', n: 10 },
      ]),
      join(FIXTURE_ROOT, LISTED),
      outside,
    );
    const res = runChecker([writeReport(report), ...allowFlags]);
    expect(res.status, res.output).not.toBe(0);
    expect(res.stderr).toMatch(/outside the allowlist/);
    expect(res.stderr).toContain(outside);

    const relativeName = renamed(
      buildReport([
        { file: LISTED, status: 'skipped', n: 1 },
        { file: 'src/version.test.ts', status: 'passed', n: 10 },
      ]),
      join(FIXTURE_ROOT, LISTED),
      LISTED,
    );
    const ok = runChecker([writeReport(relativeName), ...allowFlags]);
    expect(ok.status, ok.output).toBe(0);
  });

  it('AIM-18.AC1 the root defaults to the checker cwd when no --root is given', () => {
    // Report names under the spawn cwd resolve without --root ...
    const cwd = join(workDir, 'cwd-root');
    mkdirSync(cwd, { recursive: true });
    const report = renamed(
      buildReport([
        { file: LISTED, status: 'skipped', n: 1 },
        { file: 'src/version.test.ts', status: 'passed', n: 10 },
      ]),
      join(FIXTURE_ROOT, LISTED),
      join(cwd, LISTED),
    );
    const ok = runChecker([writeReport(report), ...allowFlags], { root: null, cwd });
    expect(ok.status, ok.output).toBe(0);

    // ... while the same names outside that cwd (the fixture root) do not.
    const elsewhere = buildReport([
      { file: LISTED, status: 'skipped', n: 1 },
      { file: 'src/version.test.ts', status: 'passed', n: 10 },
    ]);
    const res = runChecker([writeReport(elsewhere), ...allowFlags], { root: null, cwd });
    expect(res.status, res.output).not.toBe(0);
    expect(res.stderr).toContain(join(FIXTURE_ROOT, LISTED));
  });

  it('AIM-18.AC1 normalises backslashes and a leading ./ on both sides before the exact comparison', () => {
    const report = renamed(
      buildReport([
        { file: LISTED, status: 'skipped', n: 1 },
        { file: 'src/version.test.ts', status: 'passed', n: 10 },
      ]),
      join(FIXTURE_ROOT, LISTED),
      join(FIXTURE_ROOT, LISTED).replace(/\//g, '\\'),
    );
    const res = runChecker([writeReport(report), `--allow-skips=./${LISTED}`]);
    expect(res.status, res.output).toBe(0);
  });

  it('AIM-18.AC1 a skip at an exactly-listed path is tolerated and the ok and counts lines keep their text', () => {
    const report = buildReport([
      { file: 'src/a2a/A2AClient.integration.test.ts', status: 'skipped', n: 21 },
      { file: 'src/client/AIMClient.integration.test.ts', status: 'skipped', n: 13 },
      { file: 'src/auth/oauth.integration.test.ts', status: 'skipped', n: 2 },
      { file: 'src/version.test.ts', status: 'passed', n: 1158 },
    ]);
    const res = runChecker([writeReport(report), ...allowFlags]);
    expect(res.status, res.output).toBe(0);
    expect(res.stdout).toContain(
      'tests passed=1158 failed=0 skipped=36 pending=0 todo=0',
    );
    expect(res.stdout).toContain(
      'ok: 1158 passed; 36 skipped, all inside the allowlisted environment-gated files ' +
        `(${INTEGRATION_FILES.join(', ')})`,
    );
  });

  it('AIM-18.AC1 the matcher carries no suffix arm', () => {
    const source = readFileSync(CHECKER, 'utf-8');
    expect(source).not.toMatch(/endsWith\(/);
  });
});

describe('both workflow gates invoke the committed checker', () => {
  interface Step {
    name?: string;
    run?: string;
    env?: Record<string, unknown>;
  }
  interface Wf {
    jobs: Record<string, { steps?: Step[] }>;
  }

  const wfDir = join(__dirname, '..', '..', '..', '.github', 'workflows');
  const releaseRaw = readFileSync(join(wfDir, 'release.yml'), 'utf-8');
  const ciRaw = readFileSync(join(wfDir, 'ci.yml'), 'utf-8');

  function gateStep(raw: string): Step {
    const wf = load(raw) as Wf;
    const steps = Object.values(wf.jobs).flatMap((j) => j.steps ?? []);
    const candidates = steps.filter(
      (s) => typeof s.run === 'string' && s.run.includes('vitest run --reporter'),
    );
    expect(candidates.length, 'exactly one gate step per workflow').toBe(1);
    return candidates[0];
  }

  for (const [wfName, raw] of [
    ['release.yml', releaseRaw],
    ['ci.yml', ciRaw],
  ] as const) {
    it(`AIM-10.AC2 the ${wfName} gate step runs the committed checker on its JSON report under CI=true, allowlisting only the three integration files`, () => {
      const step = gateStep(raw);
      const run = step.run ?? '';
      expect(run).toContain('--reporter=json');
      expect(run).toContain('scripts/check-vitest-report.mjs');
      expect(step.env?.CI, 'the gate step must set env CI: "true"').toBe('true');
      for (const f of INTEGRATION_FILES) {
        expect(run).toContain(`--allow-skips=${f}`);
      }
      // Exactly the three named exclusions — an allowlist that grows must
      // change this committed assertion.
      expect(run.match(/--allow-skips=/g)?.length).toBe(3);
    });

    it(`AIM-10.AC2 no aggregate-only numPendingTests/numTodoTests refusal expression remains in any ${wfName} step`, () => {
      const wf = load(raw) as Wf;
      for (const [jobId, job] of Object.entries(wf.jobs)) {
        for (const step of job.steps ?? []) {
          const run = step.run ?? '';
          const label = `${jobId} > ${step.name ?? '(unnamed)'}`;
          expect(run, label).not.toContain('numPendingTests');
          expect(run, label).not.toContain('numTodoTests');
          expect(run, label).not.toMatch(
            /!\(total > 0\) \|\| failed > 0 \|\| skipped > 0/,
          );
        }
      }
    });
  }
});
