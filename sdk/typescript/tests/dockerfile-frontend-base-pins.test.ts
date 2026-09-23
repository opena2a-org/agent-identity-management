import { describe, it, expect } from 'vitest';
import { readFileSync, readdirSync } from 'fs';
import { join, relative, sep } from 'path';
import { load } from 'js-yaml';

/**
 * QGF-230 — the base-image unit of the AIM image advisory.
 *
 * The released aim-dashboard runtime image inherited the npm CLI's tar and the
 * alpine OpenSSL pair from a floating `node:20-alpine`: all 20 node-pkg
 * findings sat under `usr/local/lib/node_modules/npm/`, a component the
 * dashboard never invokes and no lockfile clears. The fix is structural — the
 * runtime stage roots on the official alpine image and the node binary reaches
 * it by a COPY from the pinned builder of the SAME alpine release — so this
 * cell is the tree-side refusal that keeps the published bytes that shape:
 *
 *   AC1  infrastructure/docker/Dockerfile.frontend (the file docker-publish.yml
 *        builds) is pinned, alpine-rooted, copies the node binary from a pinned
 *        official-node builder, runs non-root and keeps CMD ["node","server.js"];
 *   AC2  apps/backend/infrastructure/docker/Dockerfile.frontend, the twin, same;
 *   AC3  the check itself, over fixture texts: the two base texts are refused by
 *        line, the repaired text passes, and each planted regression is refused
 *        at the line its instruction keyword sits on;
 *   AC4  every Dockerfile.frontend the tree walk finds passes, the population
 *        includes the workflow's `file:`, and the build step names no `target`
 *        and no `build-contexts` — so the published image is the final stage of
 *        that file and no stage is remapped to an image outside it.
 *
 * The AC3 fixtures are literal texts, not reads of the tree: the check must keep
 * refusing the pre-fix shape long after the tree stops carrying it.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');

// ---------------------------------------------------------------------------
// The base-pin check (QGF-230.AC3)
// ---------------------------------------------------------------------------

interface Failure {
  line: number;
  rule: string;
  detail: string;
}

interface Instruction {
  keyword: string;
  /** 1-indexed line the instruction keyword sits on. */
  line: number;
  /** Full text, keyword to the line before the next instruction keyword. */
  text: string;
  /** `text` with the leading keyword removed. */
  args: string;
}

interface Stage {
  index: number;
  /** Lower-cased `AS <name>`, absent for an unnamed stage. */
  name?: string;
  base: string;
  line: number;
}

const INSTRUCTION_KEYWORDS = new Set([
  'ADD',
  'ARG',
  'CMD',
  'COPY',
  'ENTRYPOINT',
  'ENV',
  'EXPOSE',
  'FROM',
  'HEALTHCHECK',
  'LABEL',
  'MAINTAINER',
  'ONBUILD',
  'RUN',
  'SHELL',
  'STOPSIGNAL',
  'USER',
  'VOLUME',
  'WORKDIR',
]);

/** Commands the runtime stage may run. An allowlist, never a denylist. */
const RUNTIME_ALLOWED_COMMANDS = new Set([
  'addgroup',
  'adduser',
  'chown',
  'chmod',
  'mkdir',
  'ln',
  'apk',
]);

/**
 * Tokens that may not appear anywhere in a runtime-stage RUN. Deletion and
 * replacement of lower-layer bytes leave the vulnerable bytes in a pullable
 * layer of the released digest; the shells and the package managers are the
 * wrappers that would otherwise smuggle them past the first-token allowlist.
 */
const RUNTIME_FORBIDDEN_TOKENS = new Set([
  'rm',
  'rmdir',
  'unlink',
  'find',
  'npm',
  'npx',
  'corepack',
  'yarn',
  'pnpm',
  'sh',
  'ash',
  'bash',
  'eval',
  'exec',
  'xargs',
]);

const RUNTIME_ALLOWED_APK_PACKAGES = new Set(['libstdc++', 'libgcc']);

const OFFICIAL_ALPINE_REPOSITORIES = new Set([
  'alpine',
  'library/alpine',
  'docker.io/alpine',
  'index.docker.io/alpine',
  'registry-1.docker.io/alpine',
]);

const OFFICIAL_NODE_REPOSITORIES = new Set([
  'node',
  'library/node',
  'docker.io/node',
  'index.docker.io/node',
  'registry-1.docker.io/node',
]);

