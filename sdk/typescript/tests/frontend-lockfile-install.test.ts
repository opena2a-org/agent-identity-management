import { describe, it, expect } from 'vitest';
import { existsSync, readFileSync, readdirSync } from 'fs';
import { join, relative, sep } from 'path';
import { load } from 'js-yaml';

/**
 * The install pin for the published aim-dashboard image.
 *
 * The published aim-dashboard image was built by docker-publish.yml from a
 * Dockerfile whose `COPY apps/web/package.json apps/web/package-lock.json* ./`
 * matched no lockfile (the glob tolerated its absence) and whose
 * `RUN npm install --legacy-peer-deps` therefore resolved every dependency on
 * build day and ran every lifecycle script it pulled. The fix pins both
 * frontend Dockerfiles to the committed apps/web/package-lock.json with
 * `npm ci --ignore-scripts`, and this cell is the tree-side refusal that keeps
 * the image's inputs that shape:
 *
 *   AC1  infrastructure/docker/Dockerfile.frontend (the file docker-publish.yml
 *        builds) copies the manifest and the lockfile by literal name, installs
 *        with exactly `npm ci --ignore-scripts [--legacy-peer-deps]`, invokes
 *        no other npm verb than `ci` and `run build`, and keeps `RUN npm run build`;
 *   AC2  apps/backend/infrastructure/docker/Dockerfile.frontend, the twin, same
 *        (with `--include=dev` admitted);
 *   AC3  the check itself, over fixture texts: the two pre-fix texts are refused
 *        by line, the repaired text passes, and every planted install form —
 *        aliases, continuation lines, heredocs, a shell wrapper, another package
 *        manager, a glob or absent lockfile COPY — is refused at the line its
 *        instruction keyword sits on;
 *   AC4  every Dockerfile the tree walk finds passes the check and the
 *        population includes the workflow's `file:`;
 *   AC5  apps/web/package-lock.json is lockfileVersion 3, pins one `next` at the
 *        exact version apps/web/package.json names, carries the manifest's two
 *        dependency maps (as does the root lockfile's `apps/web` entry), and
 *        every entry it carries is one the root package-lock.json already
 *        records at the same name, version and integrity;
 *   AC6  docker-publish.yml keeps its signed, attested shape and builds a file
 *        that exists and passes the check.
 *
 * The AC3 fixtures are literal texts, not reads of the tree: the check must keep
 * refusing the pre-fix shape long after the tree stops carrying it.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');

// ---------------------------------------------------------------------------
// Dockerfile parsing (the same shape dockerfile-frontend-base-pins.test.ts reads)
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

/** Lower-cased `AS <name>` of the FROM governing `line`, or null. */
function stageNameAt(instructions: Instruction[], line: number): string | null {
  let name: string | null = null;
  for (const instruction of instructions) {
    if (instruction.line > line) break;
    if (instruction.keyword !== 'FROM') continue;
    const tokens = instruction.args.split(/\s+/).filter((t) => t.length > 0);
    let at = 0;
    while (at < tokens.length && tokens[at].startsWith('--')) at += 1;
    name =
      tokens[at + 1] !== undefined &&
      tokens[at + 1].toUpperCase() === 'AS' &&
      tokens[at + 2] !== undefined
        ? tokens[at + 2].toLowerCase()
        : null;
  }
  return name;
}

