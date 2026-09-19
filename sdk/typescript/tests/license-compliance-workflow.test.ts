import { describe, it, expect } from 'vitest';
import { spawnSync } from 'child_process';
import {
  chmodSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  realpathSync,
  rmSync,
  statSync,
  writeFileSync,
} from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { load } from 'js-yaml';

/**
 * The Go license gate of .github/workflows/security.yml must produce a VERDICT,
 * and the verdict must be the license checker's exit status.
 *
 * Before this file, the "Check Go licenses" step was:
 *
 *     go install github.com/google/go-licenses@latest
 *     go-licenses check ./... --disallowed_types=restricted,reciprocal 2>&1 \
 *       | tee license-report.txt
 *     if grep -iE 'GPL|AGPL|SSPL' license-report.txt | grep -iv 'LGPL'; then ...
 *
 * so the step's exit status was `tee`'s (the workflow sets no pipefail) and the
 * only surviving verdict was a text grep. Four separate defects rode on that:
 *
 *   AC1  the checker's exit status is now the step's, and the class list is the
 *        full set — an explicit --disallowed_types REPLACES the tool's default
 *        of forbidden,unknown, so `restricted,reciprocal` alone would have been
 *        a WEAKER gate than the grep it replaced (AGPL is class forbidden, an
 *        unidentifiable license is class unknown);
 *   AC2  the tool is pinned by version AND by h1 sum, and the sum is asserted
 *        on the INSTALLED binary before the checker is trusted to run;
 *   AC3  every failure is classified, once, in a fixed order, so a module-proxy
 *        stream error can never again report as a license finding;
 *   AC4  the Go step and the npm step are gated on their own inputs, so a
 *        frontend-lockfile-only pull request no longer runs the Go check;
 *   AC5  each of those properties refuses a planted regression;
 *   AC6  the workflow's permissions, action set and sibling filters are held.
 *
 * The workflow assertions are functions over a parsed document and the script
 * assertions are functions over a recorded invocation log, never constants
 * evaluated at module load, so AC5 can apply every one of them to a mutated
 * in-memory copy and demand a throw.
 *
 * This file runs inside the `sdk-tests` job of ci.yml, which CI Gate needs, so
 * the guard is load-bearing on every pull request.
 */

// ── The pin, as security.yml carries it ────────────────────────────────

const GO_LICENSES_MODULE = 'github.com/google/go-licenses/v2';
const GO_LICENSES_VERSION = 'v2.0.1';
const GO_LICENSES_SUM = 'h1:ti+9bi5o7DKbeeg5eBb/uZTgsaPNoJaLCh93cRcXsW8=';

/** The class set the checker must be given, as a set: order is not meaningful. */
const DISALLOWED_TYPES = ['forbidden', 'reciprocal', 'restricted', 'unknown'];

/**
 * Anything that would weaken or disable Go module verification. None of these
 * may appear in security.yml, and none may reach the toolchain's environment
 * from the script — the checksum database is what makes the h1 sum mean
 * something.
 */
const GO_ENV_ESCAPE_HATCHES = [
  'GOFLAGS',
  'GOSUMDB',
  'GONOSUMDB',
  'GONOSUMCHECK',
  'GOPRIVATE',
  'GOINSECURE',
];

/** The six phrases the script is allowed to conclude with. */
const PHRASE = {
  pass: 'no disallowed license found',
  finding: 'disallowed license found',
  notBuilt: 'tool could not be built',
  badSum: 'tool checksum mismatch',
  cannotRun: 'checker could not run',
  noVerdict: 'checker failed before a verdict',
};

const REPO_ROOT = realpathSync(join(__dirname, '..', '..', '..'));
const WORKFLOW_PATH = join(REPO_ROOT, '.github', 'workflows', 'security.yml');
const SCRIPT_PATH = join(REPO_ROOT, 'scripts', 'go-licenses-check.sh');

/** The verbatim proxy failure that failed #493's job. */
const STREAM_ERROR =
  'stream error: stream ID 1489; INTERNAL_ERROR; received from peer';
/** The planted reciprocal finding, in v2.0.1's own wording. */
const PLANTED_FINDING =
  "License 'MPL-2.0' of not allowed license type 'Reciprocal' found for library 'example.com/planted'.";
/** The same finding for a library whose name also matches the network pattern. */
const PLANTED_FINDING_TIMEOUT =
  "License 'MPL-2.0' of not allowed license type 'Reciprocal' found for library 'example.com/timeout/planted'.";

// ── Sources, read inside functions ────────────────────────────────────

function workflowSource(): string {
  expect(
    existsSync(WORKFLOW_PATH),
    `${WORKFLOW_PATH} must exist`,
  ).toBe(true);
  return readFileSync(WORKFLOW_PATH, 'utf-8');
}

function scriptSource(): string {
  expect(
    existsSync(SCRIPT_PATH),
    'scripts/go-licenses-check.sh must exist: it is the step\'s logic',
  ).toBe(true);
  return readFileSync(SCRIPT_PATH, 'utf-8');
}

// ── The parsed workflow document ──────────────────────────────────────

interface WorkflowStep {
  name?: string;
  id?: string;
  uses?: string;
  run?: string;
  if?: unknown;
  env?: Record<string, unknown>;
  with?: Record<string, unknown>;
  'working-directory'?: string;
  'continue-on-error'?: unknown;
}

interface WorkflowJob {
  name?: string;
  needs?: unknown;
  if?: unknown;
  permissions?: unknown;
  env?: Record<string, unknown>;
  outputs?: Record<string, string>;
  steps?: WorkflowStep[];
}

interface Workflow {
  permissions?: Record<string, string>;
  jobs: Record<string, WorkflowJob>;
}

function parseWorkflow(source: string): Workflow {
  const doc = load(source) as Workflow;
  expect(doc?.jobs, 'security.yml must parse to a document with jobs').toBeDefined();
  return doc;
}

function job(source: string, id: string): WorkflowJob {
  const found = parseWorkflow(source).jobs[id];
  expect(found, `security.yml must define the ${id} job`).toBeDefined();
  return found;
}

function stepNamed(source: string, jobId: string, name: string): WorkflowStep {
  const step = (job(source, jobId).steps ?? []).find((s) => s.name === name);
  expect(step, `the ${jobId} job must have a step named "${name}"`).toBeDefined();
  return step as WorkflowStep;
}

/**
 * The dorny/paths-filter `filters:` input is a block scalar of YAML, so the
 * filters are the parse of that text — not a grep over the workflow.
 */
