import { describe, it, expect } from 'vitest';
import { readFileSync, readdirSync } from 'fs';
import { join } from 'path';
import { load } from 'js-yaml';

/**
 * GET /health/ready reports `commit` and `version` from two package-level
 * strings in apps/backend/cmd/server, `buildCommit` and `buildVersion`, which
 * the image build stamps with `go build -ldflags "-X main.<name>=<value>"`.
 * The Go linker silently ignores an -X whose target is not declared, so a
 * Dockerfile that stamps a name the server does not declare still builds, and
 * every image it produces reports `"commit": null` and `"version": null`.
 *
 * This cell holds the aim-server build of the Dockerfile that docker-publish.yml
 * builds for the backend image to the shape that stamps both:
 *
 *   - every `-X main.<name>=` target is a package-level string declared in a
 *     non-test file of apps/backend/cmd/server;
 *   - `main.buildCommit` is stamped from `ARG GIT_COMMIT` and
 *     `main.buildVersion` from `ARG VERSION`, each declared inside the build
 *     stage before that RUN (an ARG declared before FROM is not visible there).
 *
 * The same check runs over literal fixture texts, including the text this
 * Dockerfile carried when it stamped `main.version` and no commit, so it keeps
 * refusing that shape after the tree stops carrying it. A file this cell names
 * and cannot read fails the cell.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');
const SERVER_PKG = join(REPO_ROOT, 'apps', 'backend', 'cmd', 'server');
const PUBLISH_WORKFLOW = join(REPO_ROOT, '.github', 'workflows', 'docker-publish.yml');

/** The stamps /health/ready reads, and the build ARG each must come from. */
const REQUIRED_STAMPS: Record<string, string> = {
  buildCommit: 'GIT_COMMIT',
  buildVersion: 'VERSION',
};

// ---------------------------------------------------------------------------
// The stamp check
// ---------------------------------------------------------------------------

interface Instruction {
  keyword: string;
  /** 1-indexed line the instruction starts on. */
  line: number;
  /** Arguments with line continuations joined. */
  args: string;
}

/** Splits a Dockerfile into instructions, joining `\` continuations and skipping comments. */
function parseInstructions(text: string): Instruction[] {
  const out: Instruction[] = [];
  const lines = text.split('\n');
  let current: Instruction | null = null;
  for (let i = 0; i < lines.length; i++) {
    const raw = lines[i];
    const trimmed = raw.trim();
    if (current === null) {
      if (trimmed === '' || trimmed.startsWith('#')) continue;
      const m = /^(\S+)\s*(.*)$/.exec(trimmed) as RegExpExecArray;
      current = { keyword: m[1].toUpperCase(), line: i + 1, args: m[2] };
    } else {
      if (trimmed.startsWith('#')) continue;
      current.args += ' ' + trimmed;
    }
    if (current.args.endsWith('\\')) {
      current.args = current.args.slice(0, -1).trimEnd();
      continue;
    }
    out.push(current);
    current = null;
  }
  if (current !== null) out.push(current);
  return out;
}

/** Package-level `string` variables declared in Go source text (gofmt layout). */
function declaredStrings(goSource: string): Set<string> {
  const names = new Set<string>();
  const lines = goSource.split('\n');
  let inBlock = false;
  for (const line of lines) {
    if (inBlock) {
      if (/^\)/.test(line)) {
        inBlock = false;
        continue;
      }
      const m = /^\s+(\w+)\s+string\b/.exec(line);
      if (m) names.add(m[1]);
      continue;
    }
    if (/^var\s*\(\s*$/.test(line)) {
      inBlock = true;
      continue;
    }
    const m = /^var\s+(\w+)\s+string\b/.exec(line);
    if (m) names.add(m[1]);
  }
  return names;
}

/** The value of `-ldflags` in a `go build` command line. */
function ldflagsOf(args: string): string | null {
  const m = /-ldflags(?:=|\s+)(?:"([^"]*)"|'([^']*)'|(\S+))/.exec(args);
  if (!m) return null;
  return m[1] ?? m[2] ?? m[3];
}

/**
 * Failures of the aim-server build in `dockerfile` against the package-level
 * strings `declared` in cmd/server; empty when the build stamps both values.
 */
