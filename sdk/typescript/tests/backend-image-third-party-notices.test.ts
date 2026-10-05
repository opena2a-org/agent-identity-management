import { describe, it, expect } from 'vitest';
import { spawnSync } from 'child_process';
import {
  chmodSync,
  copyFileSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  realpathSync,
  rmSync,
  writeFileSync,
} from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { load } from 'js-yaml';

/**
 * The published backend image (infrastructure/docker/Dockerfile.backend, built
 * and pushed by docker-publish.yml) carries the third-party license notices its
 * binaries owe.
 *
 * Every library linked into aim-server, aim-migrate and aim-bootstrap is
 * redistributed in the image. Before this file the image held the binaries,
 * the migrations and the SDK directory and no license material at all: no
 * MIT or BSD license text, and nothing telling a recipient where to obtain the
 * source of the ten MPL-2.0 HashiCorp modules admitted in
 * apps/backend/go-licenses-allowlist.tsv (MPL-2.0 section 3.2(a)).
 *
 * The builder stage now runs apps/backend/scripts/third-party-notices.sh,
 * which installs go-licenses at the license gate's pin, writes
 * `go-licenses save` (license texts, and full source for MPL-2.0) plus a
 * manifest from `go-licenses report` (the source pointer per library), and
 * verifies its own output before the build continues. The runtime stage copies
 * the result to /app/third_party.
 *
 * Covered here:
 *   - the Dockerfile wiring, as a function over the Dockerfile text, so each
 *     planted regression (a swallowed failure, a dropped COPY, a drifted pin)
 *     is refused;
 *   - the pin equals the one security.yml gives the license gate;
 *   - the script, end to end against stub `go` and `go-licenses` binaries:
 *     the tree it writes, the ten MPL-2.0 entries by module path, and every
 *     failure it must turn into a failed build with nothing left behind.
 */

const REPO_ROOT = realpathSync(join(__dirname, '..', '..', '..'));
const DOCKERFILE_PATH = join(REPO_ROOT, 'infrastructure', 'docker', 'Dockerfile.backend');
const SCRIPT_PATH = join(REPO_ROOT, 'apps', 'backend', 'scripts', 'third-party-notices.sh');
const ALLOWLIST_PATH = join(REPO_ROOT, 'apps', 'backend', 'go-licenses-allowlist.tsv');
const SECURITY_WORKFLOW_PATH = join(REPO_ROOT, '.github', 'workflows', 'security.yml');

const CHECKER_MODULE = 'github.com/google/go-licenses/v2';

/** The builder-stage RUN, exactly. Anything appended can swallow a failure. */
const NOTICES_RUN = 'CGO_ENABLED=0 sh scripts/third-party-notices.sh /third_party';

/** The runtime-stage COPY, exactly: WORKDIR /app makes it /app/third_party. */
const NOTICES_COPY = '--from=builder /third_party ./third_party';

/** The MPL-2.0 libraries the image must carry with their source. */
const MPL_LIBRARIES = [
  'github.com/hashicorp/vault/api',
  'github.com/hashicorp/errwrap',
  'github.com/hashicorp/go-cleanhttp',
  'github.com/hashicorp/go-multierror',
  'github.com/hashicorp/go-retryablehttp',
  'github.com/hashicorp/go-rootcerts',
  'github.com/hashicorp/go-secure-stdlib/parseutil',
  'github.com/hashicorp/go-secure-stdlib/strutil',
  'github.com/hashicorp/go-sockaddr',
  'github.com/hashicorp/hcl',
];

// ---------------------------------------------------------------------------
// The pin
// ---------------------------------------------------------------------------

interface Pin {
  version: string;
  sum: string;
}

/** The go-licenses pin from the one security.yml job that declares it. */
function securityWorkflowPin(): Pin {
  const doc = load(readFileSync(SECURITY_WORKFLOW_PATH, 'utf-8')) as {
    jobs: Record<string, { env?: Record<string, unknown> }>;
  };
  const envs = Object.values(doc.jobs)
    .map((job) => job.env)
    .filter((env): env is Record<string, unknown> => !!env && 'GO_LICENSES_VERSION' in env);
  expect(envs, 'exactly one security.yml job pins go-licenses').toHaveLength(1);
  const version = String(envs[0].GO_LICENSES_VERSION);
  const sum = String(envs[0].GO_LICENSES_SUM);
  expect(version).toMatch(/^v\d+\.\d+\.\d+$/);
  expect(sum).toMatch(/^h1:[A-Za-z0-9+/]+=*$/);
  return { version, sum };
}