function pathsFilters(source: string): Record<string, string[]> {
  const changes = job(source, 'changes');
  const filterStep = (changes.steps ?? []).find((s) => s.id === 'filter');
  expect(
    filterStep,
    'the changes job must carry the dorny/paths-filter step with id: filter',
  ).toBeDefined();
  const text = (filterStep?.with ?? {}).filters;
  expect(
    typeof text,
    'the paths-filter step must carry a `filters:` block scalar',
  ).toBe('string');
  const parsed = load(text as string) as Record<string, string[]>;
  expect(parsed, 'the `filters:` block must itself parse as YAML').toBeTruthy();
  return parsed;
}

function filterPatterns(
  source: string,
  name: string,
): string[] {
  const patterns = pathsFilters(source)[name];
  expect(
    Array.isArray(patterns),
    `the paths-filter must define a \`${name}\` filter as a list of patterns`,
  ).toBe(true);
  return patterns;
}

// ── The recorded invocation log ───────────────────────────────────────

/** Field separator the stubs join argv and environment entries with (US). */
const SEP = '\u001f';

interface Invocation {
  /** argv[0] as the kernel resolved it — which binary actually ran. */
  argv0: string;
  /** the working directory the invocation was made from. */
  cwd: string;
  argv: string[];
  /** `NAME=VALUE` for every Go escape-hatch variable visible to the stub. */
  env: string[];
}

function readLog(path: string): Invocation[] {
  return readFileSync(path, 'utf-8')
    .split('\n')
    .filter((line) => line.length > 0)
    .map((line) => {
      const [argv0 = '', cwd = '', argvField = '', envField = ''] =
        line.split('\t');
      // Each stub emits a trailing separator after every element.
      const fields = (f: string) => (f.length > 0 ? f.split(SEP).slice(0, -1) : []);
      return { argv0, cwd, argv: fields(argvField), env: fields(envField) };
    });
}

// ── The stubs ─────────────────────────────────────────────────────────

/**
 * Shared prelude. `qgf_log` records argv0, the cwd, the argv and any Go
 * escape-hatch variable in the process environment; `qgf_replay` replays a
 * per-attempt outcome planted in $STUB_DIR, so a retry can be made to fail
 * once and then succeed.
 */
const STUB_PRELUDE = String.raw`
qgf_log() {
  local argv_joined="" env_joined=""
  if [ "$#" -gt 0 ]; then argv_joined="$(printf '%s\037' "$@")"; fi
  env_joined="$(env | grep -E '^(GOFLAGS|GOSUMDB|GONOSUMDB|GONOSUMCHECK|GOPRIVATE|GOINSECURE)=' | tr '\n' '\037')"
  printf '%s\t%s\t%s\t%s\n' "$0" "$PWD" "$argv_joined" "$env_joined" >> "$STUB_LOG"
}

qgf_replay() {
  local action="$1" counter n out rcf rc
  counter="$STUB_DIR/count.$action"
  n=1
  if [ -f "$counter" ]; then n=$(( $(cat "$counter") + 1 )); fi
  printf '%s' "$n" > "$counter"
  out="$STUB_DIR/$action.$n.out"
  if [ ! -f "$out" ]; then out="$STUB_DIR/$action.default.out"; fi
  rcf="$STUB_DIR/$action.$n.rc"
  if [ ! -f "$rcf" ]; then rcf="$STUB_DIR/$action.default.rc"; fi
  if [ -f "$out" ]; then cat "$out"; fi
  rc=0
  if [ -f "$rcf" ]; then rc="$(cat "$rcf")"; fi
  return "$rc"
}
`;

/**
 * Stub `go`. Handles the two subcommands the script may use: `install`, which
 * on a planned success materialises the checker in $GOBIN exactly as a real
 * install would, and `version -m`, which reports the build-info `mod` line.
 */
const STUB_GO = `#!/usr/bin/env bash
# Test stub for \`go\`. Not a toolchain.
${STUB_PRELUDE}
qgf_log "$@"

case "$1" in
  install)
    qgf_replay install
    rc="$?"
    if [ "$rc" -eq 0 ]; then
      cp "$STUB_DIR/go-licenses.stub" "$GOBIN/go-licenses"
      chmod +x "$GOBIN/go-licenses"
    fi
    exit "$rc"
    ;;
  version)
    printf '%s: go1.25.0\\n' "$3"
    printf '\\tpath\\t%s\\n' "$STUB_MOD_PATH"
    if [ "$STUB_MOD_EMIT" = "1" ]; then
      printf '\\tmod\\t%s\\t%s\\t%s\\n' "$STUB_MOD_PATH" "$STUB_MOD_VERSION" "$STUB_MOD_SUM"
    fi
    printf '\\tbuild\\t-buildmode=exe\\n'
    exit 0
    ;;
esac

printf 'stub go: unexpected invocation: %s\\n' "$*" >&2
exit 97
`;

/** Stub `go-licenses`. Records the invocation and replays a planned verdict. */
const STUB_GO_LICENSES = `#!/usr/bin/env bash
# Test stub for the go-licenses checker.
${STUB_PRELUDE}
qgf_log "$@"
qgf_replay "$1"
exit "$?"
`;

// ── Driving the script ────────────────────────────────────────────────

interface Plan {
  rc: number;
  out: string;
}

interface SandboxOptions {
  /** per-attempt `go install` outcomes; the last entry repeats. */
  install?: Plan[];
  /** per-attempt checker outcomes; the last entry repeats. */
  check?: Plan[];
  /** the h1 sum the stub's `go version -m` reports. */
  modSum?: string;
  /** false to omit the `mod` line from `go version -m` entirely. */
  emitModLine?: boolean;
  /** run a mutated copy of the script under a mirror repo root instead. */
  scriptSource?: string;
  /** false to leave GITHUB_STEP_SUMMARY unset. */
  stepSummary?: boolean;
}

interface RunResult {
  status: number;
  stdout: string;
  stderr: string;
  /** contents of the GITHUB_STEP_SUMMARY file ('' when it was left unset). */
  summary: string;
  log: Invocation[];
  installs: Invocation[];
  checks: Invocation[];
  /** the root the script resolved from its own location. */
  repoRoot: string;
  gobin: string;
}

function writeExecutable(path: string, contents: string): void {
  writeFileSync(path, contents);
  chmodSync(path, 0o755);
}