/** Surrounding quotes and any leading directory path removed: `"npm` is `npm`. */
function normalizeToken(token: string): string {
  const unquoted = token.replace(/^['"]+/, '').replace(/['"]+$/, '');
  const slash = unquoted.lastIndexOf('/');
  return slash >= 0 ? unquoted.slice(slash + 1) : unquoted;
}

const SHELL_SEPARATORS = /&&|\|\||;|\|/;

/**
 * A shell-form text has its backslash continuations joined, then is split on
 * newlines (so a heredoc body is read line by line) and on `&&`, `||`, `;`
 * and `|`; each piece is a command, split on whitespace into tokens.
 */
function shellTextCommands(text: string): string[][] {
  const joined = text.replace(/\\[ \t]*\r?\n/g, ' ');
  const pieces: string[] = [];
  for (const physical of joined.split('\n')) {
    for (const piece of physical.split(SHELL_SEPARATORS)) {
      pieces.push(piece);
    }
  }
  return pieces
    .map((piece) => piece.trim())
    .filter((piece) => piece.length > 0)
    .map((piece) => piece.split(/\s+/).filter((token) => token.length > 0));
}

/** An exec-form entry that is itself a script: `npm install` in `["sh", "-c", "npm install"]`. */
function isScriptEntry(entry: string): boolean {
  return /\s/.test(entry) || SHELL_SEPARATORS.test(entry);
}

/**
 * An exec-form JSON array is one command whose entries are its tokens, and
 * every entry that is itself a script (the body a `["sh", "-c", ...]` wrapper
 * hands to the shell) is read the way shell form is, so the exec-form spelling
 * of a wrapper hides no install the shell-form spelling would show. A
 * shell-form text is read by shellTextCommands.
 */
function shellCommands(instruction: Instruction): string[][] {
  const trimmed = instruction.args.trim();
  if (trimmed.startsWith('[')) {
    try {
      const parsed: unknown = JSON.parse(trimmed);
      if (Array.isArray(parsed)) {
        const entries = parsed.map((entry) => String(entry));
        return [
          entries,
          ...entries.filter(isScriptEntry).flatMap((entry) => shellTextCommands(entry)),
        ];
      }
    } catch {
      // Not a well-formed exec form; fall through and read it as shell form.
    }
  }
  return shellTextCommands(instruction.args);
}

function copySources(instruction: Instruction): string[] {
  const tokens = instruction.args.split(/\s+/).filter((t) => t.length > 0);
  const positional = tokens.filter((t) => !t.startsWith('--'));
  return positional.slice(0, -1);
}

const GLOB_CHARACTERS = /[*?[\]]/;

function basenameOf(source: string): string {
  const slash = source.lastIndexOf('/');
  return slash >= 0 ? source.slice(slash + 1) : source;
}

/** A COPY naming a literal source whose basename is package-lock.json, no glob anywhere. */
function copiesLiteralLockfile(instruction: Instruction): boolean {
  if (instruction.keyword !== 'COPY') return false;
  if (GLOB_CHARACTERS.test(instruction.text)) return false;
  return copySources(instruction).some((source) => basenameOf(source) === 'package-lock.json');
}

// ---------------------------------------------------------------------------
// The install check (AC3)
// ---------------------------------------------------------------------------

/** Package managers whose every invocation is refused. */
const OTHER_PACKAGE_MANAGERS = new Set(['npx', 'yarn', 'pnpm', 'bun', 'corepack']);

/** Every `npm` token of a command with the tokens that follow it, up to the command's end. */
interface NpmInvocation {
  line: number;
  /** The tokens after `npm`: the verb first, then its arguments. */
  args: string[];
}

function npmInvocationsOf(instruction: Instruction): NpmInvocation[] {
  const found: NpmInvocation[] = [];
  for (const command of shellCommands(instruction)) {
    const tokens = command.map(normalizeToken);
    for (let at = 0; at < tokens.length; at += 1) {
      if (tokens[at] === 'npm') {
        found.push({ line: instruction.line, args: tokens.slice(at + 1) });
      }
    }
  }
  return found;
}

/**
 * `--ignore-scripts` (or `--ignore-scripts=true`) present, no other
 * `--ignore-scripts=<value>`, no `--no-ignore-scripts`, and no bare
 * `--ignore-scripts false` (nopt reads the following true/false as the value).
 */
function ignoreScriptsRefusal(args: string[]): string | null {
  let enabled = false;
  for (let i = 0; i < args.length; i += 1) {
    const arg = args[i];
    if (arg === '--no-ignore-scripts') return 'carries --no-ignore-scripts';
    if (arg === '--ignore-scripts') {
      if (args[i + 1] === 'false') return 'carries --ignore-scripts false';
      enabled = true;
    } else if (arg === '--ignore-scripts=true') {
      enabled = true;
    } else if (arg.startsWith('--ignore-scripts=')) {
      return `carries ${arg}`;
    }
  }
  return enabled ? null : 'carries no --ignore-scripts';
}

function checkRun(
  instruction: Instruction,
  literalLockfileCopied: boolean,
  failures: Failure[],
): void {
  const line = instruction.line;
  for (const command of shellCommands(instruction)) {
    const tokens = command.map(normalizeToken);
    for (let at = 0; at < tokens.length; at += 1) {
      const token = tokens[at];
      if (OTHER_PACKAGE_MANAGERS.has(token)) {
        failures.push({
          line,
          rule: 'package-manager',
          detail: `"${token}" is refused; the only admitted install is npm ci --ignore-scripts from the committed lockfile`,
        });
        continue;
      }
      if (token !== 'npm') continue;
      const verb = tokens[at + 1];
      if (verb === 'run') continue;
      if (verb !== 'ci') {
        failures.push({
          line,
          rule: 'npm-verb',
          detail: `npm ${verb ?? '<nothing>'} is refused; only npm ci and npm run are admitted`,
        });
        continue;
      }
      const refusal = ignoreScriptsRefusal(tokens.slice(at + 2));
      if (refusal !== null) {
        failures.push({ line, rule: 'npm-ci-scripts', detail: `npm ci ${refusal}` });
      }
      if (!literalLockfileCopied) {
        failures.push({
          line,
          rule: 'npm-ci-no-lockfile',
          detail:
            'npm ci follows no earlier COPY that names a literal package-lock.json source with no glob in the instruction',
        });
      }
    }
  }
}

/**
 * Refuse, by line, every package installation that is not a lockfile install
 * with lifecycle scripts disabled.
 */
function checkDockerfile(text: string): Failure[] {
  const failures: Failure[] = [];
  let literalLockfileCopied = false;
  for (const instruction of parseInstructions(text)) {
    if (copiesLiteralLockfile(instruction)) literalLockfileCopied = true;
    if (instruction.keyword === 'RUN') checkRun(instruction, literalLockfileCopied, failures);
  }
  return failures;
}

function refusedLines(text: string): number[] {
  return Array.from(new Set(checkDockerfile(text).map((f) => f.line))).sort((a, b) => a - b);
}

function describeFailures(text: string): string {
  return checkDockerfile(text)
    .map((f) => `line ${f.line} [${f.rule}] ${f.detail}`)
    .join('; ');
}

// ---------------------------------------------------------------------------
// Fixture texts (AC3): the two files before the fix — lines 11/14/20 of the
// root file and 10/11/22 of the twin are the ones the contract names at
// efe28e11875666a1737021959ebc9c5565660a97 — and the single-change variants.
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

const PASSING_TEXT = replaceLine(
  replaceLine(BASE_ROOT_TEXT, 11, 'COPY apps/web/package.json apps/web/package-lock.json ./'),
  14,
  'RUN npm ci --ignore-scripts --legacy-peer-deps',
);

interface FixtureCase {
  name: string;
  text: string;
  /** Sorted unique line numbers the check must name. */
  refused: number[];
}

const AC3_LINE14_REGRESSIONS = [
  'RUN npm ci --legacy-peer-deps',
  'RUN npm ci --ignore-scripts=false',
  'RUN npm ci --ignore-scripts --no-ignore-scripts',
  'RUN npm \\\ninstall --legacy-peer-deps',
  'RUN <<EOF\nnpm install\nEOF',
  'RUN npm i',
  'RUN npm inst',
  'RUN npm add next',
  'RUN npm up',
  'RUN npm rebuild',
  'RUN npm rb',
  'RUN npm exec next build',
  'RUN npx next build',
  'RUN sh -c "npm install"',
  // Exec-form spellings of the same wrappers: a JSON array whose entry carries
  // a shell script is still an install, and is read the way the shell form is.
  'RUN ["sh", "-c", "npm install"]',
  'RUN ["/bin/sh", "-c", "npm install --legacy-peer-deps"]',
  'RUN ["bash", "-lc", "npm ci"]',
  'RUN ["npm", "install"]',
  'RUN ["sh", "-c", "npm ci --ignore-scripts && npm rebuild"]',
];

const AC3_CASES: FixtureCase[] = [
  {
    name: 'infrastructure/docker/Dockerfile.frontend before the fix',
    text: BASE_ROOT_TEXT,
    refused: [14],
  },
  {
    name: 'apps/backend/infrastructure/docker/Dockerfile.frontend before the fix',
    text: BASE_TWIN_TEXT,
    refused: [11],
  },
  { name: 'the passing text', text: PASSING_TEXT, refused: [] },
  ...AC3_LINE14_REGRESSIONS.map((replacement) => ({
    name: `line 14 as ${JSON.stringify(replacement)}`,
    text: replaceLine(PASSING_TEXT, 14, replacement),
    refused: [14],
  })),
  {
    name: 'a line RUN yarn install inserted after line 14',
    text: insertAfterLine(PASSING_TEXT, 14, 'RUN yarn install'),
    refused: [15],
  },
  {
    name: 'line 11 as the glob COPY apps/web/package.json apps/web/package-lock.json* ./',
    text: replaceLine(PASSING_TEXT, 11, 'COPY apps/web/package.json apps/web/package-lock.json* ./'),
    refused: [14],
  },
  {
    name: 'line 11 as COPY apps/web/package.json ./',
    text: replaceLine(PASSING_TEXT, 11, 'COPY apps/web/package.json ./'),
    refused: [14],
  },

  // --- shapes the check must admit ---------------------------------------
  {
    name: 'line 14 as RUN npm ci --ignore-scripts=true',
    text: replaceLine(PASSING_TEXT, 14, 'RUN npm ci --ignore-scripts=true'),
    refused: [],
  },
  {
    name: 'line 14 as a continued npm ci whose second line carries --ignore-scripts',
    text: replaceLine(PASSING_TEXT, 14, 'RUN npm ci \\\n    --ignore-scripts'),
    refused: [],
  },
  {
    name: 'line 14 as npm ci in exec form',
    text: replaceLine(PASSING_TEXT, 14, 'RUN ["npm", "ci", "--ignore-scripts"]'),
    refused: [],
  },
  {
    name: 'line 14 as npm ci --ignore-scripts behind an exec-form sh -c wrapper',
    text: replaceLine(
      PASSING_TEXT,
      14,
      'RUN ["sh", "-c", "npm ci --ignore-scripts --legacy-peer-deps && npm run build"]',
    ),
    refused: [],
  },
];

// ---------------------------------------------------------------------------
// The two frontend Dockerfiles (AC1, AC2)
// ---------------------------------------------------------------------------

const ROOT_DOCKERFILE = join(REPO_ROOT, 'infrastructure', 'docker', 'Dockerfile.frontend');

const TWIN_DOCKERFILE = join(
  REPO_ROOT,
  'apps',
  'backend',
  'infrastructure',
  'docker',
  'Dockerfile.frontend',
);

interface InstallShape {
  label: string;
  /** Lower-cased name of the stage that must carry the COPY and the install. */
  stage: string;
  /** The arguments `npm ci` may carry beyond --ignore-scripts. */
  optionalArgs: string[];
  /** The pre-fix install line that must be gone. */
  absentLine: string;
}

function assertInstallsFromTheCommittedLockfile(text: string, shape: InstallShape): void {
  const { label } = shape;
  const instructions = parseInstructions(text);

  expect(
    refusedLines(text),
    `${label} must pass the install check: ${describeFailures(text)}`,
  ).toEqual([]);

  // Every npm invocation is `npm ci` or `npm run build`.
  const invocations = instructions.flatMap(npmInvocationsOf);
  expect(invocations.length, `${label} must invoke npm`).toBeGreaterThan(0);
  for (const invocation of invocations) {
    const isCi = invocation.args[0] === 'ci';
    const isRunBuild =
      invocation.args.length === 2 && invocation.args[0] === 'run' && invocation.args[1] === 'build';
    expect(
      isCi || isRunBuild,
      `${label} line ${invocation.line}: npm ${invocation.args.join(' ')} is neither npm ci nor npm run build`,
    ).toBe(true);
  }

  // Exactly one RUN invokes npm ci, with exactly the admitted arguments.
  const ciRuns = instructions.filter(
    (i) => i.keyword === 'RUN' && npmInvocationsOf(i).some((n) => n.args[0] === 'ci'),
  );
  expect(ciRuns.length, `${label} must carry exactly one RUN that invokes npm ci`).toBe(1);
  const ciRun = ciRuns[0];
  const ciArgs = npmInvocationsOf(ciRun)
    .filter((n) => n.args[0] === 'ci')
    .flatMap((n) => n.args.slice(1));
  expect(
    ciArgs.some((a) => a === '--ignore-scripts' || a === '--ignore-scripts=true'),
    `${label} line ${ciRun.line}: npm ci must carry --ignore-scripts`,
  ).toBe(true);
  const admitted = new Set(['--ignore-scripts', '--ignore-scripts=true', ...shape.optionalArgs]);
  for (const arg of ciArgs) {
    expect(
      admitted.has(arg),
      `${label} line ${ciRun.line}: npm ci argument "${arg}" is outside ${Array.from(admitted).join(', ')}`,
    ).toBe(true);
  }
  expect(
    stageNameAt(instructions, ciRun.line),
    `${label} line ${ciRun.line}: npm ci must run in the ${shape.stage} stage`,
  ).toBe(shape.stage);

  // Before it, in the same stage, a COPY names both files literally, no glob.
  const literalCopies = instructions.filter((i) => {
    if (i.keyword !== 'COPY' || i.line >= ciRun.line) return false;
    if (GLOB_CHARACTERS.test(i.text)) return false;
    const sources = copySources(i);
    return (
      sources.includes('apps/web/package.json') &&
      sources.includes('apps/web/package-lock.json') &&
      stageNameAt(instructions, i.line) === shape.stage
    );
  });
  expect(
    literalCopies.length,
    `${label}: a COPY in the ${shape.stage} stage before line ${ciRun.line} must name apps/web/package.json and apps/web/package-lock.json literally with no glob character`,
  ).toBeGreaterThan(0);

  // The pre-fix line is gone and the build instruction survives unchanged.
  expect(
    text.split('\n').map((l) => l.trim()),
    `${label} must no longer carry \`${shape.absentLine}\``,
  ).not.toContain(shape.absentLine);
  const builds = instructions.filter((i) => i.text.trim() === 'RUN npm run build');
  expect(builds.length, `${label}: \`RUN npm run build\` must survive unchanged`).toBe(1);
}

// ---------------------------------------------------------------------------
// The tree walk (AC4)
// ---------------------------------------------------------------------------

/**
 * A Dockerfile is a file whose basename, case-insensitively, begins with
 * `Dockerfile` or `Containerfile` or ends with `.dockerfile` or
 * `.containerfile`. After the stem a name carries at most one dotted suffix
 * (`Dockerfile`, `Dockerfile.frontend`, `Dockerfile-dev`): a dotted chain such
 * as `dockerfile-frontend-base-pins.test.ts` is a vitest cell whose fixtures
 * quote the refused instructions as literal text, not a Dockerfile.
 */
function isDockerfileName(name: string): boolean {
  const lower = name.toLowerCase();
  if (lower.endsWith('.dockerfile') || lower.endsWith('.containerfile')) return true;
  const stem = ['dockerfile', 'containerfile'].find((s) => lower.startsWith(s));
  if (stem === undefined) return false;
  const rest = lower.slice(stem.length);
  return (rest.match(/\./g) ?? []).length <= 1;
}

function walkDockerfiles(root: string): string[] {
  const found: string[] = [];
  const pending = [root];
  while (pending.length > 0) {
    const directory = pending.pop() as string;
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      if (entry.isDirectory()) {
        if (entry.name === 'node_modules' || entry.name === '.git') continue;
        pending.push(join(directory, entry.name));
      } else if (entry.isFile() && isDockerfileName(entry.name)) {
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
// The lockfile and its parity with the root lockfile (AC5)
// ---------------------------------------------------------------------------

interface LockEntry {
  version?: string;
  integrity?: string;
  link?: boolean;
  dependencies?: Record<string, string>;
  devDependencies?: Record<string, string>;
}

interface Lockfile {
  lockfileVersion?: number;
  packages?: Record<string, LockEntry>;
}

interface Manifest {
  dependencies?: Record<string, string>;
  devDependencies?: Record<string, string>;
}

interface LockFailure {
  /** The differing key: a dependency name, a `packages` key, or a field. */
  key: string;
  detail: string;
}

const WEB_DIR = join(REPO_ROOT, 'apps', 'web');
const ROOT_LOCKFILE = join(REPO_ROOT, 'package-lock.json');

function readJson<T>(path: string): T {
  return JSON.parse(readFileSync(path, 'utf-8')) as T;
}

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

/** The path after the last `node_modules/`. */
function packageNameOf(key: string): string {
  const marker = 'node_modules/';
  const at = key.lastIndexOf(marker);
  return at < 0 ? key : key.slice(at + marker.length);
}

function mapDifferences(
  field: string,
  left: Record<string, string>,
  leftLabel: string,
  right: Record<string, string>,
  rightLabel: string,
): LockFailure[] {
  const failures: LockFailure[] = [];
  for (const key of new Set([...Object.keys(left), ...Object.keys(right)])) {
    if (left[key] !== right[key]) {
      failures.push({
        key,
        detail: `${field}.${key} is ${JSON.stringify(left[key])} in ${leftLabel} but ${JSON.stringify(right[key])} in ${rightLabel}`,
      });
    }
  }
  return failures;
}

function isNextKey(key: string): boolean {
  return key === 'node_modules/next' || key.endsWith('/node_modules/next');
}

const EXACT_VERSION = /^\d+(\.\d+)*$/;

function checkLockfileParity(nested: Lockfile, manifest: Manifest, root: Lockfile): LockFailure[] {
  const failures: LockFailure[] = [];

  if (nested.lockfileVersion !== 3) {
    failures.push({
      key: 'lockfileVersion',
      detail: `apps/web/package-lock.json lockfileVersion is ${String(nested.lockfileVersion)}, not 3`,
    });
  }

  const packages = nested.packages ?? {};
  const nextKeys = Object.keys(packages).filter(isNextKey);
  if (nextKeys.length !== 1) {
    failures.push({
      key: 'node_modules/next',
      detail: `apps/web/package-lock.json carries ${nextKeys.length} node_modules/next entries (${nextKeys.join(', ')}), not exactly one`,
    });
  }

  const nestedRoot = packages[''] ?? {};
  const rootWeb = root.packages?.['apps/web'] ?? {};
  for (const field of ['dependencies', 'devDependencies'] as const) {
    failures.push(
      ...mapDifferences(
        field,
        nestedRoot[field] ?? {},
        'apps/web/package-lock.json packages[""]',
        manifest[field] ?? {},
        'apps/web/package.json',
      ),
      ...mapDifferences(
        field,
        rootWeb[field] ?? {},
        'package-lock.json packages["apps/web"]',
        manifest[field] ?? {},
        'apps/web/package.json',
      ),
    );
  }

  const nextRange = manifest.dependencies?.next;
  if (typeof nextRange !== 'string' || !EXACT_VERSION.test(nextRange)) {
    failures.push({
      key: 'next',
      detail: `apps/web/package.json names next as ${JSON.stringify(nextRange)}, not an exact version of digits and dots`,
    });
  } else if (nextKeys.length === 1 && packages[nextKeys[0]].version !== nextRange) {
    failures.push({
      key: 'next',
      detail: `apps/web/package.json pins next ${nextRange} but ${nextKeys[0]} is version ${String(packages[nextKeys[0]].version)}`,
    });
  }

  const rootIndex = new Map<string, Set<string>>();
  for (const [key, entry] of Object.entries(root.packages ?? {})) {
    if (key === '') continue;
    const name = packageNameOf(key);
    const signatures = rootIndex.get(name) ?? new Set<string>();
    signatures.add(`${String(entry.version)}\n${String(entry.integrity)}`);
    rootIndex.set(name, signatures);
  }
  for (const [key, entry] of Object.entries(packages)) {
    if (key === '' || entry.link === true) continue;
    const name = packageNameOf(key);
    const signature = `${String(entry.version)}\n${String(entry.integrity)}`;
    if (!(rootIndex.get(name)?.has(signature) ?? false)) {
      failures.push({
        key,
        detail: `package-lock.json carries no entry named ${name} at version ${String(entry.version)} with integrity ${String(entry.integrity)}`,
      });
    }
  }

  return failures;
}

function describeLockFailures(failures: LockFailure[]): string {
  return failures.map((f) => `${f.key}: ${f.detail}`).join('; ');
}

// ---------------------------------------------------------------------------
// The publish workflow (AC6)
// ---------------------------------------------------------------------------

interface WorkflowStep {
  name?: string;
  uses?: string;
  run?: string;
  env?: Record<string, unknown>;
  with?: Record<string, unknown>;
}

interface PublishWorkflow {
  on?: { push?: { paths?: string[] } };
  env?: Record<string, unknown>;
  jobs?: Record<string, { steps?: WorkflowStep[] }>;
}

const PUBLISH_WORKFLOW = join(REPO_ROOT, '.github', 'workflows', 'docker-publish.yml');

function readPublishWorkflow(): PublishWorkflow {
  const parsed = load(readFileSync(PUBLISH_WORKFLOW, 'utf-8')) as Record<string, unknown>;
  // A YAML 1.1 loader would read the `on` key as boolean true; js-yaml 4 reads
  // it as the string "on". Accept either spelling of the same key.
  const on = parsed.on ?? parsed.true;
  return { ...parsed, on } as PublishWorkflow;
}

const FRONTEND_GHCR_IMAGE = '${{ env.REGISTRY }}/opena2a-org/aim-dashboard';
const FRONTEND_HUB_IMAGE = 'opena2a/aim-dashboard';
const PUSH_DIGEST = '${{ steps.push.outputs.digest }}';

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

function assertPublishWorkflowShape(workflow: PublishWorkflow): void {
  const paths = workflow.on?.push?.paths ?? [];
  expect(paths, 'on.push.paths must contain the frontend Dockerfile').toContain(
    'infrastructure/docker/Dockerfile.frontend',
  );
  expect(paths, 'on.push.paths must contain apps/web/**').toContain('apps/web/**');
  expect(workflow.env?.REGISTRY, 'env.REGISTRY must be ghcr.io').toBe('ghcr.io');

  const build = frontendBuildStep(workflow);
  const withMap = build.with ?? {};
  expect(withMap.file, 'the build step must build infrastructure/docker/Dockerfile.frontend').toBe(
    'infrastructure/docker/Dockerfile.frontend',
  );
  expect(withMap.context, 'the build step context must be .').toBe('.');
  expect(withMap.platforms, 'the build step must build linux/amd64,linux/arm64').toBe(
    'linux/amd64,linux/arm64',
  );
  expect(withMap.push, 'the build step must push').toBe(true);
  expect(withMap.provenance, 'the build step must attach provenance').toBe(true);
  expect(withMap.sbom, 'the build step must attach an sbom').toBe(true);

  const steps = workflow.jobs?.['build-frontend']?.steps ?? [];
  const metadata = steps.filter(
    (s) => typeof s.uses === 'string' && s.uses.startsWith('docker/metadata-action'),
  );
  expect(metadata.length, 'build-frontend must carry two docker/metadata-action steps').toBe(2);
  const images = metadata.map((s) => s.with?.images);
  expect(images, `one metadata step must name ${FRONTEND_GHCR_IMAGE}`).toContain(
    FRONTEND_GHCR_IMAGE,
  );
  expect(images, `one metadata step must name ${FRONTEND_HUB_IMAGE}`).toContain(FRONTEND_HUB_IMAGE);

  const runs = steps.filter((s) => typeof s.run === 'string');
  for (const reference of [
    `cosign sign --yes "${FRONTEND_GHCR_IMAGE}@\${DIGEST}"`,
    `cosign sign --yes "docker.io/${FRONTEND_HUB_IMAGE}@\${DIGEST}"`,
  ]) {
    const signing = runs.filter((s) => (s.run as string).includes(reference));
    expect(signing.length, `build-frontend must carry a run step containing ${reference}`).toBe(1);
    expect(
      signing[0].env?.DIGEST,
      `the step signing ${reference} must carry env.DIGEST ${PUSH_DIGEST}`,
    ).toBe(PUSH_DIGEST);
  }

  const attest = steps.filter(
    (s) => typeof s.uses === 'string' && s.uses.startsWith('actions/attest-build-provenance'),
  );
  expect(attest.length, 'build-frontend must carry one actions/attest-build-provenance step').toBe(
    1,
  );
  expect(attest[0].with?.['subject-name'], 'the attestation subject must be the GHCR image').toBe(
    FRONTEND_GHCR_IMAGE,
  );
  expect(attest[0].with?.['push-to-registry'], 'the attestation must be pushed').toBe(true);

  // The file the step builds exists at the repository root and passes the check.
  const built = join(REPO_ROOT, withMap.file as string);
  expect(existsSync(built), `${String(withMap.file)} must exist relative to the repository root`).toBe(
    true,
  );
  const text = readFileSync(built, 'utf-8');
  expect(
    refusedLines(text),
    `${String(withMap.file)} must pass the install check: ${describeFailures(text)}`,
  ).toEqual([]);
}

// ---------------------------------------------------------------------------

describe('both frontend Dockerfiles install from the committed lockfile with scripts disabled', () => {
  it('AC1 infrastructure/docker/Dockerfile.frontend, the file docker-publish.yml builds, copies apps/web/package.json and apps/web/package-lock.json literally, runs exactly one npm ci --ignore-scripts [--legacy-peer-deps], invokes no other npm verb than ci and run build, drops RUN npm install --legacy-peer-deps and keeps RUN npm run build', () => {
    expect(
      frontendBuildStep(readPublishWorkflow()).with?.file,
      'docker-publish.yml build-frontend must build infrastructure/docker/Dockerfile.frontend',
    ).toBe('infrastructure/docker/Dockerfile.frontend');

    assertInstallsFromTheCommittedLockfile(readFileSync(ROOT_DOCKERFILE, 'utf-8'), {
      label: 'infrastructure/docker/Dockerfile.frontend',
      stage: 'builder',
      optionalArgs: ['--legacy-peer-deps'],
      absentLine: 'RUN npm install --legacy-peer-deps',
    });
  });

  it('AC2 apps/backend/infrastructure/docker/Dockerfile.frontend, the twin, copies both files literally in its deps stage, runs exactly one npm ci --ignore-scripts [--legacy-peer-deps] [--include=dev], invokes no other npm verb than ci and run build, drops RUN npm install --production=false --legacy-peer-deps and keeps RUN npm run build', () => {
    assertInstallsFromTheCommittedLockfile(readFileSync(TWIN_DOCKERFILE, 'utf-8'), {
      label: 'apps/backend/infrastructure/docker/Dockerfile.frontend',
      stage: 'deps',
      optionalArgs: ['--legacy-peer-deps', '--include=dev'],
      absentLine: 'RUN npm install --production=false --legacy-peer-deps',
    });
  });

  it('AC3 the install check refuses the two pre-fix texts by line, admits the repaired text, and names the right line for every planted install form', () => {
    for (const fixture of AC3_CASES) {
      expect(
        refusedLines(fixture.text),
        `${fixture.name}: ${describeFailures(fixture.text) || 'no failure reported'}`,
      ).toEqual(fixture.refused);
    }
  });

  it('AC4 every Dockerfile the tree walk finds passes the install check, and the population carries the path docker-publish.yml builds', () => {
    const population = walkDockerfiles(REPO_ROOT).map(repoRelative);
    // Report the reach of the walk, not only its verdict.
    console.log(`AC4 Dockerfile population (${population.length}): ${population.join(', ')}`);

    const built = frontendBuildStep(readPublishWorkflow()).with?.file;
    expect(typeof built, 'the build-frontend build step must carry a file: key').toBe('string');
    expect(
      population,
      `a walk rooted at ${REPO_ROOT} must reach the file docker-publish.yml builds`,
    ).toContain(built as string);

    for (const path of population) {
      const text = readFileSync(join(REPO_ROOT, path), 'utf-8');
      expect(
        refusedLines(text),
        `${path} must pass the install check: ${describeFailures(text)}`,
      ).toEqual([]);
    }
  });

  it('AC5 apps/web/package-lock.json is lockfileVersion 3, pins one next at the exact version apps/web/package.json names, carries the manifest maps the root lockfile also carries for apps/web, and records only name/version/integrity triples the root package-lock.json records; each planted divergence is named by key', () => {
    const nested = readJson<Lockfile>(join(WEB_DIR, 'package-lock.json'));
    const manifest = readJson<Manifest>(join(WEB_DIR, 'package.json'));
    const root = readJson<Lockfile>(ROOT_LOCKFILE);

    const delivered = checkLockfileParity(nested, manifest, root);
    expect(delivered, describeLockFailures(delivered)).toEqual([]);

    const entries = Object.keys(nested.packages ?? {}).filter((k) => k !== '');
    console.log(
      `AC5 apps/web/package-lock.json: ${entries.length} entries held to package-lock.json; next pinned at ${String(manifest.dependencies?.next)}`,
    );

    const keysOf = (failures: LockFailure[]) => failures.map((f) => f.key);

    // One range of the nested root dependencies differs from the manifest.
    const rangeDrift = clone(nested);
    (rangeDrift.packages as Record<string, LockEntry>)[''].dependencies = {
      ...(nested.packages?.['']?.dependencies ?? {}),
      zod: '^0.0.1-planted',
    };
    expect(keysOf(checkLockfileParity(rangeDrift, manifest, root))).toContain('zod');

    // The nested next is a version the root lockfile does not carry for next.
    const nextDrift = clone(nested);
    const nextKey = Object.keys(nextDrift.packages ?? {}).find(isNextKey) as string;
    (nextDrift.packages as Record<string, LockEntry>)[nextKey].version = '0.0.1-planted';
    expect(keysOf(checkLockfileParity(nextDrift, manifest, root))).toContain(nextKey);

    // One entry carries an integrity the root lockfile does not carry.
    const integrityDrift = clone(nested);
    const planted = entries.find(
      (k) => (integrityDrift.packages as Record<string, LockEntry>)[k].link !== true,
    ) as string;
    (integrityDrift.packages as Record<string, LockEntry>)[planted].integrity = 'sha512-planted';
    expect(keysOf(checkLockfileParity(integrityDrift, manifest, root))).toContain(planted);

    // The manifest pins next with a caret in front of the delivered version.
    const caret = clone(manifest);
    caret.dependencies = { ...manifest.dependencies, next: `^${String(manifest.dependencies?.next)}` };
    expect(keysOf(checkLockfileParity(nested, caret, root))).toContain('next');
  });

  it('AC6 docker-publish.yml keeps its signed, attested shape and builds a file that exists and passes the install check; a scratch copy naming an absent file or lacking a cosign step fails', () => {
    assertPublishWorkflowShape(readPublishWorkflow());

    const absentFile = readPublishWorkflow();
    (frontendBuildStep(absentFile).with as Record<string, unknown>).file =
      'infrastructure/docker/Dockerfile.frontend.absent';
    expect(() => assertPublishWorkflowShape(absentFile)).toThrow();

    const noHubSigning = readPublishWorkflow();
    const job = noHubSigning.jobs?.['build-frontend'] as { steps?: WorkflowStep[] };
    job.steps = (job.steps ?? []).filter(
      (s) => !(typeof s.run === 'string' && s.run.includes('cosign sign --yes "docker.io/')),
    );
    expect(() => assertPublishWorkflowShape(noHubSigning)).toThrow();
  });
});