function checkServerStamp(dockerfile: string, declared: Set<string>): string[] {
  const failures: string[] = [];
  const instructions = parseInstructions(dockerfile);

  let stageArgs = new Set<string>();
  let build: Instruction | null = null;
  let argsBeforeBuild = new Set<string>();
  for (const ins of instructions) {
    if (ins.keyword === 'FROM') {
      stageArgs = new Set<string>();
      continue;
    }
    if (ins.keyword === 'ARG') {
      const name = /^(\w+)/.exec(ins.args)?.[1];
      if (name) stageArgs.add(name);
      continue;
    }
    if (ins.keyword === 'RUN' && /\bgo build\b/.test(ins.args) && /\.\/cmd\/server\b/.test(ins.args)) {
      if (build !== null) {
        failures.push(`line ${ins.line}: a second go build of ./cmd/server; the stamp check needs exactly one`);
        continue;
      }
      build = ins;
      argsBeforeBuild = new Set(stageArgs);
    }
  }
  if (build === null) {
    failures.push('no RUN instruction runs go build of ./cmd/server');
    return failures;
  }

  const ldflags = ldflagsOf(build.args);
  if (ldflags === null) {
    failures.push(`line ${build.line}: the ./cmd/server build passes no -ldflags`);
    return failures;
  }

  const stamps = new Map<string, string>();
  for (const m of ldflags.matchAll(/-X(?:=|\s+)main\.(\w+)=(\S*)/g)) {
    stamps.set(m[1], m[2]);
    if (!declared.has(m[1])) {
      failures.push(
        `line ${build.line}: -X main.${m[1]} targets no package-level string in apps/backend/cmd/server, so the linker drops it`,
      );
    }
  }

  for (const [name, arg] of Object.entries(REQUIRED_STAMPS)) {
    if (!declared.has(name)) {
      failures.push(`apps/backend/cmd/server declares no package-level string ${name}`);
    }
    const value = stamps.get(name);
    if (value === undefined) {
      failures.push(`line ${build.line}: the ./cmd/server build does not stamp -X main.${name}`);
      continue;
    }
    if (value !== `\${${arg}}` && value !== `$${arg}`) {
      failures.push(`line ${build.line}: -X main.${name} is stamped from ${JSON.stringify(value)}, not from ARG ${arg}`);
    }
    if (!argsBeforeBuild.has(arg)) {
      failures.push(`line ${build.line}: ARG ${arg} is not declared in the build stage before the ./cmd/server build`);
    }
  }
  return failures;
}

// ---------------------------------------------------------------------------
// Tree reads
// ---------------------------------------------------------------------------

function serverDeclaredStrings(): Set<string> {
  const files = readdirSync(SERVER_PKG).filter((f) => f.endsWith('.go') && !f.endsWith('_test.go'));
  expect(files.length, 'apps/backend/cmd/server has Go source files').toBeGreaterThan(0);
  const names = new Set<string>();
  for (const f of files) {
    for (const n of declaredStrings(readFileSync(join(SERVER_PKG, f), 'utf-8'))) names.add(n);
  }
  return names;
}

interface WorkflowStep {
  uses?: string;
  with?: Record<string, unknown>;
}

interface PublishWorkflow {
  jobs?: Record<string, { steps?: WorkflowStep[] }>;
}