// ---------------------------------------------------------------------------
// The Dockerfile wiring
// ---------------------------------------------------------------------------

interface Instruction {
  keyword: string;
  /** Arguments with continuations joined and whitespace collapsed. */
  args: string;
  /** 1-indexed line the instruction starts on. */
  line: number;
}

interface Stage {
  name?: string;
  instructions: Instruction[];
}

function parseDockerfile(text: string): Stage[] {
  const lines = text.split('\n');
  const stages: Stage[] = [];
  let i = 0;
  while (i < lines.length) {
    const start = i;
    let full = lines[i];
    i++;
    if (/^\s*(#.*)?$/.test(full)) continue;
    while (/\\\s*$/.test(full) && i < lines.length) {
      full = full.replace(/\\\s*$/, ' ') + lines[i];
      i++;
    }
    const match = /^\s*(\S+)\s*(.*)$/.exec(full);
    if (!match) continue;
    const keyword = match[1].toUpperCase();
    const args = match[2].replace(/\s+/g, ' ').trim();
    if (keyword === 'FROM') {
      const from = /^\S+(?:\s+AS\s+(\S+))?$/i.exec(args);
      stages.push({ name: from?.[1], instructions: [] });
      continue;
    }
    if (stages.length === 0) continue; // a global ARG before the first FROM
    stages[stages.length - 1].instructions.push({ keyword, args, line: start + 1 });
  }
  return stages;
}

/** Every way the Dockerfile text fails to ship verified notices; [] is a pass. */
function noticesWiringFailures(dockerfile: string, pin: Pin): string[] {
  const failures: string[] = [];
  const stages = parseDockerfile(dockerfile);
  const builder = stages.find((s) => s.name === 'builder');
  const runtime = stages[stages.length - 1];
  if (!builder) return ['there is no stage named builder'];
  if (runtime === builder) return ['the final stage is the builder stage'];

  const b = builder.instructions;
  if (!b.some((x) => x.keyword === 'WORKDIR' && x.args === '/app')) {
    failures.push('builder: WORKDIR /app, where apps/backend lands, is missing');
  }
  if (!b.some((x) => x.keyword === 'COPY' && x.args === 'apps/backend/ ./')) {
    failures.push('builder: `COPY apps/backend/ ./`, which brings scripts/third-party-notices.sh, is missing');
  }

  const runs = b.filter((x) => x.keyword === 'RUN' && x.args.includes('third-party-notices'));
  if (runs.length !== 1 || runs[0].args !== NOTICES_RUN) {
    failures.push(
      `builder: expected exactly one \`RUN ${NOTICES_RUN}\`, found ${JSON.stringify(runs.map((r) => r.args))}`,
    );
  }

  const wanted: [string, string][] = [
    ['GO_LICENSES_VERSION', pin.version],
    ['GO_LICENSES_SUM', pin.sum],
  ];
  for (const [name, value] of wanted) {
    const decls = b.filter((x) => x.keyword === 'ARG' && x.args.split('=')[0] === name);
    if (decls.length !== 1 || decls[0].args !== `${name}=${value}`) {
      failures.push(
        `builder: expected exactly one \`ARG ${name}=${value}\` (the security.yml pin), found ${JSON.stringify(decls.map((d) => d.args))}`,
      );
    } else if (runs.length === 1 && decls[0].line > runs[0].line) {
      failures.push(`builder: ARG ${name} is declared after the notices RUN, which then runs without it`);
    }
    if (b.some((x) => x.keyword === 'ENV' && new RegExp(`(^|\\s)${name}[=\\s]`).test(x.args))) {
      failures.push(`builder: an ENV overrides ${name}`);
    }
  }

  const r = runtime.instructions;
  if (!r.some((x) => x.keyword === 'WORKDIR' && x.args === '/app')) {
    failures.push('runtime: WORKDIR /app is missing, so the notices would not land at /app/third_party');
  }
  const copies = r.filter((x) => x.keyword === 'COPY' && x.args.includes('third_party'));
  if (copies.length !== 1 || copies[0].args !== NOTICES_COPY) {
    failures.push(
      `runtime: expected exactly one \`COPY ${NOTICES_COPY}\`, found ${JSON.stringify(copies.map((c) => c.args))}`,
    );
  }
  return failures;
}

describe('Dockerfile.backend ships verified third-party notices', () => {
  const dockerfile = readFileSync(DOCKERFILE_PATH, 'utf-8');

  it('the builder runs the notices script at the license gate pin and the runtime copies its output to /app/third_party', () => {
    expect(noticesWiringFailures(dockerfile, securityWorkflowPin())).toEqual([]);
  });

  it('the notices script the builder runs is in the tree under apps/backend', () => {
    expect(existsSync(SCRIPT_PATH), SCRIPT_PATH).toBe(true);
  });

  const runLine = `RUN ${NOTICES_RUN}`;
  const copyLine = `COPY ${NOTICES_COPY}`;
  const mutations: [string, string, string][] = [
    ['a failure swallowed with || true', runLine, `${runLine} || true`],
    ['a failure swallowed with ; true', runLine, `${runLine}; true`],
    ['a failure swallowed with set +e', runLine, `RUN set +e; ${NOTICES_RUN}`],
    ['the notices RUN removed', runLine, ''],
    ['the runtime COPY removed', copyLine, ''],
    ['the runtime COPY of only the license texts', copyLine, 'COPY --from=builder /third_party/licenses ./third_party'],
    ['the version pin drifted', 'ARG GO_LICENSES_VERSION=v2.0.1', 'ARG GO_LICENSES_VERSION=v2.0.2'],
    ['the sum pin dropped', /^ARG GO_LICENSES_SUM=.*$/m.exec(dockerfile)?.[0] ?? '<absent>', 'ARG GO_LICENSES_SUM'],
  ];
  for (const [label, from, to] of mutations) {
    it(`refuses a planted regression: ${label}`, () => {
      expect(dockerfile.includes(from), `the Dockerfile carries ${JSON.stringify(from)}`).toBe(true);
      const mutated = dockerfile.replace(from, to);
      expect(mutated).not.toBe(dockerfile);
      expect(noticesWiringFailures(mutated, securityWorkflowPin()).length).toBeGreaterThan(0);
    });
  }
});

// ---------------------------------------------------------------------------
// The script, against stub `go` and `go-licenses`
// ---------------------------------------------------------------------------

/**
 * Stub `go`: `install` materialises the stub checker in $GOBIN as a real
 * install would; `version -m` reports the build-info mod line.
 */
const STUB_GO = `#!/bin/sh
printf 'go %s\\n' "$*" >> "$STUB_LOG"
case "$1" in
  install)
    rc="$(cat "$STUB_DIR/install.rc")"
    if [ "$rc" -eq 0 ]; then
      cp "$STUB_DIR/go-licenses" "$GOBIN/go-licenses"
      chmod 755 "$GOBIN/go-licenses"
    fi
    exit "$rc"
    ;;
  version)
    printf '%s: go1.25.0\\n' "$3"
    printf '\\tpath\\t%s\\n' "$STUB_MOD_PATH"
    printf '\\tmod\\t%s\\t%s\\t%s\\n' "$STUB_MOD_PATH" "$STUB_MOD_VERSION" "$STUB_MOD_SUM"
    exit 0
    ;;
esac
printf 'stub go: unexpected invocation: %s\\n' "$*" >&2
exit 97
`;

/**
 * Stub checker: `report` prints the planted CSV; `save` copies the planted tree
 * to --save_path (refusing an existing one, as the real tool does) and then
 * exits with the planted status, so a failing save leaves partial output.
 */
const STUB_GO_LICENSES = `#!/bin/sh
printf '%s %s [cwd %s]\\n' "$0" "$*" "$PWD" >> "$STUB_LOG"
case "$1" in
  report)
    cat "$STUB_DIR/report.csv"
    printf 'W stub warning on stderr\\n' >&2
    exit "$(cat "$STUB_DIR/report.rc")"
    ;;
  save)
    dest=""
    for arg in "$@"; do
      case "$arg" in --save_path=*) dest="$(printf '%s' "$arg" | sed 's/^--save_path=//')" ;; esac
    done
    [ -n "$dest" ] || exit 98
    [ ! -e "$dest" ] || exit 99
    cp -R "$STUB_DIR/tree" "$dest"
    exit "$(cat "$STUB_DIR/save.rc")"
    ;;
esac
exit 97
`;

interface Library {
  name: string;
  url: string;
  license: string;
}

function lib(name: string, license: string): Library {
  return { name, url: `https://${name}/blob/v1.0.0/LICENSE`, license };
}

/** The ruled rows of the real allowlist, as [library, license]. */
function allowlistRows(): [string, string][] {
  return readFileSync(ALLOWLIST_PATH, 'utf-8')
    .split('\n')
    .filter((line) => !/^[ \t]*(#|$)/.test(line))
    .map((line) => {
      const [library, license] = line.split('\t');
      return [library, license];
    });
}

/** A module graph shaped like the backend's: permissive libraries, a nested path, the MPL-2.0 ten. */
function backendLikeLibraries(): Library[] {
  return [
    lib('cloud.google.com/go/auth', 'Apache-2.0'),
    lib('cloud.google.com/go/auth/oauth2adapt', 'Apache-2.0'),
    lib('github.com/google/uuid', 'BSD-3-Clause'),
    lib('github.com/opena2a-org/agent-identity-management/apps/backend', 'Apache-2.0'),
    lib('golang.org/x/crypto', 'BSD-3-Clause'),
    ...allowlistRows().map(([library, license]) => lib(library, license)),
  ];
}

interface Sandbox {
  libraries?: Library[];
  /** library names save writes no directory for */
  unsaved?: string[];
  /** MPL-2.0 library names save writes without source */
  withoutSource?: string[];
  installRc?: number;
  reportRc?: number;
  saveRc?: number;
  modSum?: string;
  env?: Record<string, string | undefined>;
  /** create the output directory before the run */
  outExists?: boolean;
}

interface Run {
  status: number | null;
  stdout: string;
  stderr: string;
  log: string[];
  root: string;
  moduleRoot: string;
  gobin: string;
  out: string;
  /** what the run left at <out>: file paths relative to it, or null when absent */
  outFiles: string[] | null;
  partialExists: boolean;
  manifest: string | null;
  readme: string | null;
}

function listFiles(dir: string, prefix = ''): string[] {
  const files: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const rel = prefix ? `${prefix}/${entry.name}` : entry.name;
    if (entry.isDirectory()) files.push(...listFiles(join(dir, entry.name), rel));
    else files.push(rel);
  }
  return files.sort();
}

function writeExecutable(path: string, contents: string): void {
  writeFileSync(path, contents);
  chmodSync(path, 0o755);
}

function runNotices(opts: Sandbox = {}): Run {
  const pin = securityWorkflowPin();
  const libraries = opts.libraries ?? backendLikeLibraries();
  const root = mkdtempSync(join(tmpdir(), 'aim-notices-'));
  try {
    const moduleRoot = join(root, 'mod');
    const stubDir = join(root, 'stub');
    const binDir = join(stubDir, 'bin');
    const gobin = join(root, 'gobin');
    const tmp = join(root, 'tmp');
    const out = join(root, 'image', 'third_party');
    for (const dir of [join(moduleRoot, 'scripts'), binDir, gobin, tmp, join(root, 'image')]) {
      mkdirSync(dir, { recursive: true });
    }
    copyFileSync(SCRIPT_PATH, join(moduleRoot, 'scripts', 'third-party-notices.sh'));
    copyFileSync(ALLOWLIST_PATH, join(moduleRoot, 'go-licenses-allowlist.tsv'));
    if (opts.outExists) mkdirSync(out);

    writeExecutable(join(binDir, 'go'), STUB_GO);
    // A decoy on PATH: the script must run the checker it verified in GOBIN.
    writeExecutable(join(binDir, 'go-licenses'), STUB_GO_LICENSES);
    writeExecutable(join(stubDir, 'go-licenses'), STUB_GO_LICENSES);
    writeFileSync(join(stubDir, 'install.rc'), String(opts.installRc ?? 0));
    writeFileSync(join(stubDir, 'report.rc'), String(opts.reportRc ?? 0));
    writeFileSync(join(stubDir, 'save.rc'), String(opts.saveRc ?? 0));
    writeFileSync(
      join(stubDir, 'report.csv'),
      libraries.map((l) => `${l.name},${l.url},${l.license}\n`).join(''),
    );
    const tree = join(stubDir, 'tree');
    mkdirSync(tree);
    for (const l of libraries) {
      if (opts.unsaved?.includes(l.name)) continue;
      const dir = join(tree, ...l.name.split('/'));
      mkdirSync(dir, { recursive: true });
      writeFileSync(join(dir, 'LICENSE'), `license text of ${l.name}\n`);
      if (l.license === 'MPL-2.0' && !opts.withoutSource?.includes(l.name)) {
        writeFileSync(join(dir, 'source.go'), `package x // source of ${l.name}\n`);
      }
    }

    const logPath = join(stubDir, 'log');
    writeFileSync(logPath, '');
    const env: Record<string, string> = {
      PATH: `${binDir}:${process.env.PATH ?? '/usr/bin:/bin'}`,
      HOME: root,
      TMPDIR: tmp,
      GOBIN: gobin,
      GO_LICENSES_VERSION: pin.version,
      GO_LICENSES_SUM: pin.sum,
      STUB_DIR: stubDir,
      STUB_LOG: logPath,
      STUB_MOD_PATH: CHECKER_MODULE,
      STUB_MOD_VERSION: pin.version,
      STUB_MOD_SUM: opts.modSum ?? pin.sum,
    };
    for (const [key, value] of Object.entries(opts.env ?? {})) {
      if (value === undefined) delete env[key];
      else env[key] = value;
    }

    // As the builder stage runs it: from the module root, by relative path.
    const result = spawnSync('sh', ['scripts/third-party-notices.sh', out], {
      cwd: moduleRoot,
      env,
      encoding: 'utf-8',
    });
    const read = (path: string) => (existsSync(path) ? readFileSync(path, 'utf-8') : null);
    return {
      status: result.status,
      stdout: result.stdout,
      stderr: result.stderr,
      log: readFileSync(logPath, 'utf-8').split('\n').filter((line) => line.length > 0),
      root,
      moduleRoot,
      gobin,
      out,
      outFiles: existsSync(out) ? listFiles(out) : null,
      partialExists: existsSync(`${out}.partial`),
      manifest: read(join(out, 'manifest.csv')),
      readme: read(join(out, 'README')),
    };
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
}

describe('third-party-notices.sh writes and verifies the notices tree', () => {
  it('the allowlist the script verifies against holds exactly the ten MPL-2.0 modules', () => {
    const rows = allowlistRows();
    expect(rows.map(([library]) => library).sort()).toEqual([...MPL_LIBRARIES].sort());
    for (const [, license] of rows) expect(license).toBe('MPL-2.0');
  });

  it('writes license texts, MPL-2.0 source, a manifest and a README, and names the ten MPL-2.0 modules', () => {
    const libraries = backendLikeLibraries();
    const run = runNotices({ libraries });
    expect(run.stderr).toBe('');
    expect(run.status).toBe(0);

    const lines = run.stdout.trimEnd().split('\n');
    expect(lines[0]).toBe(
      `third-party notices: ${libraries.length} libraries, 10 MPL-2.0 with source, 10 allowlisted, written to ${run.out}`,
    );
    expect(lines.slice(1)).toEqual(MPL_LIBRARIES.map((m) => `allowlisted: ${m} MPL-2.0`));

    expect(run.manifest).toBe(
      'library,licenseURL,licenseName\n' +
        libraries.map((l) => `${l.name},${l.url},${l.license}\n`).join(''),
    );
    const expected = ['README', 'manifest.csv'];
    for (const l of libraries) {
      expected.push(`licenses/${l.name}/LICENSE`);
      if (l.license === 'MPL-2.0') expected.push(`licenses/${l.name}/source.go`);
    }
    expect(run.outFiles).toEqual(expected.sort());
    for (const m of MPL_LIBRARIES) {
      expect(run.outFiles).toContain(`licenses/${m}/LICENSE`);
      expect(run.outFiles).toContain(`licenses/${m}/source.go`);
      expect(run.manifest).toMatch(new RegExp(`^${m.replace(/[.]/g, '\\.')},[^\\n]*,MPL-2\\.0$`, 'm'));
    }
    expect(run.readme).toContain('manifest.csv');
    expect(run.readme).toContain('licenses/');
    expect(run.partialExists).toBe(false);
  });

  it('installs the pinned checker and runs that binary, not one off PATH, from the module root', () => {
    const run = runNotices();
    expect(run.status).toBe(0);
    const pin = securityWorkflowPin();
    const checker = join(run.gobin, 'go-licenses');
    expect(run.log).toEqual([
      `go install ${CHECKER_MODULE}@${pin.version}`,
      `go version -m ${checker}`,
      `${checker} report ./... [cwd ${run.moduleRoot}]`,
      `${checker} save ./... --save_path=${run.out}.partial/licenses [cwd ${run.moduleRoot}]`,
    ]);
  });

  interface Refusal {
    label: string;
    opts: () => Sandbox;
    status: number;
    message: string;
    /** whether the checker may have run before the refusal */
    checkerRan: boolean;
  }

  const hcl = 'github.com/hashicorp/hcl';
  const uuid = 'github.com/google/uuid';
  const refusals: Refusal[] = [
    {
      label: 'GO_LICENSES_SUM unset',
      opts: () => ({ env: { GO_LICENSES_SUM: undefined } }),
      status: 2,
      message: 'GO_LICENSES_VERSION and GO_LICENSES_SUM must both be set',
      checkerRan: false,
    },
    {
      label: 'the output directory already exists',
      opts: () => ({ outExists: true }),
      status: 2,
      message: 'already exists; notices are written to a new directory',
      checkerRan: false,
    },
    {
      label: 'go install fails',
      opts: () => ({ installRc: 1 }),
      status: 1,
      message: `go install ${CHECKER_MODULE}@v2.0.1 failed`,
      checkerRan: false,
    },
    {
      label: 'the installed checker is not the pinned build',
      opts: () => ({ modSum: 'h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=' }),
      status: 1,
      message: `is not ${CHECKER_MODULE} v2.0.1`,
      checkerRan: false,
    },
    {
      label: 'go-licenses report fails',
      opts: () => ({ reportRc: 1 }),
      status: 1,
      message: 'go-licenses report failed',
      checkerRan: true,
    },
    {
      label: 'go-licenses save fails after writing part of the tree',
      opts: () => ({ saveRc: 1 }),
      status: 1,
      message: 'go-licenses save failed',
      checkerRan: true,
    },
    {
      label: 'the report names no libraries',
      opts: () => ({ libraries: [] }),
      status: 1,
      message: 'go-licenses report named no libraries',
      checkerRan: true,
    },
    {
      label: 'a library has no license URL',
      opts: () => ({
        libraries: backendLikeLibraries().map((l) => (l.name === uuid ? { ...l, url: 'Unknown' } : l)),
      }),
      status: 1,
      message: `${uuid} has no license URL`,
      checkerRan: true,
    },
    {
      label: 'save wrote nothing for a library in the report',
      opts: () => ({ unsaved: [uuid] }),
      status: 1,
      message: `go-licenses save wrote no licenses/${uuid}`,
      checkerRan: true,
    },
    {
      label: 'an MPL-2.0 library ships without its source',
      opts: () => ({ withoutSource: [hcl] }),
      status: 1,
      message: `${hcl} is MPL-2.0 but licenses/${hcl} holds none of its source`,
      checkerRan: true,
    },
    {
      label: 'an allowlisted MPL-2.0 library is missing from the report',
      opts: () => ({ libraries: backendLikeLibraries().filter((l) => l.name !== hcl) }),
      status: 1,
      message: `allowlisted ${hcl} appears 0 times in the manifest, not once`,
      checkerRan: true,
    },
    {
      label: 'an allowlisted library is reported under another license',
      opts: () => ({
        libraries: backendLikeLibraries().map((l) => (l.name === hcl ? { ...l, license: 'MIT' } : l)),
      }),
      status: 1,
      message: `allowlisted ${hcl} is MIT in the manifest, not the ruled MPL-2.0`,
      checkerRan: true,
    },
  ];

  for (const refusal of refusals) {
    it(`fails the build and leaves no notices behind when ${refusal.label}`, () => {
      const run = runNotices(refusal.opts());
      expect(run.status).toBe(refusal.status);
      expect(run.stderr).toContain(refusal.message);
      expect(run.stdout).not.toContain('third-party notices:');
      expect(run.partialExists).toBe(false);
      if (refusal.label === 'the output directory already exists') {
        expect(run.outFiles).toEqual([]);
      } else {
        expect(run.outFiles).toBeNull();
      }
      const checkerRuns = run.log.filter((line) => line.startsWith(join(run.gobin, 'go-licenses')));
      if (!refusal.checkerRan) expect(checkerRuns).toEqual([]);
      expect(run.log.some((line) => line.includes(`${join('stub', 'bin', 'go-licenses')} `))).toBe(false);
    });
  }
});