function opensHeredocs(line: string): string[] {
  const delimiters: string[] = [];
  const pattern = /<<-?\s*(['"]?)([A-Za-z_][A-Za-z0-9_]*)\1/g;
  let match: RegExpExecArray | null = pattern.exec(line);
  while (match !== null) {
    delimiters.push(match[2]);
    match = pattern.exec(line);
  }
  return delimiters;
}

function isContinued(line: string): boolean {
  return /\\\s*$/.test(line);
}

/**
 * An instruction runs from its keyword to the next instruction keyword: comment
 * and blank lines between instructions are skipped, backslash continuations and
 * BuildKit heredoc bodies are part of the instruction they belong to.
 */
function parseInstructions(text: string): Instruction[] {
  const lines = text.split('\n');
  const instructions: Instruction[] = [];
  let i = 0;
  while (i < lines.length) {
    const trimmed = lines[i].trim();
    if (trimmed === '' || trimmed.startsWith('#')) {
      i += 1;
      continue;
    }
    const opener = /^\s*([A-Za-z][A-Za-z0-9_]*)(?:\s|$)/.exec(lines[i]);
    const keyword = opener ? opener[1].toUpperCase() : '';
    if (!INSTRUCTION_KEYWORDS.has(keyword)) {
      i += 1;
      continue;
    }
    const start = i;
    let end = i;
    let heredocs = opensHeredocs(lines[i]);
    let continued = isContinued(lines[i]);
    while ((continued || heredocs.length > 0) && end + 1 < lines.length) {
      end += 1;
      if (heredocs.length > 0) {
        if (lines[end].trim() === heredocs[0]) heredocs.shift();
        if (heredocs.length === 0) continued = false;
        continue;
      }
      continued = isContinued(lines[end]);
      heredocs = opensHeredocs(lines[end]);
    }
    const body = lines.slice(start, end + 1);
    const withoutKeyword = [
      body[0].replace(/^\s*[A-Za-z][A-Za-z0-9_]*/, ''),
      ...body.slice(1),
    ];
    instructions.push({
      keyword,
      line: start + 1,
      text: body.join('\n'),
      args: withoutKeyword.join('\n'),
    });
    i = end + 1;
  }
  return instructions;
}

/** `FROM [--platform=<value>] <base> [AS <name>]`, the name case-insensitive. */
function parseStages(instructions: Instruction[]): Stage[] {
  const stages: Stage[] = [];
  for (const instruction of instructions) {
    if (instruction.keyword !== 'FROM') continue;
    const tokens = instruction.args.split(/\s+/).filter((t) => t.length > 0);
    let at = 0;
    while (at < tokens.length && tokens[at].startsWith('--')) at += 1;
    const base = tokens[at] ?? '';
    let name: string | undefined;
    if (
      tokens[at + 1] !== undefined &&
      tokens[at + 1].toUpperCase() === 'AS' &&
      tokens[at + 2] !== undefined
    ) {
      name = tokens[at + 2].toLowerCase();
    }
    stages.push({ index: stages.length, name, base, line: instruction.line });
  }
  return stages;
}

/** The index of the stage an earlier `AS <name>` declared, or -1. */
function earlierStageNamed(stages: Stage[], name: string, before: number): number {
  const wanted = name.toLowerCase();
  for (let i = before - 1; i >= 0; i -= 1) {
    if (stages[i].name !== undefined && stages[i].name === wanted) return i;
  }
  return -1;
}

function isStageReference(stages: Stage[], index: number): boolean {
  return earlierStageNamed(stages, stages[index].base, index) !== -1;
}

/** Follow stage references to the first non-stage base. */
function rootImageOf(stages: Stage[], index: number): string {
  let current = index;
  const seen = new Set<number>();
  for (;;) {
    if (seen.has(current)) return stages[current].base;
    seen.add(current);
    const referenced = earlierStageNamed(stages, stages[current].base, current);
    if (referenced === -1) return stages[current].base;
    current = referenced;
  }
}

/**
 * `<repository>:<tag>@sha256:` + exactly 64 lowercase hex, a non-empty tag and
 * no `$` anywhere. Returns why the reference is refused, or null.
 */
function pinRefusal(reference: string): string | null {
  if (reference === '') return 'the FROM instruction names no base';
  if (reference.includes('$')) {
    return `base "${reference}" interpolates a variable`;
  }
  const at = reference.indexOf('@');
  if (at < 0) return `base "${reference}" carries no @sha256: digest`;
  const digest = reference.slice(at + 1);
  if (!/^sha256:[0-9a-f]{64}$/.test(digest)) {
    return `base "${reference}" does not end in @sha256: plus exactly 64 lowercase hex characters`;
  }
  const name = reference.slice(0, at);
  const colon = name.indexOf(':', name.lastIndexOf('/') + 1);
  if (colon < 0) return `base "${reference}" carries no tag`;
  if (name.slice(colon + 1) === '') return `base "${reference}" carries an empty tag`;
  return null;
}

/** The reference with `@sha256:<digest>` and then the tag after the last `/` removed. */
function repositoryOf(reference: string): string {
  const at = reference.indexOf('@');
  const name = at >= 0 ? reference.slice(0, at) : reference;
  const colon = name.indexOf(':', name.lastIndexOf('/') + 1);
  return colon >= 0 ? name.slice(0, colon) : name;
}

function tagOf(reference: string): string | null {
  const at = reference.indexOf('@');
  const name = at >= 0 ? reference.slice(0, at) : reference;
  const colon = name.indexOf(':', name.lastIndexOf('/') + 1);
  return colon >= 0 ? name.slice(colon + 1) : null;
}

function isOfficialAlpine(repository: string): boolean {
  return (
    OFFICIAL_ALPINE_REPOSITORIES.has(repository) ||
    repository.endsWith('/library/alpine')
  );
}

function isOfficialNode(repository: string): boolean {
  return (
    OFFICIAL_NODE_REPOSITORIES.has(repository) || repository.endsWith('/library/node')
  );
}

/** Surrounding quotes and any leading directory path removed: `/bin/rm` is `rm`. */
function normalizeToken(token: string): string {
  const unquoted = token.replace(/^['"]+/, '').replace(/['"]+$/, '');
  const slash = unquoted.lastIndexOf('/');
  return slash >= 0 ? unquoted.slice(slash + 1) : unquoted;
}

/**
 * An exec-form JSON array is one command; a shell-form text is split on `&&`,
 * `||`, `;`, `|` and on newlines, so a heredoc body is read line by line.
 */
function runCommands(instruction: Instruction): string[][] {
  const trimmed = instruction.args.trim();
  if (trimmed.startsWith('[')) {
    try {
      const parsed: unknown = JSON.parse(trimmed);
      if (Array.isArray(parsed)) return [parsed.map((entry) => String(entry))];
    } catch {
      // Not a well-formed exec form; fall through and read it as shell form.
    }
  }
  const pieces: string[] = [];
  for (const physical of instruction.args.split('\n')) {
    for (const piece of physical.replace(/\\\s*$/, '').split(/&&|\|\||;|\|/)) {
      pieces.push(piece);
    }
  }
  return pieces
    .map((piece) => piece.trim())
    .filter((piece) => piece.length > 0)
    .map((piece) => piece.split(/\s+/).filter((token) => token.length > 0));
}

function copyParts(instruction: Instruction): {
  from: string | null;
  sources: string[];
  destination: string | null;
} {
  const tokens = instruction.args.split(/\s+/).filter((t) => t.length > 0);
  const flags = tokens.filter((t) => t.startsWith('--'));
  const positional = tokens.filter((t) => !t.startsWith('--'));
  const fromFlag = flags.find((f) => f.toLowerCase().startsWith('--from='));
  return {
    from: fromFlag === undefined ? null : fromFlag.slice(fromFlag.indexOf('=') + 1),
    sources: positional.slice(0, -1),
    destination: positional.length > 0 ? positional[positional.length - 1] : null,
  };
}

function checkRuntimeRun(
  instruction: Instruction,
  failures: Failure[],
): void {
  const line = instruction.line;
  if (instruction.text.includes('$')) {
    failures.push({
      line,
      rule: 'runtime-run-variable',
      detail: 'a runtime-stage RUN may contain no $',
    });
  }
  if (instruction.text.includes('--mount')) {
    failures.push({
      line,
      rule: 'runtime-run-mount',
      detail: 'a runtime-stage RUN may carry no --mount',
    });
  }
  for (const command of runCommands(instruction)) {
    const normalized = command.map(normalizeToken);
    for (const token of normalized) {
      if (RUNTIME_FORBIDDEN_TOKENS.has(token)) {
        failures.push({
          line,
          rule: 'runtime-run-forbidden-token',
          detail: `token "${token}" may not appear in a runtime-stage RUN`,
        });
      }
    }
    let at = 0;
    while (at < command.length && /^[A-Za-z_][A-Za-z0-9_]*=/.test(command[at])) at += 1;
    if (at >= command.length) continue;
    const head = normalized[at];
    if (!RUNTIME_ALLOWED_COMMANDS.has(head)) {
      failures.push({
        line,
        rule: 'runtime-run-not-allowlisted',
        detail: `command "${head}" is outside the runtime-stage allowlist`,
      });
      continue;
    }
    if (head !== 'apk') continue;
    const rest = normalized.slice(at + 1);
    if (rest[0] !== 'add') {
      failures.push({
        line,
        rule: 'runtime-apk-not-add',
        detail: `apk ${rest[0] ?? '<nothing>'} is refused; the runtime stage may only apk add`,
      });
      continue;
    }
    for (const token of rest.slice(1)) {
      if (token.startsWith('-')) {
        if (token !== '--no-cache') {
          failures.push({
            line,
            rule: 'runtime-apk-flag',
            detail: `apk add flag "${token}" is refused; only --no-cache is allowed`,
          });
        }
      } else if (!RUNTIME_ALLOWED_APK_PACKAGES.has(token)) {
        failures.push({
          line,
          rule: 'runtime-apk-package',
          detail: `apk add package "${token}" is refused; only libstdc++ and libgcc are allowed`,
        });
      }
    }
  }
}

function checkRuntimeCopy(
  instruction: Instruction,
  stages: Stage[],
  finalIndex: number,
  failures: Failure[],
): void {
  const { from, sources } = copyParts(instruction);
  if (from === null) {
    failures.push({
      line: instruction.line,
      rule: 'runtime-copy-no-from',
      detail: 'every runtime-stage COPY must carry --from=<earlier stage>',
    });
  } else if (earlierStageNamed(stages, from, finalIndex) === -1) {
    failures.push({
      line: instruction.line,
      rule: 'runtime-copy-from-not-a-stage',
      detail: `COPY --from=${from} names no earlier stage of this file`,
    });
  }
  for (const source of sources) {
    if (source !== '/usr/local/bin/node' && !source.startsWith('/app/')) {
      failures.push({
        line: instruction.line,
        rule: 'runtime-copy-source',
        detail: `COPY source "${source}" is neither /usr/local/bin/node nor under /app/`,
      });
    }
  }
}

/**
 * Refuse, by line: an unpinned, untagged or variable base; a runtime stage whose
 * root is not the official alpine image; and any runtime-stage instruction
 * outside the allowlist.
 */
function checkDockerfile(text: string): Failure[] {
  const failures: Failure[] = [];
  const instructions = parseInstructions(text);
  const stages = parseStages(instructions);
  if (stages.length === 0) {
    return [{ line: 1, rule: 'no-from', detail: 'the file declares no FROM instruction' }];
  }

  for (let i = 0; i < stages.length; i += 1) {
    if (isStageReference(stages, i)) continue;
    const refusal = pinRefusal(stages[i].base);
    if (refusal !== null) {
      failures.push({ line: stages[i].line, rule: 'unpinned-base', detail: refusal });
    }
  }

  const finalIndex = stages.length - 1;
  const finalFromLine = stages[finalIndex].line;
  const runtimeRoot = rootImageOf(stages, finalIndex);
  if (!isOfficialAlpine(repositoryOf(runtimeRoot))) {
    failures.push({
      line: finalFromLine,
      rule: 'runtime-root-not-alpine',
      detail: `the runtime stage roots on "${runtimeRoot}", which is not the official alpine image`,
    });
  }

  const runtimeInstructions = instructions.filter((i) => i.line >= finalFromLine);
  const users: Instruction[] = [];
  for (const instruction of runtimeInstructions) {
    switch (instruction.keyword) {
      case 'RUN':
        checkRuntimeRun(instruction, failures);
        break;
      case 'COPY':
        checkRuntimeCopy(instruction, stages, finalIndex, failures);
        break;
      case 'ADD':
        failures.push({
          line: instruction.line,
          rule: 'runtime-add',
          detail: 'the runtime stage may carry no ADD instruction',
        });
        break;
      case 'USER':
        users.push(instruction);
        break;
      default:
        break;
    }
  }

  if (users.length === 0) {
    failures.push({
      line: finalFromLine,
      rule: 'runtime-user-missing',
      detail: 'the runtime stage carries no USER instruction',
    });
  } else {
    const last = users[users.length - 1];
    const named = (last.args.trim().split(/\s+/)[0] ?? '').split(':')[0];
    if (named === 'root' || named === '0') {
      failures.push({
        line: last.line,
        rule: 'runtime-user-root',
        detail: `the runtime stage's last USER names "${named}"`,
      });
    }
  }

  return failures;
}

function refusedLines(text: string): number[] {
  return Array.from(new Set(checkDockerfile(text).map((f) => f.line))).sort(
    (a, b) => a - b,
  );
}

function describeFailures(text: string): string {
  return checkDockerfile(text)
    .map((f) => `line ${f.line} [${f.rule}] ${f.detail}`)
    .join('; ');
}

// ---------------------------------------------------------------------------
// Fixture texts (AC3): the two files as committed at 1ea0dff, and the
// single-change variants the check must refuse or admit.
// ---------------------------------------------------------------------------

const BASE_ROOT_TEXT = `# Build stage
FROM node:20-alpine AS builder

# Accept build argument
ARG NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL

WORKDIR /app

# Copy package files from web app
COPY apps/web/package.json apps/web/package-lock.json* ./

# Install dependencies
RUN npm install --legacy-peer-deps

# Copy source code from web app
COPY apps/web/ ./

# Build the application
RUN npm run build

# Runtime stage - optimized for standalone output
FROM node:20-alpine

# SECURITY: Run as non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

ENV NODE_ENV=production
ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

# Copy standalone build
COPY --from=builder /app/.next/standalone ./

# Copy static files to the standalone folder
COPY --from=builder /app/.next/static ./.next/static

# Copy public folder to the standalone folder
COPY --from=builder /app/public ./public

# Ensure correct ownership
RUN chown -R appuser:appgroup /app

USER appuser

# OCI labels
LABEL org.opencontainers.image.source="https://github.com/opena2a-org/agent-identity-management"
LABEL org.opencontainers.image.title="AIM Dashboard"
LABEL org.opencontainers.image.description="Web dashboard for Agent Identity Management"
LABEL org.opencontainers.image.licenses="Apache-2.0"

# Expose port
EXPOSE 3000

# Start the standalone server
CMD ["node", "server.js"]
`;

const BASE_TWIN_TEXT = `# Multi-stage build for Next.js frontend
FROM node:20-alpine AS base

# Install dependencies only when needed
FROM base AS deps
RUN apk add --no-cache libc6-compat
WORKDIR /app

# Copy package files
COPY apps/web/package*.json ./
RUN npm install --production=false --legacy-peer-deps

# Rebuild the source code only when needed
FROM base AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY apps/web/ ./

# Next.js collects completely anonymous telemetry data about general usage
ENV NEXT_TELEMETRY_DISABLED=1

RUN npm run build

# Production image, copy all the files and run next
FROM base AS runner
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

RUN addgroup --system --gid 1001 nodejs
RUN adduser --system --uid 1001 nextjs

# Copy necessary files
COPY --from=builder /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs

EXPOSE 3000

ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

CMD ["node", "server.js"]
`;

/** A fixture digest and fixture tags — never a pin. */
const FIXTURE_DIGEST = `sha256:${'0'.repeat(64)}`;
const SHORT_DIGEST = `sha256:${'0'.repeat(63)}`;

function replaceLine(text: string, line: number, replacement: string): string {
  const lines = text.split('\n');
  lines[line - 1] = replacement;
  return lines.join('\n');
}

function insertAfterLine(text: string, line: number, ...inserted: string[]): string {
  const lines = text.split('\n');
  lines.splice(line, 0, ...inserted);
  return lines.join('\n');
}

function deleteLine(text: string, line: number): string {
  const lines = text.split('\n');
  lines.splice(line - 1, 1);
  return lines.join('\n');
}

const PASSING_TEXT = replaceLine(
  replaceLine(
    BASE_ROOT_TEXT,
    2,
    `FROM node:22-alpine3.21@${FIXTURE_DIGEST} AS builder`,
  ),
  23,
  `FROM alpine:3.21@${FIXTURE_DIGEST}`,
);

interface FixtureCase {
  name: string;
  text: string;
  /** Sorted unique line numbers the check must name. */
  refused: number[];
}

const AC3_CASES: FixtureCase[] = [
  {
    name: 'infrastructure/docker/Dockerfile.frontend at 1ea0dff',
    text: BASE_ROOT_TEXT,
    refused: [2, 23],
  },
  {
    name: 'apps/backend/infrastructure/docker/Dockerfile.frontend at 1ea0dff',
    text: BASE_TWIN_TEXT,
    refused: [2, 25],
  },
  { name: 'the passing text', text: PASSING_TEXT, refused: [] },

  // --- planted regressions: the base -------------------------------------
  {
    name: 'line 2 unpinned again',
    text: replaceLine(PASSING_TEXT, 2, 'FROM node:20-alpine AS builder'),
    refused: [2],
  },
  {
    name: 'line 23 tagged but not pinned',
    text: replaceLine(PASSING_TEXT, 23, 'FROM alpine:3.21'),
    refused: [23],
  },
  {
    name: 'line 23 pinned but untagged',
    text: replaceLine(PASSING_TEXT, 23, `FROM alpine@${FIXTURE_DIGEST}`),
    refused: [23],
  },
  {
    name: 'line 23 tag from a variable',
    text: replaceLine(PASSING_TEXT, 23, `FROM alpine:$ALPINE@${FIXTURE_DIGEST}`),
    refused: [23],
  },
  {
    name: 'line 23 rooted on the node image',
    text: replaceLine(PASSING_TEXT, 23, `FROM node:22-alpine3.21@${FIXTURE_DIGEST}`),
    refused: [23],
  },
  {
    name: 'line 23 rooted on docker.io/library/node',
    text: replaceLine(
      PASSING_TEXT,
      23,
      `FROM docker.io/library/node:22-alpine3.21@${FIXTURE_DIGEST}`,
    ),
    refused: [23],
  },
  {
    name: 'line 23 rooted on a node mirror',
    text: replaceLine(
      PASSING_TEXT,
      23,
      `FROM public.ecr.aws/docker/library/node:22-alpine3.21@${FIXTURE_DIGEST}`,
    ),
    refused: [23],
  },
  {
    name: 'line 23 rooted on distroless',
    text: replaceLine(
      PASSING_TEXT,
      23,
      `FROM gcr.io/distroless/static-debian12:nonroot@${FIXTURE_DIGEST}`,
    ),
    refused: [23],
  },
  {
    name: 'line 23 digest one hex character short',
    text: replaceLine(PASSING_TEXT, 23, `FROM alpine:3.21@${SHORT_DIGEST}`),
    refused: [23],
  },
  {
    name: 'line 23 base from a variable',
    text: replaceLine(PASSING_TEXT, 23, 'FROM $RUNTIME_BASE'),
    refused: [23],
  },
  {
    name: 'line 23 rooted on the builder stage',
    text: replaceLine(PASSING_TEXT, 23, 'FROM builder'),
    refused: [23],
  },

  // --- planted regressions: the runtime stage's instructions --------------
  {
    name: 'RUN rm -rf of the npm directory',
    text: insertAfterLine(PASSING_TEXT, 23, 'RUN rm -rf /usr/local/lib/node_modules/npm'),
    refused: [24],
  },
  {
    name: 'RUN rm in exec form',
    text: insertAfterLine(
      PASSING_TEXT,
      23,
      'RUN ["rm", "-rf", "/usr/local/lib/node_modules/npm"]',
    ),
    refused: [24],
  },
  {
    name: 'RUN rm behind an absolute path',
    text: insertAfterLine(PASSING_TEXT, 23, 'RUN /bin/rm -rf /usr/local/lib/node_modules'),
    refused: [24],
  },
  {
    name: 'RUN rm behind a shell',
    text: insertAfterLine(
      PASSING_TEXT,
      23,
      'RUN sh -c "rm -rf /usr/local/lib/node_modules"',
    ),
    refused: [24],
  },
  {
    name: 'RUN rm chained behind an allowlisted command',
    text: insertAfterLine(
      PASSING_TEXT,
      23,
      'RUN chown -R appuser:appgroup /app && rm -rf /usr/local/lib/node_modules',
    ),
    refused: [24],
  },
  {
    name: 'RUN apk upgrade',
    text: insertAfterLine(PASSING_TEXT, 23, 'RUN apk upgrade --no-cache'),
    refused: [24],
  },
  {
    name: 'RUN apk del npm',
    text: insertAfterLine(PASSING_TEXT, 23, 'RUN apk del npm'),
    refused: [24],
  },
  {
    name: 'RUN apk add npm',
    text: insertAfterLine(PASSING_TEXT, 23, 'RUN apk add --no-cache npm'),
    refused: [24],
  },
  {
    name: 'RUN apk add a base package',
    text: insertAfterLine(PASSING_TEXT, 23, 'RUN apk add --no-cache libssl3'),
    refused: [24],
  },
  {
    name: 'RUN apk add --upgrade',
    text: insertAfterLine(PASSING_TEXT, 23, 'RUN apk add --no-cache --upgrade libstdc++'),
    refused: [24],
  },
  {
    name: 'RUN apk add continued onto a second line',
    text: insertAfterLine(
      PASSING_TEXT,
      23,
      'RUN apk add --no-cache libstdc++ \\',
      'npm',
    ),
    refused: [24],
  },
  {
    name: 'RUN with a heredoc body',
    text: insertAfterLine(
      PASSING_TEXT,
      23,
      'RUN <<EOF',
      'rm -rf /usr/local/lib/node_modules/npm',
      'EOF',
    ),
    refused: [24],
  },
  {
    name: 'RUN find -delete',
    text: insertAfterLine(
      PASSING_TEXT,
      23,
      "RUN find /usr/local/lib/node_modules -name '*.md' -delete",
    ),
    refused: [24],
  },
  {
    name: 'RUN corepack enable',
    text: insertAfterLine(PASSING_TEXT, 23, 'RUN corepack enable'),
    refused: [24],
  },
  {
    name: 'RUN from a variable',
    text: insertAfterLine(PASSING_TEXT, 23, 'RUN $CLEANUP'),
    refused: [24],
  },
  {
    name: 'RUN with a build mount',
    text: insertAfterLine(
      PASSING_TEXT,
      23,
      'RUN --mount=type=bind,from=builder,source=/usr/local/lib/node_modules,target=/mnt cp -r /mnt /usr/local/lib/node_modules',
    ),
    refused: [24],
  },
  {
    name: 'COPY of the npm directory',
    text: insertAfterLine(
      PASSING_TEXT,
      23,
      'COPY --from=builder /usr/local/lib/node_modules /usr/local/lib/node_modules',
    ),
    refused: [24],
  },
  {
    name: 'COPY of /usr/local',
    text: insertAfterLine(PASSING_TEXT, 23, 'COPY --from=builder /usr/local /usr/local'),
    refused: [24],
  },
  {
    name: 'COPY of the whole builder root',
    text: insertAfterLine(PASSING_TEXT, 23, 'COPY --from=builder / /'),
    refused: [24],
  },
  {
    name: 'COPY --from an image reference',
    text: insertAfterLine(
      PASSING_TEXT,
      23,
      `COPY --from=node:22-alpine3.21@${FIXTURE_DIGEST} /usr/local/bin/node /usr/local/bin/node`,
    ),
    refused: [24],
  },
  {
    name: 'COPY --from a stage index',
    text: insertAfterLine(
      PASSING_TEXT,
      23,
      'COPY --from=0 /usr/local/bin/node /usr/local/bin/node',
    ),
    refused: [24],
  },
  {
    name: 'COPY from the build context',
    text: insertAfterLine(PASSING_TEXT, 23, 'COPY apps/web/public ./public'),
    refused: [24],
  },
  {
    name: 'ADD from the network',
    text: insertAfterLine(
      PASSING_TEXT,
      23,
      'ADD https://registry.npmjs.org/npm/-/npm-10.8.2.tgz /tmp/npm.tgz',
    ),
    refused: [24],
  },
  {
    name: 'USER root after the non-root USER',
    text: insertAfterLine(PASSING_TEXT, 46, 'USER root'),
    refused: [47],
  },
  {
    name: 'the USER instruction deleted',
    text: deleteLine(PASSING_TEXT, 46),
    refused: [23],
  },

  // --- shapes the check must admit ---------------------------------------
  {
    name: 'a --platform flag on the builder FROM',
    text: replaceLine(
      PASSING_TEXT,
      2,
      `FROM --platform=$BUILDPLATFORM node:22-alpine3.21@${FIXTURE_DIGEST} AS builder`,
    ),
    refused: [],
  },
  {
    name: 'a --platform flag on the runtime FROM',
    text: replaceLine(
      PASSING_TEXT,
      23,
      `FROM --platform=$TARGETPLATFORM alpine:3.21@${FIXTURE_DIGEST}`,
    ),
    refused: [],
  },
  {
    name: 'RUN apk add --no-cache libstdc++',
    text: insertAfterLine(PASSING_TEXT, 23, 'RUN apk add --no-cache libstdc++'),
    refused: [],
  },
  {
    name: 'RUN apk add --no-cache libstdc++ libgcc',
    text: insertAfterLine(PASSING_TEXT, 23, 'RUN apk add --no-cache libstdc++ libgcc'),
    refused: [],
  },
  {
    name: 'COPY of the node binary from the builder',
    text: insertAfterLine(
      PASSING_TEXT,
      23,
      'COPY --from=builder /usr/local/bin/node /usr/local/bin/node',
    ),
    refused: [],
  },
  {
    name: 'COPY --chown of an /app/ path',
    text: insertAfterLine(
      PASSING_TEXT,
      23,
      'COPY --from=builder --chown=appuser:appgroup /app/public ./public',
    ),
    refused: [],
  },
  {
    name: 'RUN mkdir chained with chown',
    text: insertAfterLine(
      PASSING_TEXT,
      23,
      'RUN mkdir -p /app/.next/cache && chown -R appuser:appgroup /app/.next',
    ),
    refused: [],
  },
  {
    name: 'RUN adduser in exec form',
    text: insertAfterLine(
      PASSING_TEXT,
      23,
      'RUN ["adduser", "-S", "appuser2", "-G", "appgroup"]',
    ),
    refused: [],
  },
  {
    name: 'RUN rm in the builder stage, which the runtime rules do not reach',
    text: insertAfterLine(PASSING_TEXT, 14, 'RUN rm -rf /tmp/build'),
    refused: [],
  },
];

// ---------------------------------------------------------------------------
// The two frontend Dockerfiles (AC1, AC2)
// ---------------------------------------------------------------------------

const ROOT_DOCKERFILE = join(
  __dirname,
  '..',
  '..',
  '..',
  'infrastructure',
  'docker',
  'Dockerfile.frontend',
);

const TWIN_DOCKERFILE = join(
  __dirname,
  '..',
  '..',
  '..',
  'apps',
  'backend',
  'infrastructure',
  'docker',
  'Dockerfile.frontend',
);

/**
 * The node binary reaches the runtime stage only by a COPY from a pinned
 * official-node builder of the SAME alpine release, so the copied binary and
 * the runtime's musl/libstdc++ come from one alpine branch.
 */
function assertNodeBinaryComesFromThePinnedBuilder(text: string, label: string): void {
  const instructions = parseInstructions(text);
  const stages = parseStages(instructions);
  const finalIndex = stages.length - 1;
  const finalFromLine = stages[finalIndex].line;

  const nodeCopies = instructions
    .filter((i) => i.keyword === 'COPY' && i.line >= finalFromLine)
    .map((i) => ({ instruction: i, parts: copyParts(i) }))
    .filter(
      ({ parts }) =>
        parts.sources.length === 1 &&
        parts.sources[0] === '/usr/local/bin/node' &&
        parts.destination === '/usr/local/bin/node',
    );
  expect(
    nodeCopies.length,
    `${label}: the runtime stage must carry exactly one COPY of /usr/local/bin/node to /usr/local/bin/node`,
  ).toBe(1);

  const from = nodeCopies[0].parts.from;
  expect(from, `${label}: that COPY must carry --from=<builder stage>`).not.toBeNull();
  const builderIndex = earlierStageNamed(stages, from as string, finalIndex);
  expect(
    builderIndex,
    `${label}: COPY --from=${from} must name an earlier stage of the same file`,
  ).toBeGreaterThanOrEqual(0);

  const builderRoot = rootImageOf(stages, builderIndex);
  expect(
    isOfficialNode(repositoryOf(builderRoot)),
    `${label}: the stage the node binary is copied from must root on the Docker official node image, not "${builderRoot}"`,
  ).toBe(true);

  const builderTag = tagOf(builderRoot) ?? '';
  const alpineRelease = /-alpine(\d+\.\d+)$/.exec(builderTag);
  expect(
    alpineRelease,
    `${label}: the builder tag "${builderTag}" must end in -alpine<major.minor>`,
  ).not.toBeNull();
  const release = (alpineRelease as RegExpExecArray)[1];

  const runtimeTag = tagOf(rootImageOf(stages, finalIndex)) ?? '';
  expect(
    runtimeTag === release || runtimeTag.startsWith(`${release}.`),
    `${label}: the runtime tag "${runtimeTag}" must be ${release} or begin with ${release}. so builder and runtime share one alpine release`,
  ).toBe(true);
}

function assertRuntimeUserIsNotRoot(text: string, label: string): void {
  const instructions = parseInstructions(text);
  const stages = parseStages(instructions);
  const finalFromLine = stages[stages.length - 1].line;
  const users = instructions.filter((i) => i.keyword === 'USER' && i.line >= finalFromLine);
  expect(users.length, `${label}: the runtime stage must carry a USER`).toBeGreaterThan(0);
  const named = (users[users.length - 1].args.trim().split(/\s+/)[0] ?? '').split(':')[0];
  expect(named, `${label}: the runtime stage's last USER must not be root`).not.toBe('root');
  expect(named, `${label}: the runtime stage's last USER must not be 0`).not.toBe('0');
}

function assertServerCmdSurvives(text: string, label: string): void {
  const cmds = parseInstructions(text).filter((i) => i.keyword === 'CMD');
  expect(cmds.length, `${label}: the file must carry exactly one CMD`).toBe(1);
  expect(cmds[0].text.trim(), `${label}: CMD ["node", "server.js"] must survive unchanged`).toBe(
    'CMD ["node", "server.js"]',
  );
}

function assertNoFromReads(text: string, refused: string[], label: string): void {
  const froms = parseInstructions(text).filter((i) => i.keyword === 'FROM');
  for (const text_ of refused) {
    expect(
      froms.map((f) => f.text.trim()),
      `${label}: no FROM instruction may read \`${text_}\``,
    ).not.toContain(text_);
  }
}

// ---------------------------------------------------------------------------
// The publish workflow and the tree walk (AC4)
// ---------------------------------------------------------------------------

interface WorkflowStep {
  name?: string;
  uses?: string;
  with?: Record<string, unknown>;
}

interface PublishWorkflow {
  jobs?: Record<string, { steps?: WorkflowStep[] }>;
}

const PUBLISH_WORKFLOW = join(
  __dirname,
  '..',
  '..',
  '..',
  '.github',
  'workflows',
  'docker-publish.yml',
);

function readPublishWorkflow(): PublishWorkflow {
  return load(readFileSync(PUBLISH_WORKFLOW, 'utf-8')) as PublishWorkflow;
}

/** The one docker/build-push-action step of the build-frontend job. */
function frontendBuildStep(workflow: PublishWorkflow): WorkflowStep {
  const steps = workflow.jobs?.['build-frontend']?.steps ?? [];
  const builds = steps.filter(
    (s) => typeof s.uses === 'string' && s.uses.startsWith('docker/build-push-action'),
  );
  expect(
    builds.length,
    'docker-publish.yml build-frontend must carry exactly one docker/build-push-action step',
  ).toBe(1);
  return builds[0];
}

/**
 * The published image is the final stage of the file at `file:`: a `target:`
 * would publish an earlier stage and a `build-contexts:` would remap a stage
 * name to an image outside the file, and either reopens the node root the
 * pinned Dockerfile just closed.
 */
function assertBuildStepIsTargetFree(workflow: PublishWorkflow): void {
  const step = frontendBuildStep(workflow);
  const withMap = step.with ?? {};
  expect(
    Object.prototype.hasOwnProperty.call(withMap, 'target'),
    'the build-frontend build step must name no target',
  ).toBe(false);
  expect(
    Object.prototype.hasOwnProperty.call(withMap, 'build-contexts'),
    'the build-frontend build step must name no build-contexts',
  ).toBe(false);
}

function walkDockerfileFrontends(root: string): string[] {
  const found: string[] = [];
  const pending = [root];
  while (pending.length > 0) {
    const directory = pending.pop() as string;
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      if (entry.isDirectory()) {
        if (entry.name === 'node_modules' || entry.name === '.git') continue;
        pending.push(join(directory, entry.name));
      } else if (entry.isFile() && entry.name === 'Dockerfile.frontend') {
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

describe('QGF-230 both frontend Dockerfiles pin every base and root the runtime on alpine', () => {
  it('QGF-230.AC1 infrastructure/docker/Dockerfile.frontend, the file docker-publish.yml builds, pins every base, roots its runtime on alpine, copies the node binary from the pinned builder of the same alpine release, runs non-root and keeps CMD ["node", "server.js"]', () => {
    const text = readFileSync(ROOT_DOCKERFILE, 'utf-8');

    // The file under test is the one the publish workflow actually builds.
    expect(
      frontendBuildStep(readPublishWorkflow()).with?.file,
      'docker-publish.yml build-frontend must build infrastructure/docker/Dockerfile.frontend',
    ).toBe('infrastructure/docker/Dockerfile.frontend');

    expect(
      refusedLines(text),
      `infrastructure/docker/Dockerfile.frontend must pass the base-pin check: ${describeFailures(text)}`,
    ).toEqual([]);

    assertNoFromReads(
      text,
      ['FROM node:20-alpine AS builder', 'FROM node:20-alpine'],
      'infrastructure/docker/Dockerfile.frontend',
    );
    assertNodeBinaryComesFromThePinnedBuilder(
      text,
      'infrastructure/docker/Dockerfile.frontend',
    );
    assertRuntimeUserIsNotRoot(text, 'infrastructure/docker/Dockerfile.frontend');
    assertServerCmdSurvives(text, 'infrastructure/docker/Dockerfile.frontend');
  });

  it('QGF-230.AC2 apps/backend/infrastructure/docker/Dockerfile.frontend, the twin, has the same properties: no unpinned node base for its stage table, an alpine-rooted runner, the node binary from the pinned builder, a non-root USER and CMD ["node", "server.js"]', () => {
    const text = readFileSync(TWIN_DOCKERFILE, 'utf-8');

    expect(
      refusedLines(text),
      `apps/backend/infrastructure/docker/Dockerfile.frontend must pass the base-pin check: ${describeFailures(text)}`,
    ).toEqual([]);

    assertNoFromReads(
      text,
      ['FROM node:20-alpine AS base'],
      'apps/backend/infrastructure/docker/Dockerfile.frontend',
    );
    assertNodeBinaryComesFromThePinnedBuilder(
      text,
      'apps/backend/infrastructure/docker/Dockerfile.frontend',
    );
    assertRuntimeUserIsNotRoot(text, 'apps/backend/infrastructure/docker/Dockerfile.frontend');
    assertServerCmdSurvives(text, 'apps/backend/infrastructure/docker/Dockerfile.frontend');
  });

  it('QGF-230.AC3 the base-pin check refuses the two base texts by line, admits the repaired text, and names the right line for every planted regression', () => {
    for (const fixture of AC3_CASES) {
      expect(
        refusedLines(fixture.text),
        `${fixture.name}: ${describeFailures(fixture.text) || 'no failure reported'}`,
      ).toEqual(fixture.refused);
    }
  });

  it('QGF-230.AC4 every Dockerfile.frontend the tree walk finds passes the check, the population carries the path docker-publish.yml builds, and that build step names no target and no build-contexts', () => {
    const population = walkDockerfileFrontends(REPO_ROOT).map(repoRelative);
    // Report the reach of the walk, not only its verdict.
    console.log(
      `QGF-230.AC4 Dockerfile.frontend population (${population.length}): ${population.join(', ')}`,
    );

    const workflow = readPublishWorkflow();
    const built = frontendBuildStep(workflow).with?.file;
    expect(typeof built, 'the build-frontend build step must carry a file: key').toBe('string');
    expect(
      population,
      `a walk rooted at ${REPO_ROOT} must reach the file docker-publish.yml builds`,
    ).toContain(built as string);

    for (const path of population) {
      const text = readFileSync(join(REPO_ROOT, path), 'utf-8');
      expect(
        refusedLines(text),
        `${path} must pass the base-pin check: ${describeFailures(text)}`,
      ).toEqual([]);
    }

    assertBuildStepIsTargetFree(workflow);

    // Negative controls: the same assertion must fail on a scratch copy that
    // publishes an earlier stage, and on one that remaps a stage to an image.
    const withTarget = readPublishWorkflow();
    (frontendBuildStep(withTarget).with as Record<string, unknown>).target = 'builder';
    expect(() => assertBuildStepIsTargetFree(withTarget)).toThrow();

    const withBuildContexts = readPublishWorkflow();
    (frontendBuildStep(withBuildContexts).with as Record<string, unknown>)['build-contexts'] =
      'builder=docker-image://node:20-alpine';
    expect(() => assertBuildStepIsTargetFree(withBuildContexts)).toThrow();
  });
});