/** The `file:` of the one docker/build-push-action step of a docker-publish.yml job. */
function publishedDockerfile(job: string): string {
  const workflow = load(readFileSync(PUBLISH_WORKFLOW, 'utf-8')) as PublishWorkflow;
  const builds = (workflow.jobs?.[job]?.steps ?? []).filter(
    (s) => typeof s.uses === 'string' && s.uses.startsWith('docker/build-push-action'),
  );
  expect(builds.length, `docker-publish.yml ${job} carries exactly one docker/build-push-action step`).toBe(1);
  const file = builds[0].with?.file;
  expect(typeof file, `docker-publish.yml ${job} names its Dockerfile with file:`).toBe('string');
  return file as string;
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const DECLARED = new Set(['buildCommit', 'buildVersion']);

/** The build stage as it stood when it stamped main.version and no commit. */
const BEFORE_FIX = `FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY apps/backend/ ./
ARG TARGETARCH
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=\${TARGETARCH} go build \\
    -ldflags="-s -w -X main.version=\${VERSION}" \\
    -o /bin/aim-server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=\${TARGETARCH} go build \\
    -ldflags="-s -w" \\
    -o /bin/aim-migrate ./cmd/migrate
FROM alpine:3.21
COPY --from=builder /bin/aim-server ./
`;

const REPAIRED = `FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY apps/backend/ ./
ARG TARGETARCH
ARG VERSION=dev
ARG GIT_COMMIT
RUN CGO_ENABLED=0 GOOS=linux GOARCH=\${TARGETARCH} go build \\
    -ldflags="-s -w -X main.buildCommit=\${GIT_COMMIT} -X main.buildVersion=\${VERSION}" \\
    -o /bin/aim-server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=\${TARGETARCH} go build \\
    -ldflags="-s -w" \\
    -o /bin/aim-migrate ./cmd/migrate
FROM alpine:3.21
COPY --from=builder /bin/aim-server ./
`;

// ---------------------------------------------------------------------------

describe('the published backend image stamps the commit and version /health/ready reports', () => {
  it('the check refuses the Dockerfile text that stamped main.version and no commit', () => {
    const failures = checkServerStamp(BEFORE_FIX, DECLARED);
    expect(failures).toEqual([
      'line 6: -X main.version targets no package-level string in apps/backend/cmd/server, so the linker drops it',
      'line 6: the ./cmd/server build does not stamp -X main.buildCommit',
      'line 6: the ./cmd/server build does not stamp -X main.buildVersion',
    ]);
  });

  it('the check passes the repaired text', () => {
    expect(checkServerStamp(REPAIRED, DECLARED)).toEqual([]);
  });

  it('the check refuses each planted regression of the repaired text', () => {
    const cases: Array<[string, string, string]> = [
      [
        'a misspelt target',
        REPAIRED.replace('main.buildCommit=', 'main.buildcommit='),
        '-X main.buildcommit targets no package-level string',
      ],
      [
        'a commit not taken from the build arg',
        REPAIRED.replace('main.buildCommit=${GIT_COMMIT}', 'main.buildCommit=unknown'),
        '-X main.buildCommit is stamped from "unknown", not from ARG GIT_COMMIT',
      ],
      [
        'no ARG GIT_COMMIT',
        REPAIRED.replace('ARG GIT_COMMIT\n', ''),
        'ARG GIT_COMMIT is not declared in the build stage',
      ],
      [
        'ARG GIT_COMMIT declared after the build',
        REPAIRED.replace('ARG GIT_COMMIT\n', '').replace('FROM alpine', 'ARG GIT_COMMIT\nFROM alpine'),
        'ARG GIT_COMMIT is not declared in the build stage',
      ],
      [
        'ARG GIT_COMMIT declared only before FROM',
        'ARG GIT_COMMIT\n' + REPAIRED.replace('ARG GIT_COMMIT\n', ''),
        'ARG GIT_COMMIT is not declared in the build stage',
      ],
      [
        'the version stamp removed',
        REPAIRED.replace(' -X main.buildVersion=${VERSION}', ''),
        'does not stamp -X main.buildVersion',
      ],
    ];
    for (const [label, text, expected] of cases) {
      const failures = checkServerStamp(text, DECLARED);
      expect(
        failures.some((f) => f.includes(expected)),
        `${label}: expected a failure containing ${JSON.stringify(expected)}, got ${JSON.stringify(failures)}`,
      ).toBe(true);
    }
  });

  it('the check refuses a stamp the server package does not declare', () => {
    expect(checkServerStamp(REPAIRED, new Set(['buildCommit']))).toContain(
      'line 7: -X main.buildVersion targets no package-level string in apps/backend/cmd/server, so the linker drops it',
    );
  });

  it('declaredStrings reads single and grouped package-level declarations and skips function bodies', () => {
    const src = [
      'package main',
      '',
      'var buildCommit string',
      '',
      'var (',
      '\tbuildVersion string',
      '\tother        int',
      ')',
      '',
      'func f() {',
      '\tvar local string',
      '\t_ = local',
      '}',
    ].join('\n');
    expect([...declaredStrings(src)].sort()).toEqual(['buildCommit', 'buildVersion']);
  });

  it('cmd/server declares the two stamps /health/ready reads', () => {
    const declared = serverDeclaredStrings();
    for (const name of Object.keys(REQUIRED_STAMPS)) {
      expect(declared.has(name), `apps/backend/cmd/server declares var ${name} string`).toBe(true);
    }
  });

  it('the Dockerfile docker-publish.yml builds for the backend image stamps both', () => {
    const files = new Set([publishedDockerfile('build-backend'), publishedDockerfile('verify-backend')]);
    const declared = serverDeclaredStrings();
    for (const file of files) {
      const text = readFileSync(join(REPO_ROOT, file), 'utf-8');
      expect(checkServerStamp(text, declared), file).toEqual([]);
    }
  });
});
