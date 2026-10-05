import { describe, it, expect } from 'vitest';
import { existsSync, readFileSync, readdirSync } from 'fs';
import { join, relative, sep } from 'path';
import { load } from 'js-yaml';

/**
 * The backend image has one Dockerfile: the one docker-publish.yml builds.
 *
 * A second backend Dockerfile under apps/backend/infrastructure/docker/ rooted
 * its runtime stage on a floating `alpine:latest`, built the server with cgo,
 * stamped no version, and was the file scripts/test-quickstart.sh built. The
 * quickstart test therefore exercised an image no release ships, from base
 * bytes that could change without a commit. That copy is removed and the
 * quickstart test builds the published file. This cell keeps it that way:
 *
 *   - every Dockerfile in the tree that builds ./cmd/server is the file
 *     docker-publish.yml builds for the backend image;
 *   - scripts/test-quickstart.sh builds that file, and every Dockerfile a
 *     compose file in the tree builds exists;
 *   - no FROM in any Dockerfile in the tree takes a floating base: a reference
 *     with no tag, or with the tag `latest`, and no digest.
 *
 * The checks also run over literal fixture texts, including the removed file,
 * so they keep refusing that shape after the tree stops carrying it.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');
const PUBLISH_WORKFLOW = join(REPO_ROOT, '.github', 'workflows', 'docker-publish.yml');
const QUICKSTART_SCRIPT = join(REPO_ROOT, 'scripts', 'test-quickstart.sh');

// ---------------------------------------------------------------------------
// The checks
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
    const trimmed = lines[i].trim();
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

/**
 * Refuses every FROM whose base floats: no digest, and either no tag or the
 * tag `latest`. A base named through an ARG is resolved against the ARG's
 * default declared before the first FROM; an ARG with no default is refused,
 * since the build would float to whatever the caller passes or to nothing.
 * References to an earlier stage and `scratch` are not images and pass.
 */
