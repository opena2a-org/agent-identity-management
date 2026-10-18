import { describe, it, expect } from 'vitest';
import { existsSync, readFileSync } from 'fs';
import { join } from 'path';
import { load } from 'js-yaml';

/**
 * The aim-dashboard image takes no API URL when it is built.
 *
 * The dashboard reads NEXT_PUBLIC_API_URL from the container's environment each
 * time it renders a page, and hands it to the browser
 * (apps/web/lib/runtime-config.ts). next build writes a NEXT_PUBLIC_API_URL it
 * reads into the browser's code, where no setting of the running container
 * changes it. docker-publish.yml passed the build argument
 * NEXT_PUBLIC_API_URL=http://localhost:8080 to the frontend Dockerfile, and while
 * that file declared ARG NEXT_PUBLIC_API_URL, next build read it: an image built
 * that way held http://localhost:8080 in 2 files under .next/static.
 *
 * A build argument reaches the RUN instructions of a stage through an ARG of its
 * name. This cell holds each frontend Dockerfile to naming NEXT_PUBLIC_API_URL in
 * comments only: no ARG declares it, no ENV sets it in any stage (an ENV in the
 * runtime stage goes into the image's configuration, the environment a container
 * starts with), and no RUN assigns it. The files are the one docker-publish.yml's build-frontend job
 * builds, the one docker-compose.yml's frontend service builds, and the twin
 * under apps/backend.
 *
 * The check also runs over the text the published file carried when it declared
 * the ARG, so it keeps refusing that shape after the tree stops carrying it.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');
const PUBLISH_WORKFLOW = join(REPO_ROOT, '.github', 'workflows', 'docker-publish.yml');
const COMPOSE_FILE = join(REPO_ROOT, 'docker-compose.yml');
const TWIN_DOCKERFILE = 'apps/backend/infrastructure/docker/Dockerfile.frontend';

const VARIABLE = 'NEXT_PUBLIC_API_URL';
const NAMES_VARIABLE = /\bNEXT_PUBLIC_API_URL\b/;

// ---------------------------------------------------------------------------
// The check
// ---------------------------------------------------------------------------

interface InstructionLine {
  /** 1-indexed line number. */
  line: number;
  /** Keyword of the instruction the line belongs to. */
  keyword: string;
  text: string;
}