function writePlan(dir: string, action: string, plans: Plan[]): void {
  plans.forEach((plan, i) => {
    writeFileSync(join(dir, `${action}.${i + 1}.rc`), String(plan.rc));
    writeFileSync(join(dir, `${action}.${i + 1}.out`), plan.out);
  });
  // A further attempt repeats the last plan; the invocation count is what
  // catches an unplanned retry, not a missing plan file.
  const last = plans[plans.length - 1];
  writeFileSync(join(dir, `${action}.default.rc`), String(last.rc));
  writeFileSync(join(dir, `${action}.default.out`), last.out);
}

/**
 * A parent directory the stubs can actually be EXECUTED from.
 *
 * os.tmpdir() is the right place, and is what a GitHub runner gives us, but a
 * hardened container may mount /tmp noexec — and a stub `go` that cannot be
 * executed is silently SKIPPED by PATH resolution, so the real toolchain
 * answers and every cell quietly proves nothing. The capability is therefore
 * probed rather than assumed, with node_modules (never committed, always
 * present after the job's `npm ci`) as the fallback, and a loud throw if
 * neither works.
 */
let cachedSandboxParent: string | undefined;
function sandboxParent(): string {
  if (cachedSandboxParent !== undefined) return cachedSandboxParent;
  const candidates = [
    tmpdir(),
    join(REPO_ROOT, 'sdk', 'typescript', 'node_modules', '.cache'),
  ];
  for (const candidate of candidates) {
    let probeDir: string | undefined;
    try {
      mkdirSync(candidate, { recursive: true });
      probeDir = mkdtempSync(join(candidate, 'qgf-251-exec-probe-'));
      const probe = join(probeDir, 'probe');
      writeExecutable(probe, '#!/usr/bin/env bash\nexit 0\n');
      if (spawnSync(probe, [], { encoding: 'utf-8' }).status === 0) {
        cachedSandboxParent = candidate;
        return candidate;
      }
    } catch {
      // fall through to the next candidate
    } finally {
      if (probeDir) rmSync(probeDir, { recursive: true, force: true });
    }
  }
  throw new Error(
    'found no directory that permits executing a file; the stub `go` and ' +
      '`go-licenses` must be executable or the real toolchain answers instead',
  );
}