function floatingBases(text: string): string[] {
  const failures: string[] = [];
  const globalArgs = new Map<string, string | undefined>();
  const stages = new Set<string>();
  let seenFrom = false;
  for (const ins of parseInstructions(text)) {
    if (ins.keyword === 'ARG' && !seenFrom) {
      for (const decl of ins.args.split(/\s+/).filter(Boolean)) {
        const eq = decl.indexOf('=');
        if (eq < 0) globalArgs.set(decl, undefined);
        else globalArgs.set(decl.slice(0, eq), decl.slice(eq + 1).replace(/^["']|["']$/g, ''));
      }
      continue;
    }
    if (ins.keyword !== 'FROM') continue;
    seenFrom = true;
    const tokens = ins.args.split(/\s+/).filter((t) => t !== '' && !t.startsWith('--'));
    const written = tokens[0] ?? '';
    const earlierStages = new Set(stages);
    if (tokens.length >= 3 && tokens[1].toLowerCase() === 'as') stages.add(tokens[2].toLowerCase());

    let unresolved: string | null = null;
    const ref = written.replace(/\$\{([A-Za-z_][A-Za-z0-9_]*)\}|\$([A-Za-z_][A-Za-z0-9_]*)/g, (_, braced, bare) => {
      const name = (braced ?? bare) as string;
      const value = globalArgs.get(name);
      if (value === undefined || value === '') {
        unresolved = name;
        return '';
      }
      return value;
    });
    if (unresolved !== null) {
      failures.push(`line ${ins.line}: FROM ${written} names its base through ARG ${unresolved}, which declares no default`);
      continue;
    }

    const lowered = ref.toLowerCase();
    if (lowered === 'scratch' || earlierStages.has(lowered)) continue;
    if (ref.includes('@')) continue;
    const afterRegistry = ref.slice(ref.lastIndexOf('/') + 1);
    const colon = afterRegistry.indexOf(':');
    const tag = colon < 0 ? '' : afterRegistry.slice(colon + 1);
    if (tag === '') {
      failures.push(`line ${ins.line}: FROM ${written} takes no tag and no digest, so the base floats to latest`);
    } else if (tag === 'latest') {
      failures.push(`line ${ins.line}: FROM ${written} takes the floating tag latest and no digest`);
    }
  }
  return failures;
}

/** True when a RUN in the Dockerfile compiles the aim-server package. */
function buildsServer(text: string): boolean {
  return parseInstructions(text).some(
    (ins) => ins.keyword === 'RUN' && /\bgo\s+build\b/.test(ins.args) && /(^|\s)\.\/cmd\/server(\s|$)/.test(ins.args),
  );
}

/** Dockerfile paths a shell script passes to `-f` / `--file`, with a leading `$VAR/` stripped. */
function scriptDockerfiles(script: string): string[] {
  const out: string[] = [];
  const pattern = /(?:^|\s)(?:-f|--file)[\s=]+["']?([^\s"']*Dockerfile[^\s"']*)["']?/g;
  let match: RegExpExecArray | null = pattern.exec(script);
  while (match !== null) {
    out.push(match[1].replace(/^\$\{?[A-Za-z_][A-Za-z0-9_]*\}?\//, ''));
    match = pattern.exec(script);
  }
  return out;
}

interface PublishWorkflow {
  jobs?: Record<string, { steps?: Array<{ uses?: string; with?: { file?: string } }> }>;
}

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

function isDockerfileName(name: string): boolean {
  return /^Dockerfile(\..+)?$/.test(name) || name.endsWith('.Dockerfile');
}

function walk(root: string, accept: (name: string) => boolean): string[] {
  const found: string[] = [];
  const pending = [root];
  while (pending.length > 0) {
    const directory = pending.pop() as string;
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      if (entry.isDirectory()) {
        if (entry.name === 'node_modules' || entry.name === '.git') continue;
        pending.push(join(directory, entry.name));
      } else if (entry.isFile() && accept(entry.name)) {
        found.push(join(directory, entry.name));
      }
    }
  }
  return found.sort();
}

function repoRelative(absolute: string): string {
  return relative(REPO_ROOT, absolute).split(sep).join('/');
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

/** The second backend Dockerfile as it stood when scripts/test-quickstart.sh built it. */
const REMOVED_COPY = `# Multi-stage build for Go backend
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Set working directory
WORKDIR /build

# Copy go mod files
COPY apps/backend/go.mod apps/backend/go.sum ./
RUN go mod download

# Copy source code
COPY apps/backend/ ./

# Build the application, stamping the git commit for GET /health/ready
# (the deploy passes --build-arg GIT_COMMIT=<sha>; empty means commit: null)
ARG GIT_COMMIT
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -ldflags "-X main.buildCommit=$GIT_COMMIT" -o aim-server ./cmd/server

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata wget
`;

/** The stages of the published backend Dockerfile. */
const PUBLISHED_SHAPE = `FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /app
COPY apps/backend/ ./
ARG TARGETARCH
ARG VERSION=dev
ARG GIT_COMMIT
RUN CGO_ENABLED=0 GOOS=linux GOARCH=\${TARGETARCH} go build \\
    -ldflags="-s -w -X main.buildCommit=\${GIT_COMMIT} -X main.buildVersion=\${VERSION}" \\
    -o /bin/aim-server ./cmd/server
FROM alpine:3.21
RUN apk --no-cache add ca-certificates
COPY --from=builder /bin/aim-server ./
`;

const DIGEST = 'sha256:ce64758a109eb420d874a118f87920e625e12d3634e03b4a5573fd9f6e5d3507';

// ---------------------------------------------------------------------------

describe('the backend image is built from the one Dockerfile docker-publish.yml builds', () => {
  it('the floating-base check refuses the removed copy at its alpine:latest line', () => {
    expect(floatingBases(REMOVED_COPY)).toEqual([
      'line 23: FROM alpine:latest takes the floating tag latest and no digest',
    ]);
  });

  it('the floating-base check passes the published shape', () => {
    expect(floatingBases(PUBLISHED_SHAPE)).toEqual([]);
  });

  it('the floating-base check refuses each planted floating base', () => {
    const cases: Array<[string, string, string]> = [
      ['an untagged base', 'FROM alpine\n', 'line 1: FROM alpine takes no tag and no digest'],
      [
        'a fully qualified latest',
        'FROM docker.io/library/alpine:latest\n',
        'line 1: FROM docker.io/library/alpine:latest takes the floating tag latest',
      ],
      [
        'an untagged base on a registry with a port',
        'FROM localhost:5000/alpine\n',
        'line 1: FROM localhost:5000/alpine takes no tag and no digest',
      ],
      [
        'a platform flag in front of a latest base',
        'FROM --platform=$BUILDPLATFORM alpine:latest AS base\n',
        'line 1: FROM alpine:latest takes the floating tag latest',
      ],
      [
        'an ARG default that floats',
        'ARG BASE=alpine:latest\nFROM ${BASE}\n',
        'line 2: FROM ${BASE} takes the floating tag latest',
      ],
      [
        'an ARG with no default',
        'ARG BASE\nFROM $BASE\n',
        'line 2: FROM $BASE names its base through ARG BASE, which declares no default',
      ],
      [
        'a runtime stage that floats behind a pinned builder',
        PUBLISHED_SHAPE.replace('FROM alpine:3.21', 'FROM alpine'),
        'line 11: FROM alpine takes no tag and no digest',
      ],
      [
        'a stage named like the image it claims to reuse',
        'FROM alpine AS alpine\n',
        'line 1: FROM alpine takes no tag and no digest',
      ],
    ];
    for (const [label, text, expected] of cases) {
      const failures = floatingBases(text);
      expect(
        failures.some((f) => f.startsWith(expected)),
        `${label}: expected a failure starting ${JSON.stringify(expected)}, got ${JSON.stringify(failures)}`,
      ).toBe(true);
    }
  });

  it('the floating-base check passes digests, tags, stage references and scratch', () => {
    const texts = [
      `FROM alpine:latest@${DIGEST}\n`,
      `FROM alpine@${DIGEST}\n`,
      'FROM localhost:5000/alpine:3.21\n',
      'FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder\nFROM builder AS test\nFROM scratch\n',
      'ARG BASE_IMAGE=hma-vhs:0.33.0\nFROM ${BASE_IMAGE}\n',
    ];
    for (const text of texts) {
      expect(floatingBases(text), text).toEqual([]);
    }
  });

  it('the server-build check recognises the removed copy and the published shape', () => {
    expect(buildsServer(REMOVED_COPY)).toBe(true);
    expect(buildsServer(PUBLISHED_SHAPE)).toBe(true);
    expect(buildsServer('FROM alpine:3.21\nRUN go build -o /bin/aim-migrate ./cmd/migrate\n')).toBe(false);
  });

  it('the script check reads the Dockerfile a docker build names', () => {
    const script = [
      'docker compose -f "$DIR/docker-compose.quickstart.yml" down',
      'docker build -q -t aim-server:test \\',
      '  -f "$REPO/apps/backend/infrastructure/docker/Dockerfile.backend" \\',
      '  "$REPO" >/dev/null',
      'docker build --file=${ROOT}/infrastructure/docker/Dockerfile.frontend .',
    ].join('\n');
    expect(scriptDockerfiles(script)).toEqual([
      'apps/backend/infrastructure/docker/Dockerfile.backend',
      'infrastructure/docker/Dockerfile.frontend',
    ]);
  });

  it('every Dockerfile in the tree that builds ./cmd/server is the file docker-publish.yml builds', () => {
    const published = publishedDockerfile('build-backend');
    expect(published).toBe(publishedDockerfile('verify-backend'));
    const serverBuilds = walk(REPO_ROOT, isDockerfileName)
      .filter((file) => buildsServer(readFileSync(file, 'utf-8')))
      .map(repoRelative);
    expect(serverBuilds).toEqual([published]);
  });

  it('scripts/test-quickstart.sh builds the published backend Dockerfile', () => {
    const named = scriptDockerfiles(readFileSync(QUICKSTART_SCRIPT, 'utf-8'));
    expect(named).toEqual([publishedDockerfile('build-backend')]);
    expect(existsSync(join(REPO_ROOT, named[0])), named[0]).toBe(true);
  });

  it('every Dockerfile a compose file in the tree builds exists', () => {
    const composeFiles = walk(REPO_ROOT, (name) => /compose[^/]*\.ya?ml$/.test(name));
    expect(composeFiles.map(repoRelative)).toContain('docker-compose.yml');
    for (const composeFile of composeFiles) {
      const doc = load(readFileSync(composeFile, 'utf-8')) as {
        services?: Record<string, { build?: string | { context?: string; dockerfile?: string } }>;
      };
      for (const [service, spec] of Object.entries(doc?.services ?? {})) {
        const build = spec?.build;
        if (build === undefined || typeof build === 'string' || build.dockerfile === undefined) continue;
        const file = join(composeFile, '..', build.context ?? '.', build.dockerfile);
        expect(existsSync(file), `${repoRelative(composeFile)} ${service} builds ${build.dockerfile}`).toBe(true);
      }
    }
  });

  it('no Dockerfile in the tree takes a floating base', () => {
    const dockerfiles = walk(REPO_ROOT, isDockerfileName);
    expect(dockerfiles.map(repoRelative)).toContain(publishedDockerfile('build-backend'));
    for (const file of dockerfiles) {
      expect(floatingBases(readFileSync(file, 'utf-8')), repoRelative(file)).toEqual([]);
    }
  });
});