/** Delimiters of the BuildKit heredocs a line opens (`<<EOF`, `<<-"EOF"`). */
function heredocDelimiters(line: string): string[] {
  return [...line.matchAll(/<<-?\s*(['"]?)([A-Za-z_][A-Za-z0-9_]*)\1/g)].map((m) => m[2]);
}

/**
 * The lines of a Dockerfile outside comments and blank lines, each with the
 * keyword of the instruction it belongs to. A backslash continuation and a
 * heredoc body belong to the instruction they continue; a comment line inside a
 * continuation is skipped, and a heredoc body is read whole.
 */
function instructionLines(text: string): InstructionLine[] {
  const out: InstructionLine[] = [];
  let keyword = '';
  let continued = false;
  let heredocs: string[] = [];
  text.split('\n').forEach((raw, index) => {
    const trimmed = raw.trim();
    if (heredocs.length > 0) {
      out.push({ line: index + 1, keyword, text: raw });
      if (trimmed === heredocs[0]) heredocs.shift();
      return;
    }
    if (trimmed === '' || trimmed.startsWith('#')) return;
    if (!continued) keyword = trimmed.split(/\s+/, 1)[0].toUpperCase();
    out.push({ line: index + 1, keyword, text: raw });
    continued = /\\\s*$/.test(raw);
    heredocs = heredocDelimiters(raw);
  });
  return out;
}

/** Each line of a Dockerfile outside comments that names NEXT_PUBLIC_API_URL; empty when none does. */
function checkTakesNoApiUrl(text: string): string[] {
  return instructionLines(text)
    .filter((l) => NAMES_VARIABLE.test(l.text))
    .map((l) => `line ${l.line}: ${l.keyword} names ${VARIABLE}`);
}

// ---------------------------------------------------------------------------
// Tree reads
// ---------------------------------------------------------------------------

interface WorkflowStep {
  uses?: string;
  with?: Record<string, unknown>;
}

interface PublishWorkflow {
  jobs?: Record<string, { steps?: WorkflowStep[] }>;
}

/** The `file:` of the one docker/build-push-action step of docker-publish.yml's build-frontend job. */
function publishedDockerfile(): string {
  const workflow = load(readFileSync(PUBLISH_WORKFLOW, 'utf-8')) as PublishWorkflow;
  const builds = (workflow.jobs?.['build-frontend']?.steps ?? []).filter(
    (s) => typeof s.uses === 'string' && s.uses.startsWith('docker/build-push-action'),
  );
  expect(builds.length, 'docker-publish.yml build-frontend carries exactly one docker/build-push-action step').toBe(1);
  const file = builds[0].with?.file;
  expect(typeof file, 'docker-publish.yml build-frontend names its Dockerfile with file:').toBe('string');
  return file as string;
}

interface ComposeFile {
  services?: Record<string, { build?: { context?: string; dockerfile?: string } }>;
}

/** The Dockerfile docker-compose.yml's frontend service builds, relative to the repository root. */
function composeDockerfile(): string {
  const compose = load(readFileSync(COMPOSE_FILE, 'utf-8')) as ComposeFile;
  const build = compose.services?.frontend?.build;
  expect(build?.context, 'docker-compose.yml builds the frontend from the repository root').toBe('.');
  expect(typeof build?.dockerfile, 'docker-compose.yml names the frontend Dockerfile').toBe('string');
  return build?.dockerfile as string;
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

/** The published file as it stood when it declared the ARG, shortened and with no base digests. */
const BEFORE_FIX = `FROM node:22-alpine3.21 AS builder

# Accept build argument
ARG NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL

WORKDIR /app
COPY apps/web/package.json apps/web/package-lock.json ./
RUN npm ci --ignore-scripts --legacy-peer-deps
COPY apps/web/ ./
RUN npm run build

FROM alpine:3.21
COPY --from=builder /usr/local/bin/node /usr/local/bin/node
WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
CMD ["node", "server.js"]
`;

const DECLARATION =
  '# Accept build argument\nARG NEXT_PUBLIC_API_URL\nENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL\n';

const REPAIRED = BEFORE_FIX.replace(
  DECLARATION,
  '# The dashboard reads NEXT_PUBLIC_API_URL from the container environment.\n',
);

// ---------------------------------------------------------------------------

describe('the aim-dashboard image takes no API URL when it is built', () => {
  it('the check refuses the text that declared ARG NEXT_PUBLIC_API_URL and set ENV NEXT_PUBLIC_API_URL from it', () => {
    expect(checkTakesNoApiUrl(BEFORE_FIX)).toEqual([
      'line 4: ARG names NEXT_PUBLIC_API_URL',
      'line 5: ENV names NEXT_PUBLIC_API_URL',
    ]);
  });

  it('the check passes the text with the two lines removed, and a comment that names the variable', () => {
    expect(REPAIRED).not.toBe(BEFORE_FIX);
    expect(checkTakesNoApiUrl(REPAIRED)).toEqual([]);
  });

  it('the check refuses each planted form of the variable, at the line that names it', () => {
    const cases: Array<[string, string, string]> = [
      ['an ARG alone', REPAIRED.replace('WORKDIR /app\nCOPY apps/web/package.json', 'WORKDIR /app\nARG NEXT_PUBLIC_API_URL\nCOPY apps/web/package.json'), 'line 6: ARG names'],
      ['an ARG with a default', REPAIRED.replace('RUN npm run build', 'ARG NEXT_PUBLIC_API_URL=http://localhost:8080\nRUN npm run build'), 'line 9: ARG names'],
      ['an ENV of a literal URL', REPAIRED.replace('RUN npm run build', 'ENV NEXT_PUBLIC_API_URL=http://localhost:8080\nRUN npm run build'), 'line 9: ENV names'],
      ['an ENV in the legacy form', REPAIRED.replace('RUN npm run build', 'ENV NEXT_PUBLIC_API_URL http://localhost:8080\nRUN npm run build'), 'line 9: ENV names'],
      ['an ENV on a continuation line', REPAIRED.replace('ENV PORT=3000', 'ENV PORT=3000 \\\n    NEXT_PUBLIC_API_URL=http://localhost:8080'), 'line 16: ENV names'],
      ['an ENV in the runtime stage', REPAIRED.replace('ENV PORT=3000', 'ENV PORT=3000\nENV NEXT_PUBLIC_API_URL=http://localhost:8080'), 'line 16: ENV names'],
      ['a RUN that assigns it to the build', REPAIRED.replace('RUN npm run build', 'RUN NEXT_PUBLIC_API_URL=http://localhost:8080 npm run build'), 'line 9: RUN names'],
      ['a heredoc RUN that writes it to .env.production', REPAIRED.replace('RUN npm run build', 'RUN <<EOF\necho NEXT_PUBLIC_API_URL=http://localhost:8080 > .env.production\nnpm run build\nEOF'), 'line 10: RUN names'],
    ];
    for (const [label, text, expected] of cases) {
      expect(text, `${label}: the fixture changed`).not.toBe(REPAIRED);
      const failures = checkTakesNoApiUrl(text);
      expect(failures, `${label}: ${JSON.stringify(failures)}`).toEqual([`${expected} ${VARIABLE}`]);
    }
  });

  it('the check passes another NEXT_PUBLIC_ variable, a longer name that starts with NEXT_PUBLIC_API_URL, and a comment inside a continuation', () => {
    const cases: Array<[string, string]> = [
      ['another NEXT_PUBLIC_ variable', REPAIRED.replace('ENV PORT=3000', 'ENV PORT=3000\nENV NEXT_PUBLIC_APP_NAME=AIM')],
      ['a longer name', REPAIRED.replace('ENV PORT=3000', 'ENV PORT=3000\nENV NEXT_PUBLIC_API_URL_NOTE=unused')],
      ['a comment inside a continuation', REPAIRED.replace('ENV PORT=3000', 'ENV PORT=3000 \\\n# NEXT_PUBLIC_API_URL is read when a page renders\n    NODE_OPTIONS=--no-warnings')],
    ];
    for (const [label, text] of cases) {
      expect(text, `${label}: the fixture changed`).not.toBe(REPAIRED);
      expect(checkTakesNoApiUrl(text), label).toEqual([]);
    }
  });

  it('the file docker-publish.yml builds, the file docker-compose.yml builds and the twin name NEXT_PUBLIC_API_URL in comments only', () => {
    const files = [...new Set([publishedDockerfile(), composeDockerfile(), TWIN_DOCKERFILE])];
    for (const file of files) {
      const path = join(REPO_ROOT, file);
      expect(existsSync(path), `${file} exists`).toBe(true);
      expect(checkTakesNoApiUrl(readFileSync(path, 'utf-8')), file).toEqual([]);
    }
  });

  it('the check refuses the file docker-publish.yml builds with the ARG and the ENV put back after its first FROM', () => {
    const file = publishedDockerfile();
    const lines = readFileSync(join(REPO_ROOT, file), 'utf-8').split('\n');
    const from = lines.findIndex((l) => /^FROM\s/i.test(l));
    expect(from, `${file} has a FROM line`).toBeGreaterThanOrEqual(0);
    lines.splice(from + 1, 0, 'ARG NEXT_PUBLIC_API_URL', 'ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL');
    expect(checkTakesNoApiUrl(lines.join('\n'))).toEqual([
      `line ${from + 2}: ARG names ${VARIABLE}`,
      `line ${from + 3}: ENV names ${VARIABLE}`,
    ]);
  });
});