function runCheckScript(opts: SandboxOptions = {}): RunResult {
  const root = mkdtempSync(join(sandboxParent(), 'qgf-251-license-'));
  try {
    const stubDir = join(root, 'stub');
    const binDir = join(stubDir, 'bin');
    const gobin = join(root, 'gobin');
    mkdirSync(binDir, { recursive: true });
    mkdirSync(gobin, { recursive: true });

    writeExecutable(join(binDir, 'go'), STUB_GO);
    // A decoy first on PATH. If the script ever ran a `go-licenses` off PATH
    // instead of the sum-verified binary in GOBIN, this is what would answer
    // and the recorded argv0 would say so.
    writeExecutable(join(binDir, 'go-licenses'), STUB_GO_LICENSES);
    // What the stub `go install` materialises into GOBIN.
    writeExecutable(join(stubDir, 'go-licenses.stub'), STUB_GO_LICENSES);

    writePlan(stubDir, 'install', opts.install ?? [{ rc: 0, out: '' }]);
    writePlan(stubDir, 'check', opts.check ?? [{ rc: 0, out: '' }]);

    const logPath = join(root, 'invocations.log');
    writeFileSync(logPath, '');
    const summaryPath = join(root, 'step-summary.md');
    writeFileSync(summaryPath, '');

    let repoRoot = REPO_ROOT;
    let scriptPath = SCRIPT_PATH;
    if (opts.scriptSource !== undefined) {
      // A mutated copy gets its own mirror root so that the script's
      // location-relative resolution still finds an apps/backend to enter.
      repoRoot = join(root, 'mirror');
      mkdirSync(join(repoRoot, 'scripts'), { recursive: true });
      mkdirSync(join(repoRoot, 'apps', 'backend'), { recursive: true });
      scriptPath = join(repoRoot, 'scripts', 'go-licenses-check.sh');
      writeExecutable(scriptPath, opts.scriptSource);
    }
    expect(
      existsSync(scriptPath),
      `${scriptPath} must exist: the step runs it and nothing else`,
    ).toBe(true);

    const env: NodeJS.ProcessEnv = { ...process.env };
    // Cleared so that a hatch seen by a stub can only have come from the
    // script, never from whatever is ambient on the runner.
    for (const name of GO_ENV_ESCAPE_HATCHES) delete env[name];
    // The sdk-tests job runs with GITHUB_STEP_SUMMARY already set; each cell
    // decides for itself whether the script gets one.
    delete env.GITHUB_STEP_SUMMARY;
    env.PATH = `${binDir}:${process.env.PATH ?? ''}`;
    env.GOBIN = gobin;
    env.GO_LICENSES_VERSION = GO_LICENSES_VERSION;
    env.GO_LICENSES_SUM = GO_LICENSES_SUM;
    env.GO_LICENSES_RETRY_DELAY = '0';
    env.STUB_LOG = logPath;
    env.STUB_DIR = stubDir;
    env.STUB_MOD_PATH = GO_LICENSES_MODULE;
    env.STUB_MOD_VERSION = GO_LICENSES_VERSION;
    env.STUB_MOD_SUM = opts.modSum ?? GO_LICENSES_SUM;
    env.STUB_MOD_EMIT = (opts.emitModLine ?? true) ? '1' : '0';
    if (opts.stepSummary ?? true) env.GITHUB_STEP_SUMMARY = summaryPath;

    // Run from the sandbox, not from the repo: the script must resolve its
    // own location, exactly as scripts/lint-docker-paths-filter.sh does.
    const res = spawnSync(scriptPath, [], { cwd: root, env, encoding: 'utf-8' });
    expect(
      res.error,
      `spawning ${scriptPath} must not error (is it mode 100755?)`,
    ).toBeUndefined();

    const log = readLog(logPath);
    // Every cell reaches `go install` at least once, so an empty log means the
    // stubs never ran and whatever answered was not under this test's control.
    expect(
      log.length,
      `the stub toolchain recorded nothing, so the real one answered:\n${res.stdout}\n${res.stderr}`,
    ).toBeGreaterThan(0);

    return {
      status: res.status as number,
      stdout: res.stdout ?? '',
      stderr: res.stderr ?? '',
      summary: readFileSync(summaryPath, 'utf-8'),
      log,
      installs: log.filter((i) => i.argv[0] === 'install'),
      checks: log.filter((i) => i.argv[0] === 'check'),
      repoRoot,
      gobin,
    };
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
}

// ── Assertions over the parsed workflow ───────────────────────────────

function assertGoStepRunsOnlyTheScript(source: string): void {
  const step = stepNamed(source, 'license-check', 'Check Go licenses');
  const run = (step.run ?? '').trim();
  expect(
    run.split('\n'),
    'the Go step must run exactly one line — the script IS the step logic',
  ).toHaveLength(1);
  expect(
    run,
    'the Go step must end in scripts/go-licenses-check.sh',
  ).toMatch(/scripts\/go-licenses-check\.sh$/);
  expect(
    run,
    'no pipe: a pipe discards the checker exit status (the workflow sets no pipefail)',
  ).not.toContain('|');
  expect(run, 'no tee').not.toContain('tee');
  expect(
    run,
    'no grep: text classifies a failure, it never decides the verdict',
  ).not.toContain('grep');
  expect(
    step['continue-on-error'],
    'the Go step must not set continue-on-error: its exit status IS the verdict',
  ).toBeUndefined();
}

function assertPinEnvLiterals(source: string): void {
  const jobEnv = job(source, 'license-check').env ?? {};
  const stepEnv = stepNamed(source, 'license-check', 'Check Go licenses').env ?? {};
  const env = { ...jobEnv, ...stepEnv };
  expect(
    env.GO_LICENSES_VERSION,
    'the license-check job or its Go step must set GO_LICENSES_VERSION',
  ).toBe(GO_LICENSES_VERSION);
  expect(
    env.GO_LICENSES_SUM,
    'the license-check job or its Go step must set GO_LICENSES_SUM',
  ).toBe(GO_LICENSES_SUM);
}

/**
 * The pin has to be datable from the file, otherwise nobody can tell a stale
 * pin from a deliberate one. js-yaml drops comments, so the comment is read
 * from the source lines beside the two keys the parse located.
 */
function assertPinCommentDatesThePin(source: string): void {
  assertPinEnvLiterals(source);
  const lines = source.split('\n');
  const versionIdx = lines.findIndex((l) => l.includes('GO_LICENSES_VERSION:'));
  const sumIdx = lines.findIndex((l) => l.includes('GO_LICENSES_SUM:'));
  expect(versionIdx, 'GO_LICENSES_VERSION must appear in security.yml').toBeGreaterThanOrEqual(0);
  expect(sumIdx, 'GO_LICENSES_SUM must appear in security.yml').toBeGreaterThanOrEqual(0);

  let top = Math.min(versionIdx, sumIdx);
  while (top > 0 && lines[top - 1].trim().startsWith('#')) top -= 1;
  const beside = lines.slice(top, Math.max(versionIdx, sumIdx) + 2).join('\n');

  for (const token of ['2025-09-08', '2026-09-17', 'sum.golang.org']) {
    expect(
      beside,
      `a comment beside the pin must name ${token}`,
    ).toContain(token);
  }
}

function assertNoGoEscapeHatchesInWorkflow(source: string): void {
  for (const name of GO_ENV_ESCAPE_HATCHES) {
    expect(
      source,
      `security.yml must not carry ${name}: the module checksum database stays on`,
    ).not.toContain(name);
  }
}

function assertGoDepsFilter(source: string): void {
  const patterns = filterPatterns(source, 'go_deps');
  for (const pattern of patterns) {
    const allowed =
      pattern.startsWith('apps/backend/') ||
      pattern === '.github/workflows/security.yml' ||
      pattern === 'scripts/go-licenses-check.sh';
    expect(
      allowed,
      `go_deps pattern '${pattern}' is neither under apps/backend/ nor one of the two step inputs`,
    ).toBe(true);
  }
  for (const required of [
    'apps/backend/go.mod',
    'apps/backend/go.sum',
    'apps/backend/**/*.go',
    '.github/workflows/security.yml',
    // The script IS the step's logic, so a pull request that edits it must
    // run the step UNDER the edited script.
    'scripts/go-licenses-check.sh',
  ]) {
    expect(patterns, `go_deps must list ${required}`).toContain(required);
  }
}

function assertNpmDepsFilter(source: string): void {
  const patterns = filterPatterns(source, 'npm_deps');
  for (const required of [
    'apps/web/package.json',
    'apps/web/package-lock.json',
    '.github/workflows/security.yml',
  ]) {
    expect(patterns, `npm_deps must list ${required}`).toContain(required);
  }
}

/**
 * The witness for the unit's own defect: a pull request whose only changed
 * file is apps/web/package-lock.json must set go_deps false.
 */
function assertLockfileOnlyPrSkipsTheGoStep(source: string): void {
  const patterns = filterPatterns(source, 'go_deps');
  for (const pattern of patterns) {
    expect(
      pattern.startsWith('apps/web/'),
      `go_deps pattern '${pattern}' is under apps/web/, so a frontend lockfile change would run the Go check`,
    ).toBe(false);
  }
  expect(
    patterns,
    'apps/backend/go.sum must still be a go_deps pattern',
  ).toContain('apps/backend/go.sum');
}

function assertDepsFilterIsGone(source: string): void {
  expect(
    pathsFilters(source).deps,
    'the single `deps` filter must be gone: it gated the Go check on frontend manifests',
  ).toBeUndefined();
  expect(
    (job(source, 'changes').outputs ?? {}).deps,
    'the changes job must no longer export a `deps` output',
  ).toBeUndefined();
  expect(
    source,
    'nothing in security.yml may read outputs.deps any more',
  ).not.toContain('outputs.deps');
}

function assertPerStepGating(source: string): void {
  expect(
    stepNamed(source, 'license-check', 'Check Go licenses').if,
    'the Go step must be gated on go_deps',
  ).toBe(
    "github.event_name == 'schedule' || needs.changes.outputs.go_deps == 'true'",
  );
  expect(
    stepNamed(source, 'license-check', 'Check npm licenses').if,
    'the npm step must be gated on npm_deps',
  ).toBe(
    "github.event_name == 'schedule' || needs.changes.outputs.npm_deps == 'true'",
  );

  const jobIf = String(job(source, 'license-check').if ?? '');
  for (const admits of [
    "github.event_name == 'schedule'",
    "needs.changes.outputs.go_deps == 'true'",
    "needs.changes.outputs.npm_deps == 'true'",
  ]) {
    expect(
      jobIf,
      `the license-check job's if: must admit ${admits}`,
    ).toContain(admits);
  }
}

function assertNpmStepUnchanged(source: string): void {
  const step = stepNamed(source, 'license-check', 'Check npm licenses');
  expect(step['working-directory']).toBe('apps/web');
  expect(
    (step.run ?? '')
      .trim()
      .split('\n')
      .map((l) => l.trim()),
  ).toEqual([
    'npm ci',
    'npx license-checker --failOn "GPL-2.0;GPL-3.0;AGPL-1.0;AGPL-3.0;SSPL-1.0" --summary',
  ]);
}

function assertTopLevelPermissions(source: string): void {
  expect(parseWorkflow(source).permissions).toEqual({
    'contents': 'read',
    'security-events': 'write',
  });
  expect(
    job(source, 'license-check').permissions,
    'the license-check job must not widen permissions of its own',
  ).toBeUndefined();
}

/** Either the base tag, or a 40-hex sha carried beside a `# v<n>` comment. */
function assertActionRef(source: string, uses: string, baseTag: string): void {
  const [, ref = ''] = uses.split('@');
  if (ref === baseTag) return;
  expect(
    ref,
    `${uses} must be pinned either to ${baseTag} or to a 40-hex commit sha`,
  ).toMatch(/^[0-9a-f]{40}$/);
  const line = source.split('\n').find((l) => l.includes(`uses: ${uses}`));
  expect(
    line,
    `a sha-pinned ${uses} must be followed by a # v<n> comment naming the tag`,
  ).toMatch(/#\s*v\d+/);
}

function assertLicenseCheckActionSet(source: string): void {
  const steps = (job(source, 'license-check').steps ?? []).filter(
    (s) => typeof s.uses === 'string',
  );
  expect(
    steps.map((s) => (s.uses as string).split('@')[0]),
    'the license-check job must use exactly checkout, setup-go and setup-node, in that order',
  ).toEqual(['actions/checkout', 'actions/setup-go', 'actions/setup-node']);

  assertActionRef(source, steps[0].uses as string, 'v4');
  assertActionRef(source, steps[1].uses as string, 'v5');
  assertActionRef(source, steps[2].uses as string, 'v4');

  expect(steps[1].with).toEqual({
    'go-version': '1.25',
    'cache-dependency-path': 'apps/backend/go.sum',
  });
  expect(steps[2].with).toEqual({ 'node-version': '20' });
}

function assertSiblingFiltersHeld(source: string): void {
  const filters = pathsFilters(source);
  expect(
    filters.docker,
    'the five docker: patterns scripts/lint-docker-paths-filter.sh reads must stay',
  ).toEqual([
    'infrastructure/docker/**',
    'apps/backend/**',
    'apps/web/**',
    'sdk/**',
    '.github/workflows/security.yml',
  ]);
  expect(filters.backend).toEqual(['apps/backend/**']);
  expect(filters.frontend).toEqual(['apps/web/**']);
  expect(filters.crypto).toEqual([
    'apps/backend/internal/crypto/**',
    'apps/backend/internal/infrastructure/auth/**',
    'scripts/crypto-audit.sh',
  ]);
}

// ── Assertions over the invocation log ────────────────────────────────

function assertDisallowedTypesArgv(log: Invocation[]): void {
  const checks = log.filter((i) => i.argv[0] === 'check');
  expect(checks.length, 'the checker must have been invoked').toBeGreaterThan(0);
  for (const inv of checks) {
    expect(inv.argv[0]).toBe('check');
    expect(inv.argv, 'the checker must be pointed at ./...').toContain('./...');
    const flags = inv.argv.filter((a) => a.startsWith('--disallowed_types'));
    expect(
      flags,
      'exactly one --disallowed_types argument: an explicit value REPLACES the tool default',
    ).toHaveLength(1);
    const value = flags[0].split('=')[1] ?? '';
    expect(
      value.split(',').map((c) => c.trim()).sort(),
      'the class set must be exactly forbidden, unknown, restricted, reciprocal',
    ).toEqual(DISALLOWED_TYPES);
  }
}

function assertCheckerRanInAppsBackend(log: Invocation[], repoRoot: string): void {
  const checks = log.filter((i) => i.argv[0] === 'check');
  expect(checks.length, 'the checker must have been invoked').toBeGreaterThan(0);
  for (const inv of checks) {
    expect(
      realpathSync(inv.cwd),
      'the checker must run with apps/backend as its working directory',
    ).toBe(realpathSync(join(repoRoot, 'apps', 'backend')));
  }
}

function assertInstallArgv(log: Invocation[]): void {
  const installs = log.filter((i) => i.argv[0] === 'install');
  expect(installs.length, '`go install` must have been invoked').toBeGreaterThan(0);
  for (const inv of installs) {
    expect(
      inv.argv,
      'the install must name the pinned /v2 module version, never @latest and never the v1 path',
    ).toEqual(['install', `${GO_LICENSES_MODULE}@${GO_LICENSES_VERSION}`]);
  }
}

function assertCheckerRanFromGobin(log: Invocation[], gobin: string): void {
  const checks = log.filter((i) => i.argv[0] === 'check');
  expect(checks.length, 'the checker must have been invoked').toBeGreaterThan(0);
  for (const inv of checks) {
    expect(
      inv.argv0,
      'the checker must be the sum-verified binary in GOBIN, not one found on PATH',
    ).toBe(join(gobin, 'go-licenses'));
  }
}

function assertNoGoEscapeHatchesReachTheToolchain(log: Invocation[]): void {
  expect(log.length, 'the stubs must have recorded something').toBeGreaterThan(0);
  for (const inv of log) {
    expect(
      inv.env,
      `the script must not set or export a module-verification escape hatch (saw ${inv.env.join(' ')})`,
    ).toEqual([]);
    for (const name of GO_ENV_ESCAPE_HATCHES) {
      expect(
        inv.argv.join(' '),
        `the script must not pass ${name} through argv`,
      ).not.toContain(name);
    }
  }
}

// ── Planted regressions (AC5) ─────────────────────────────────────────

function lineIndex(lines: string[], match: (l: string) => boolean, what: string): number {
  const i = lines.findIndex(match);
  expect(i, `expected to find ${what} in security.yml`).toBeGreaterThanOrEqual(0);
  return i;
}

function indentOf(line: string): string {
  return ' '.repeat(line.length - line.trimStart().length);
}

/** (1) the base step body, tee pipe and text grep and all. */
function mutateRestoreBaseGoRun(source: string): string {
  const lines = source.split('\n');
  const i = lineIndex(
    lines,
    (l) => l.trim().endsWith('scripts/go-licenses-check.sh') && l.includes('run:'),
    'the Go step run: line',
  );
  const pad = indentOf(lines[i]);
  lines.splice(
    i,
    1,
    `${pad}run: |`,
    `${pad}  go install github.com/google/go-licenses@latest`,
    `${pad}  go-licenses check ./... --disallowed_types=restricted,reciprocal 2>&1 | tee license-report.txt`,
    `${pad}  # Fail on GPL/AGPL/SSPL`,
    `${pad}  if grep -iE 'GPL|AGPL|SSPL' license-report.txt | grep -iv 'LGPL'; then`,
    `${pad}    echo "::error::Copyleft license detected in Go dependencies"`,
    `${pad}    exit 1`,
    `${pad}  fi`,
  );
  return lines.join('\n');
}

/** (2) an escape hatch on the step that carries the verdict. */
function mutateAddContinueOnError(source: string): string {
  const lines = source.split('\n');
  const i = lineIndex(
    lines,
    (l) => l.trim() === '- name: Check Go licenses',
    'the Check Go licenses step header',
  );
  lines.splice(i + 1, 0, `${indentOf(lines[i])}  continue-on-error: true`);
  return lines.join('\n');
}

/** (3) the version kept, the sum dropped — a pin with no evidence. */
function mutateRemovePinSum(source: string): string {
  return source
    .split('\n')
    .filter((l) => !l.includes('GO_LICENSES_SUM:'))
    .join('\n');
}

/** (4) module verification turned off around the pinned install. */
function mutateAddGoprivate(source: string): string {
  const lines = source.split('\n');
  const i = lineIndex(
    lines,
    (l) => l.includes('GO_LICENSES_VERSION:'),
    'the job-level pin env',
  );
  lines.splice(i, 0, `${indentOf(lines[i])}GOPRIVATE: '*'`);
  return lines.join('\n');
}

/** (5) the step's own logic dropped from the filter that gates the step. */
function mutateDropScriptFromGoDeps(source: string): string {
  const lines = source.split('\n');
  const i = lineIndex(
    lines,
    (l) => l.trim() === "- 'scripts/go-licenses-check.sh'",
    'the go_deps entry for the script',
  );
  lines.splice(i, 1);
  return lines.join('\n');
}

/** (6) the frontend lockfile back in the Go filter — the original defect. */
function mutateAddWebLockfileToGoDeps(source: string): string {
  const lines = source.split('\n');
  const i = lineIndex(
    lines,
    (l) => l.trim() === "- 'apps/backend/go.mod'",
    'the go_deps entry for go.mod',
  );
  lines.splice(i, 0, `${indentOf(lines[i])}- 'apps/web/package-lock.json'`);
  return lines.join('\n');
}

/** (7) the per-step gate removed, so the Go check runs on any deps change. */
function mutateRemoveGoStepIf(source: string): string {
  return source
    .split('\n')
    .filter((l) => !l.includes('needs.changes.outputs.go_deps == \'true\'') ||
      !l.trim().startsWith('if:'))
    .join('\n');
}

/** (8) a fourth action in a job whose action set is held at three. */
function mutateAddFourthUsesStep(source: string): string {
  const lines = source.split('\n');
  const i = lineIndex(
    lines,
    (l) => l.trim() === '- name: Check Go licenses',
    'the Check Go licenses step header',
  );
  lines.splice(i, 0, `${indentOf(lines[i])}- uses: aquasecurity/trivy-action@master`, '');
  return lines.join('\n');
}

/** (9) class `unknown` quietly dropped from the checker's class set. */
function mutateScriptDropUnknownClass(source: string): string {
  const needle = 'forbidden,unknown,restricted,reciprocal';
  expect(
    source,
    'the script must carry the four disallowed classes as one literal',
  ).toContain(needle);
  return source.split(needle).join('forbidden,restricted,reciprocal');
}

// ══ AC1 ═══════════════════════════════════════════════════════════════

describe('the Go license verdict is the checker exit status', () => {
  it('QGF-251.AC1 the Check Go licenses step is one line ending in scripts/go-licenses-check.sh, with no pipe, tee, grep or continue-on-error', () => {
    assertGoStepRunsOnlyTheScript(workflowSource());
  });

  it('QGF-251.AC1 the script runs the checker as `check ./... --disallowed_types=forbidden,unknown,restricted,reciprocal` with apps/backend as its working directory', () => {
    const run = runCheckScript();
    assertDisallowedTypesArgv(run.log);
    assertCheckerRanInAppsBackend(run.log, run.repoRoot);
  });

  it('QGF-251.AC1 a planted reciprocal license the base grep would have missed makes the script exit 1', () => {
    const run = runCheckScript({ check: [{ rc: 1, out: PLANTED_FINDING }] });
    expect(run.status, `stdout:\n${run.stdout}\nstderr:\n${run.stderr}`).toBe(1);
    expect(run.summary).toContain(`License Compliance (Go): ${PHRASE.finding}`);
    expect(run.summary).toContain("example.com/planted");
  });

  it('QGF-251.AC1 a zero checker exit yields 0 and a non-zero checker exit never yields 0, whatever the text says', () => {
    expect(runCheckScript({ check: [{ rc: 0, out: '' }] }).status).toBe(0);
    for (const out of [
      PLANTED_FINDING,
      'dial tcp 142.250.1.1:443: i/o timeout',
      'go: error loading module requirements',
      '',
    ]) {
      const run = runCheckScript({ check: [{ rc: 1, out }] });
      expect(
        run.status,
        `a non-zero checker exit must never yield 0 (output: ${JSON.stringify(out)})`,
      ).not.toBe(0);
    }
  });
});

// ══ AC2 ═══════════════════════════════════════════════════════════════

describe('the go-licenses pin is a literal asserted on the installed binary', () => {
  it('QGF-251.AC2 security.yml pins GO_LICENSES_VERSION and GO_LICENSES_SUM as literals', () => {
    assertPinEnvLiterals(workflowSource());
  });

  it('QGF-251.AC2 a comment beside the pin dates it: the v2.0.1 release, the measurement and the sum source', () => {
    assertPinCommentDatesThePin(workflowSource());
  });

  it('QGF-251.AC2 the script installs the pinned /v2 module version into GOBIN, never @latest and never the v1 module path', () => {
    assertInstallArgv(runCheckScript().log);
  });

  it('QGF-251.AC2 the checker that runs is the verified binary in GOBIN, not the go-licenses first on PATH', () => {
    const run = runCheckScript();
    assertCheckerRanFromGobin(run.log, run.gobin);
  });

  it('QGF-251.AC2 a `go version -m` mod line whose sum differs exits 2 tool checksum mismatch and never invokes the checker', () => {
    const run = runCheckScript({
      modSum: 'h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=',
    });
    expect(run.status).toBe(2);
    expect(run.summary).toContain(`License Compliance (Go): ${PHRASE.badSum}`);
    expect(run.checks, 'the checker must never run on an unverified binary').toEqual([]);
  });

  it('QGF-251.AC2 an absent `go version -m` mod line exits 2 tool checksum mismatch and never invokes the checker', () => {
    const run = runCheckScript({ emitModLine: false });
    expect(run.status).toBe(2);
    expect(run.summary).toContain(`License Compliance (Go): ${PHRASE.badSum}`);
    expect(run.checks).toEqual([]);
  });

  it('QGF-251.AC2 neither security.yml nor the script relaxes module verification', () => {
    assertNoGoEscapeHatchesInWorkflow(workflowSource());
    assertNoGoEscapeHatchesReachTheToolchain(runCheckScript().log);
  });
});

// ══ AC3 ═══════════════════════════════════════════════════════════════

describe('every failure is classified once, in a fixed order', () => {
  it('QGF-251.AC3 (a) the checker exits 0: exit 0, no disallowed license found, one check invocation', () => {
    const run = runCheckScript();
    expect(run.status).toBe(0);
    expect(run.summary).toContain(`License Compliance (Go): ${PHRASE.pass}`);
    expect(run.checks).toHaveLength(1);
  });

  it('QGF-251.AC3 (b) the checker reports a reciprocal finding: exit 1, disallowed license found, one check invocation', () => {
    const run = runCheckScript({ check: [{ rc: 1, out: PLANTED_FINDING }] });
    expect(run.status).toBe(1);
    expect(run.checks).toHaveLength(1);

    // One verdict line, then the output that classified it — to the step
    // summary and, identically, to stdout.
    const expected = [
      `License Compliance (Go): ${PHRASE.finding}`,
      PLANTED_FINDING,
    ];
    expect(run.summary.trim().split('\n')).toEqual(expected);
    expect(run.stdout.trim().split('\n')).toEqual(expected);
  });

  it('QGF-251.AC3 (c) the install hits one proxy stream error then succeeds: exit 0, two install invocations', () => {
    const run = runCheckScript({
      install: [
        { rc: 1, out: `go: downloading ${GO_LICENSES_MODULE} ${GO_LICENSES_VERSION}\n${STREAM_ERROR}` },
        { rc: 0, out: '' },
      ],
    });
    expect(run.status, run.stdout).toBe(0);
    expect(run.summary).toContain(`License Compliance (Go): ${PHRASE.pass}`);
    expect(run.installs).toHaveLength(2);
  });

  it('QGF-251.AC3 (d) the install hits that error twice: exit 2, tool could not be built, two install invocations', () => {
    const run = runCheckScript({ install: [{ rc: 1, out: STREAM_ERROR }] });
    expect(run.status).toBe(2);
    expect(run.summary).toContain(`License Compliance (Go): ${PHRASE.notBuilt}`);
    expect(run.installs).toHaveLength(2);
    expect(run.checks, 'no checker may run without a tool').toEqual([]);
  });

  it('QGF-251.AC3 (e) the checker hits one network-class failure then exits 0: exit 0, two check invocations', () => {
    const run = runCheckScript({
      check: [
        { rc: 1, out: 'go: dial tcp: i/o timeout' },
        { rc: 0, out: '' },
      ],
    });
    expect(run.status, run.stdout).toBe(0);
    expect(run.summary).toContain(`License Compliance (Go): ${PHRASE.pass}`);
    expect(run.checks).toHaveLength(2);
  });

  it('QGF-251.AC3 (f) the checker hits that failure twice: exit 2, checker could not run, two check invocations', () => {
    const run = runCheckScript({ check: [{ rc: 1, out: 'go: dial tcp: i/o timeout' }] });
    expect(run.status).toBe(2);
    expect(run.summary).toContain(`License Compliance (Go): ${PHRASE.cannotRun}`);
    expect(run.checks).toHaveLength(2);
  });

  it('QGF-251.AC3 (g) a finding naming a library that matches the network pattern is still a finding: exit 1, one check invocation', () => {
    const run = runCheckScript({ check: [{ rc: 1, out: PLANTED_FINDING_TIMEOUT }] });
    expect(run.status).toBe(1);
    expect(run.summary).toContain(`License Compliance (Go): ${PHRASE.finding}`);
    expect(
      run.summary,
      'a finding must never be reported as a tool failure',
    ).not.toContain(PHRASE.cannotRun);
    expect(run.checks, 'a finding is never retried').toHaveLength(1);
  });

  it('QGF-251.AC3 (h) the checker exits 1 with neither a finding nor a network error: exit 2, checker failed before a verdict, one check invocation', () => {
    const run = runCheckScript({
      check: [{ rc: 1, out: 'go: error loading module requirements' }],
    });
    expect(run.status).toBe(2);
    expect(run.summary).toContain(`License Compliance (Go): ${PHRASE.noVerdict}`);
    expect(
      run.summary,
      'an unclassified checker failure must not be labelled a license failure',
    ).not.toContain(PHRASE.finding);
    expect(run.checks).toHaveLength(1);
  });

  it('QGF-251.AC3 (i) a checksum mismatch during install is not retried: exit 2, tool checksum mismatch, one install invocation', () => {
    const run = runCheckScript({
      install: [
        {
          rc: 1,
          out: [
            `verifying ${GO_LICENSES_MODULE}@${GO_LICENSES_VERSION}: checksum mismatch`,
            '\tdownloaded: h1:wrongwrongwrongwrongwrongwrongwrongwrongwro=',
            `\t${GO_LICENSES_SUM}`,
            'SECURITY ERROR',
            'This download does NOT match an earlier download recorded in go.sum.',
          ].join('\n'),
        },
      ],
    });
    expect(run.status).toBe(2);
    expect(run.summary).toContain(`License Compliance (Go): ${PHRASE.badSum}`);
    expect(
      run.installs,
      'a checksum mismatch is never retried: a retry would just fetch the same bad module',
    ).toHaveLength(1);
    expect(run.checks).toEqual([]);
  });

  it('QGF-251.AC3 (j) the finding cell with GITHUB_STEP_SUMMARY unset: exit 1 unchanged, the phrase still on stdout', () => {
    const run = runCheckScript({
      check: [{ rc: 1, out: PLANTED_FINDING }],
      stepSummary: false,
    });
    expect(run.status).toBe(1);
    expect(run.stdout).toContain(`License Compliance (Go): ${PHRASE.finding}`);
    expect(run.summary, 'no summary file must have been written').toBe('');
    expect(run.checks).toHaveLength(1);
  });
});

// ══ AC4 ═══════════════════════════════════════════════════════════════

describe('the Go step and the npm step are gated on their own inputs', () => {
  it('QGF-251.AC4 go_deps lists the Go module, the workflow and the script, and nothing outside apps/backend', () => {
    assertGoDepsFilter(workflowSource());
  });

  it('QGF-251.AC4 npm_deps lists the web manifests and the workflow', () => {
    assertNpmDepsFilter(workflowSource());
  });

  it('QGF-251.AC4 the single `deps` filter, its job output and every read of it are gone', () => {
    assertDepsFilterIsGone(workflowSource());
  });

  it('QGF-251.AC4 each step carries its own gate and the job admits schedule, go_deps or npm_deps', () => {
    assertPerStepGating(workflowSource());
  });

  it('QGF-251.AC4 a pull request whose only changed file is apps/web/package-lock.json sets go_deps false, while apps/backend/go.sum sets it true', () => {
    assertLockfileOnlyPrSkipsTheGoStep(workflowSource());
  });

  it('QGF-251.AC4 the Check npm licenses step keeps its base run text and working directory', () => {
    assertNpmStepUnchanged(workflowSource());
  });
});

// ══ AC5 ═══════════════════════════════════════════════════════════════

describe('each bound property refuses a planted regression', () => {
  it('QGF-251.AC5 (1) restoring the base run block — @latest, the tee pipe and the text grep — is refused', () => {
    expect(() =>
      assertGoStepRunsOnlyTheScript(mutateRestoreBaseGoRun(workflowSource())),
    ).toThrow();
  });

  it('QGF-251.AC5 (2) continue-on-error on the Go step is refused', () => {
    expect(() =>
      assertGoStepRunsOnlyTheScript(mutateAddContinueOnError(workflowSource())),
    ).toThrow();
  });

  it('QGF-251.AC5 (3) dropping GO_LICENSES_SUM from the env is refused', () => {
    expect(() =>
      assertPinEnvLiterals(mutateRemovePinSum(workflowSource())),
    ).toThrow();
  });

  it('QGF-251.AC5 (4) adding GOPRIVATE to the job env is refused', () => {
    expect(() =>
      assertNoGoEscapeHatchesInWorkflow(mutateAddGoprivate(workflowSource())),
    ).toThrow();
  });

  it('QGF-251.AC5 (5) dropping scripts/go-licenses-check.sh from go_deps is refused', () => {
    expect(() =>
      assertGoDepsFilter(mutateDropScriptFromGoDeps(workflowSource())),
    ).toThrow();
  });

  it('QGF-251.AC5 (6) adding apps/web/package-lock.json to go_deps is refused', () => {
    const mutated = mutateAddWebLockfileToGoDeps(workflowSource());
    expect(() => assertGoDepsFilter(mutated)).toThrow();
    expect(() => assertLockfileOnlyPrSkipsTheGoStep(mutated)).toThrow();
  });

  it('QGF-251.AC5 (7) removing the Go step\'s if: is refused', () => {
    expect(() =>
      assertPerStepGating(mutateRemoveGoStepIf(workflowSource())),
    ).toThrow();
  });

  it('QGF-251.AC5 (8) a fourth uses: step in the license-check job is refused', () => {
    expect(() =>
      assertLicenseCheckActionSet(mutateAddFourthUsesStep(workflowSource())),
    ).toThrow();
  });

  it('QGF-251.AC5 (9) a copy of the script with class `unknown` dropped is refused by the argv assertion', () => {
    const run = runCheckScript({
      scriptSource: mutateScriptDropUnknownClass(scriptSource()),
    });
    expect(
      run.checks,
      'the mutated copy must still have reached the checker, or the mutation proves nothing',
    ).toHaveLength(1);
    expect(() => assertDisallowedTypesArgv(run.log)).toThrow();
  });

  it('QGF-251.AC5 none of those assertions throws on the delivered files', () => {
    const source = workflowSource();
    assertGoStepRunsOnlyTheScript(source);
    assertPinEnvLiterals(source);
    assertPinCommentDatesThePin(source);
    assertNoGoEscapeHatchesInWorkflow(source);
    assertGoDepsFilter(source);
    assertNpmDepsFilter(source);
    assertDepsFilterIsGone(source);
    assertPerStepGating(source);
    assertLockfileOnlyPrSkipsTheGoStep(source);
    assertNpmStepUnchanged(source);
    assertTopLevelPermissions(source);
    assertLicenseCheckActionSet(source);
    assertSiblingFiltersHeld(source);

    const run = runCheckScript();
    assertDisallowedTypesArgv(run.log);
    assertCheckerRanInAppsBackend(run.log, run.repoRoot);
    assertInstallArgv(run.log);
    assertCheckerRanFromGobin(run.log, run.gobin);
    assertNoGoEscapeHatchesReachTheToolchain(run.log);
  });
});

// ══ AC6 ═══════════════════════════════════════════════════════════════

describe('the workflow permissions, action set and sibling filters are held', () => {
  it('QGF-251.AC6 the top-level permissions are exactly contents: read and security-events: write, and the job adds none', () => {
    assertTopLevelPermissions(workflowSource());
  });

  it('QGF-251.AC6 the license-check job uses exactly checkout, setup-go and setup-node, at the base tag or a sha with its version comment', () => {
    assertLicenseCheckActionSet(workflowSource());
  });

  it('QGF-251.AC6 the docker filter keeps its five patterns and the backend, frontend and crypto filters keep theirs', () => {
    assertSiblingFiltersHeld(workflowSource());
  });

  it('QGF-251.AC6 scripts/go-licenses-check.sh is delivered executable', () => {
    expect(existsSync(SCRIPT_PATH), 'the script must be delivered').toBe(true);
    expect(
      statSync(SCRIPT_PATH).mode & 0o111,
      'scripts/go-licenses-check.sh must be mode 100755: the step execs it',
    ).toBe(0o111);
  });
});
