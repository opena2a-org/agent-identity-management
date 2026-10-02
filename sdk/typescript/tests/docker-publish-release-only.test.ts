import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync, chmodSync } from 'fs';
import { join } from 'path';
import { spawnSync } from 'child_process';
import { load } from 'js-yaml';

/**
 * QGF-273 — docker-publish.yml publishes only from a platform-v tag push.
 *
 * Before this unit every merge to `main` under the workflow's path filter, and
 * every manual dispatch, pushed `latest` and `edge` for both images on both
 * registries: a merge was a publish. The workflow is now split at the job
 * boundary. Every main-branch and manual run is a read-only verify build
 * (`contents: read`, no secret, `push: false`, no outputs); the two publish
 * jobs are gated at job level on a push of a `platform-v*` tag, keep the
 * write permissions, logins, cosign and attest steps, build the same inputs
 * the verify jobs build, read no Actions cache, compute only the anchored
 * X.Y.Z, X.Y and X tag lines (`latest` follows from metadata-action's
 * default), and refuse, before any login, a tag that is not the newest
 * unpublished platform-vX.Y.Z.
 *
 *   AC1  triggers and registry unchanged;
 *   AC2  a non-publish run holds no write permission, no secret, no push;
 *   AC3  only build-backend and build-frontend can reach a registry, with the
 *        base's publish shape and a literal, per-job concurrency group;
 *   AC4  the verify build is the publish build (with-map parity by exclusion);
 *   AC5  a publish build reads nothing a merge-path run can write;
 *   AC6  the only tags a run can compute are a release's own;
 *   AC7  the release guard, structurally and over seventeen stubbed cases;
 *   AC8  each planted regression is refused by the assertion named for it;
 *   AC9  no in-repo text still promises a merge-built image.
 *
 * The AC8 plants are literals or programmatic edits of the parsed document,
 * never reads of a second tracked file, so the cells keep refusing the
 * pre-fix shape long after the tree stops carrying it.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');
const SDK_ROOT = join(__dirname, '..');
const WORKFLOW_PATH = join(__dirname, '..', '..', '..', '.github', 'workflows', 'docker-publish.yml');

const PUBLISH_IF = "${{ github.event_name == 'push' && startsWith(github.ref, 'refs/tags/platform-v') }}";
const PUBLISH_JOBS = ['build-backend', 'build-frontend'] as const;
const PUBLISH_PERMISSIONS = {
  contents: 'read',
  packages: 'write',
  'id-token': 'write',
  attestations: 'write',
};
const VERIFY_USES = [
  'actions/checkout@',
  'docker/setup-qemu-action@',
  'docker/setup-buildx-action@',
  'docker/build-push-action@',
];
const PUBLISH_USES = [
  'actions/checkout@',
  'docker/setup-qemu-action@',
  'docker/setup-buildx-action@',
  'docker/metadata-action@',
  'docker/build-push-action@',
  'sigstore/cosign-installer@',
  'actions/attest-build-provenance@',
];
const VERIFY_BUILD_WITH_KEYS = [
  'context',
  'file',
  'platforms',
  'push',
  'cache-from',
  'cache-to',
  'provenance',
  'sbom',
  'build-args',
];
const DOCKERFILE_OF: Record<string, string> = {
  'build-backend': 'infrastructure/docker/Dockerfile.backend',
  'build-frontend': 'infrastructure/docker/Dockerfile.frontend',
};
const IMAGES_OF: Record<string, { ghcr: string; hub: string }> = {
  'build-backend': { ghcr: '${{ env.REGISTRY }}/opena2a-org/aim-server', hub: 'opena2a/aim-server' },
  'build-frontend': { ghcr: '${{ env.REGISTRY }}/opena2a-org/aim-dashboard', hub: 'opena2a/aim-dashboard' },
};
/** The registry references the AC7 guard of each job may ask docker about. */
const REGISTRY_REFS_OF: Record<string, { ghcr: string; hub: string }> = {
  'build-backend': { ghcr: 'ghcr.io/opena2a-org/aim-server', hub: 'docker.io/opena2a/aim-server' },
  'build-frontend': { ghcr: 'ghcr.io/opena2a-org/aim-dashboard', hub: 'docker.io/opena2a/aim-dashboard' },
};
const META_TAG_LINES = [
  'type=match,pattern=^platform-v(\\d+\\.\\d+\\.\\d+)$,group=1',
  'type=match,pattern=^platform-v(\\d+\\.\\d+)\\.\\d+$,group=1',
  'type=match,pattern=^platform-v(\\d+)\\.\\d+\\.\\d+$,group=1',
];
const BUILD_TAG_LINES = ['${{ steps.meta-ghcr.outputs.tags }}', '${{ steps.meta-hub.outputs.tags }}'];
const BUILD_LABELS = '${{ steps.meta-ghcr.outputs.labels }}';
const REPOSITORY = 'opena2a-org/agent-identity-management';

// ---------------------------------------------------------------------------
// Document helpers
// ---------------------------------------------------------------------------

type Dict = Record<string, unknown>;
type Step = Dict;
interface Job extends Dict {
  steps?: Step[];
}
interface Workflow extends Dict {
  jobs?: Record<string, Job>;
}

function isDict(v: unknown): v is Dict {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

function readWorkflow(): Workflow {
  return load(readFileSync(WORKFLOW_PATH, 'utf-8')) as Workflow;
}

function parseWorkflow(text: string): Workflow {
  return load(text) as Workflow;
}

function clone<T>(v: T): T {
  return JSON.parse(JSON.stringify(v)) as T;
}

function deepEqual(a: unknown, b: unknown): boolean {
  if (Object.is(a, b)) return true;
  if (Array.isArray(a) && Array.isArray(b)) {
    return a.length === b.length && a.every((x, i) => deepEqual(x, b[i]));
  }
  if (isDict(a) && isDict(b)) {
    const ka = Object.keys(a).sort();
    const kb = Object.keys(b).sort();
    return deepEqual(ka, kb) && ka.every((k) => deepEqual(a[k], b[k]));
  }
  return false;
}

function fail(message: string): never {
  throw new Error(message);
}

function jobs(doc: Workflow): Array<[string, Job]> {
  const j = doc.jobs;
  if (!isDict(j)) fail('jobs: the workflow has no jobs mapping');
  return Object.entries(j).map(([name, job]) => {
    if (!isDict(job)) fail(`job ${name}: is not a mapping`);
    return [name, job as Job];
  });
}

function isPublishJob(job: Job): boolean {
  return job.if === PUBLISH_IF;
}

function publishJobs(doc: Workflow): Array<[string, Job]> {
  return jobs(doc).filter(([, job]) => isPublishJob(job));
}

function verifyJobs(doc: Workflow): Array<[string, Job]> {
  return jobs(doc).filter(([, job]) => !isPublishJob(job));
}

function stepsOf(name: string, job: Job): Step[] {
  const s = job.steps;
  if (!Array.isArray(s)) fail(`job ${name}: has no steps list`);
  return s.map((step, i) => {
    if (!isDict(step)) fail(`job ${name} step ${i}: is not a mapping`);
    return step;
  });
}

function stepLabel(jobName: string, index: number, step: Step): string {
  const name = typeof step.name === 'string' ? step.name : typeof step.uses === 'string' ? step.uses : '(unnamed)';
  return `job ${jobName} step ${index} "${name}"`;
}

function usesOf(step: Step): string {
  return typeof step.uses === 'string' ? step.uses : '';
}

function runOf(step: Step): string {
  return typeof step.run === 'string' ? step.run : '';
}

function withOf(step: Step): Dict {
  return isDict(step.with) ? step.with : {};
}

function nonBlankLines(block: unknown): string[] {
  if (typeof block !== 'string') return [];
  return block
    .split('\n')
    .map((l) => l.trim())
    .filter((l) => l !== '');
}

function startsWithAny(s: string, prefixes: readonly string[]): boolean {
  return prefixes.some((p) => s.startsWith(p));
}

function omit(map: Dict, keys: readonly string[]): Dict {
  const out: Dict = {};
  for (const [k, v] of Object.entries(map)) if (!keys.includes(k)) out[k] = v;
  return out;
}

/** Every string in the document, with its key path. */
function walkStrings(value: unknown, path: Array<string | number>, visit: (s: string, path: Array<string | number>) => void): void {
  if (typeof value === 'string') {
    visit(value, path);
  } else if (Array.isArray(value)) {
    value.forEach((v, i) => walkStrings(v, [...path, i], visit));
  } else if (isDict(value)) {
    for (const [k, v] of Object.entries(value)) {
      visit(k, [...path, k]);
      walkStrings(v, [...path, k], visit);
    }
  }
}

function describePath(doc: Workflow, path: Array<string | number>): string {
  if (path[0] === 'jobs' && typeof path[1] === 'string') {
    const jobName = path[1];
    if (path[2] === 'steps' && typeof path[3] === 'number') {
      const step = (doc.jobs?.[jobName]?.steps ?? [])[path[3]] ?? {};
      return `${stepLabel(jobName, path[3], step)} ${path.slice(4).join('.')}`.trim();
    }
    return `job ${jobName} ${path.slice(2).join('.')}`.trim();
  }
  return path.join('.');
}

const EXPRESSION = /\$\{\{([\s\S]*?)\}\}/g;

function expressionsNamingSecrets(s: string): string[] {
  const hits: string[] = [];
  for (const m of s.matchAll(EXPRESSION)) {
    if (/\bsecrets\b/i.test(m[1])) hits.push(m[0]);
  }
  return hits;
}

function theBuildStep(jobName: string, job: Job): [number, Step] {
  const steps = stepsOf(jobName, job);
  const builds = steps.map((s, i) => [i, s] as [number, Step]).filter(([, s]) => usesOf(s).startsWith('docker/build-push-action'));
  if (builds.length !== 1) {
    fail(`job ${jobName}: expected exactly one docker/build-push-action step, found ${builds.length}`);
  }
  return builds[0];
}

function requirePublish(doc: Workflow, jobName: string): Job {
  const job = doc.jobs?.[jobName];
  if (!isDict(job)) fail(`job ${jobName}: absent`);
  if (!isPublishJob(job)) {
    fail(`job ${jobName}: is not a publish job (its job-level if is ${JSON.stringify(job.if)}, not the publish expression)`);
  }
  return job;
}

// ---------------------------------------------------------------------------
// AC1 — triggers and registry
// ---------------------------------------------------------------------------

function assertAC1(doc: Workflow): void {
  const on = doc.on;
  if (!isDict(on)) fail('on: must be a mapping');
  const keys = Object.keys(on).sort();
  if (!deepEqual(keys, ['push', 'workflow_dispatch'])) {
    fail(`on: keys are [${keys.join(', ')}], not exactly push and workflow_dispatch`);
  }
  const push = on.push;
  if (!isDict(push)) fail('on.push: must be a mapping');
  if (!deepEqual(push.tags, ['platform-v*'])) {
    fail(`on.push.tags: is ${JSON.stringify(push.tags)}, not exactly ['platform-v*']`);
  }
  if (!deepEqual(push.branches, ['main'])) {
    fail(`on.push.branches: is ${JSON.stringify(push.branches)}, not exactly ['main']`);
  }
  const paths = [
    'apps/backend/**',
    'apps/web/**',
    'sdk/**',
    'infrastructure/docker/Dockerfile.backend',
    'infrastructure/docker/Dockerfile.frontend',
  ];
  if (!deepEqual(push.paths, paths)) {
    fail(`on.push.paths: is ${JSON.stringify(push.paths)}, not the five entries in order`);
  }
  const env = doc.env;
  if (!isDict(env) || env.REGISTRY !== 'ghcr.io') {
    fail(`env.REGISTRY: is ${JSON.stringify(isDict(env) ? env.REGISTRY : undefined)}, not ghcr.io`);
  }
}

// ---------------------------------------------------------------------------
// AC2 — a non-publish run holds no write permission, no secret, no push
// ---------------------------------------------------------------------------

function assertAC2(doc: Workflow): void {
  if (!deepEqual(doc.permissions, { contents: 'read' })) {
    fail(`permissions: workflow-level permissions are ${JSON.stringify(doc.permissions)}, not exactly {contents: read}`);
  }

  const stripped = clone(doc);
  for (const [name] of publishJobs(doc)) delete (stripped.jobs as Record<string, Job>)[name];
  walkStrings(stripped, [], (s, path) => {
    const hits = expressionsNamingSecrets(s);
    if (hits.length > 0) {
      fail(`${describePath(stripped, path)}: names secrets outside a publish job in ${hits.join(' ')}`);
    }
  });

  for (const [name, job] of verifyJobs(doc)) {
    if ('permissions' in job && !deepEqual(job.permissions, { contents: 'read' })) {
      fail(`job ${name}: verify job permissions are ${JSON.stringify(job.permissions)}, not {contents: read}`);
    }
    for (const key of ['uses', 'secrets', 'outputs', 'if']) {
      if (key in job) fail(`job ${name}: verify job carries the key ${key}`);
    }
    const steps = stepsOf(name, job);
    if (steps.length === 0) fail(`job ${name}: verify job has no steps`);
    steps.forEach((step, i) => {
      const label = stepLabel(name, i, step);
      if ('run' in step) fail(`${label}: a verify step carries run`);
      if ('if' in step) fail(`${label}: a verify step carries if`);
      const uses = usesOf(step);
      if (!startsWithAny(uses, VERIFY_USES)) {
        fail(`${label}: uses ${JSON.stringify(step.uses)} is not one of ${VERIFY_USES.join(', ')}`);
      }
      if (uses.startsWith('docker/build-push-action')) {
        const w = withOf(step);
        if (w.push !== false) fail(`${label}: with.push is ${JSON.stringify(w.push)}, not boolean false`);
        for (const k of Object.keys(w)) {
          if (!VERIFY_BUILD_WITH_KEYS.includes(k)) {
            fail(`${label}: with.${k} is not an admitted verify build input`);
          }
        }
      }
    });
  }
}

// ---------------------------------------------------------------------------
// AC3 — only the two publish jobs can reach a registry
// ---------------------------------------------------------------------------

function assertAC3(doc: Workflow): void {
  const names = publishJobs(doc).map(([n]) => n);
  for (const expected of PUBLISH_JOBS) {
    if (!names.includes(expected)) {
      const job = doc.jobs?.[expected];
      fail(`job ${expected}: lacks the publish if (job-level if is ${JSON.stringify(isDict(job) ? job.if : undefined)})`);
    }
  }
  for (const n of names) {
    if (!(PUBLISH_JOBS as readonly string[]).includes(n)) fail(`job ${n}: carries the publish if but is not build-backend or build-frontend`);
  }

  const groups: string[] = [];
  for (const name of PUBLISH_JOBS) {
    const job = requirePublish(doc, name);
    if (!deepEqual(job.permissions, PUBLISH_PERMISSIONS)) {
      fail(`job ${name}: permissions are ${JSON.stringify(job.permissions)}, not the publish set`);
    }
    if ('uses' in job) fail(`job ${name}: a publish job carries a job-level uses`);
    const steps = stepsOf(name, job);
    steps.forEach((step, i) => {
      if ('if' in step) fail(`${stepLabel(name, i, step)}: a publish step carries if`);
    });
    const [bi, build] = theBuildStep(name, job);
    const w = withOf(build);
    const expectedWith: Dict = {
      push: true,
      provenance: true,
      sbom: true,
      context: '.',
      platforms: 'linux/amd64,linux/arm64',
      file: DOCKERFILE_OF[name],
    };
    for (const [k, v] of Object.entries(expectedWith)) {
      if (w[k] !== v) fail(`${stepLabel(name, bi, build)}: with.${k} is ${JSON.stringify(w[k])}, not ${JSON.stringify(v)}`);
    }
    const count = (pred: (s: Step) => boolean): number => steps.filter(pred).length;
    const logins = count((s) => runOf(s).includes('docker login'));
    if (logins !== 2) fail(`job ${name}: ${logins} run steps contain docker login, not 2`);
    const signs = count((s) => runOf(s).includes('cosign sign'));
    if (signs !== 2) fail(`job ${name}: ${signs} run steps contain cosign sign, not 2`);
    const installers = count((s) => usesOf(s).startsWith('sigstore/cosign-installer'));
    if (installers !== 1) fail(`job ${name}: ${installers} sigstore/cosign-installer steps, not 1`);
    const attests = steps.map((s, i) => [i, s] as [number, Step]).filter(([, s]) => usesOf(s).startsWith('actions/attest-build-provenance'));
    if (attests.length !== 1) fail(`job ${name}: ${attests.length} actions/attest-build-provenance steps, not 1`);
    if (withOf(attests[0][1])['push-to-registry'] !== true) {
      fail(`${stepLabel(name, attests[0][0], attests[0][1])}: with.push-to-registry is not boolean true`);
    }
    const concurrency = job.concurrency;
    if (!isDict(concurrency)) fail(`job ${name}: has no job-level concurrency map`);
    if (typeof concurrency.group !== 'string' || concurrency.group.includes('${{')) {
      fail(`job ${name}: concurrency.group ${JSON.stringify(concurrency.group)} is not a literal string`);
    }
    if (concurrency['cancel-in-progress'] !== false) {
      fail(`job ${name}: concurrency.cancel-in-progress is ${JSON.stringify(concurrency['cancel-in-progress'])}, not boolean false`);
    }
    groups.push(concurrency.group);
  }
  if (groups[0] === groups[1]) {
    fail(`job build-backend and job build-frontend: share the concurrency group ${JSON.stringify(groups[0])}`);
  }
}

// ---------------------------------------------------------------------------
// AC4 — the verify build is the publish build
// ---------------------------------------------------------------------------

function assertAC4(doc: Workflow): void {
  interface BuildRef {
    job: string;
    index: number;
    step: Step;
    publish: boolean;
    file: string;
  }
  const builds: BuildRef[] = [];
  for (const [name, job] of jobs(doc)) {
    stepsOf(name, job).forEach((step, index) => {
      if (!usesOf(step).startsWith('docker/build-push-action')) return;
      const file = withOf(step).file;
      if (typeof file !== 'string') return;
      builds.push({ job: name, index, step, publish: isPublishJob(job), file });
    });
  }
  const files = [...new Set(builds.map((b) => b.file))].sort();
  for (const expected of Object.values(DOCKERFILE_OF)) {
    if (!files.includes(expected)) fail(`file ${expected}: no docker/build-push-action step names it`);
  }
  for (const file of files) {
    const here = builds.filter((b) => b.file === file);
    const list = (xs: BuildRef[]): string => xs.map((b) => stepLabel(b.job, b.index, b.step)).join(', ') || 'none';
    const verify = here.filter((b) => !b.publish);
    const publish = here.filter((b) => b.publish);
    if (verify.length !== 1) {
      fail(`file ${file}: expected exactly one verify build step, found ${verify.length} (${list(verify)}; publish: ${list(publish)})`);
    }
    if (publish.length !== 1) {
      fail(`file ${file}: expected exactly one publish build step, found ${publish.length} (${list(publish)}; verify: ${list(verify)})`);
    }
    const v = omit(withOf(verify[0].step), ['push', 'cache-from', 'cache-to']);
    const p = omit(withOf(publish[0].step), ['push', 'tags', 'labels']);
    if (!deepEqual(v, p)) {
      const keys = [...new Set([...Object.keys(v), ...Object.keys(p)])].filter((k) => !deepEqual(v[k], p[k]));
      fail(
        `${stepLabel(verify[0].job, verify[0].index, verify[0].step)} and ${stepLabel(publish[0].job, publish[0].index, publish[0].step)}: ` +
          `build inputs differ on ${keys.join(', ')} (verify ${JSON.stringify(keys.map((k) => v[k]))}, publish ${JSON.stringify(keys.map((k) => p[k]))})`,
      );
    }
  }
}

// ---------------------------------------------------------------------------
// AC5 — a publish build reads nothing a merge-path run can write
// ---------------------------------------------------------------------------

function assertAC5(doc: Workflow): void {
  for (const name of PUBLISH_JOBS) {
    const job = requirePublish(doc, name);
    const steps = stepsOf(name, job);
    const [bi, build] = theBuildStep(name, job);
    for (const k of ['cache-from', 'cache-to']) {
      if (k in withOf(build)) fail(`${stepLabel(name, bi, build)}: a publish build step carries with.${k}`);
    }
    if (job['runs-on'] !== 'ubuntu-latest') {
      fail(`job ${name}: runs-on is ${JSON.stringify(job['runs-on'])}, not ubuntu-latest`);
    }
    const runs: Array<[number, Step]> = [];
    steps.forEach((step, i) => {
      const label = stepLabel(name, i, step);
      const uses = usesOf(step);
      if (uses.startsWith('actions/cache')) fail(`${label}: a publish job reads the Actions cache through ${uses}`);
      if ('uses' in step) {
        if (!startsWithAny(uses, PUBLISH_USES)) fail(`${label}: uses ${JSON.stringify(step.uses)} is outside the closed publish list`);
      } else if ('run' in step) {
        runs.push([i, step]);
      } else {
        fail(`${label}: carries neither uses nor run`);
      }
      const run = runOf(step);
      if (run.includes('gh run download')) fail(`${label}: run text downloads a workflow artifact (gh run download)`);
      if (run.includes('/artifacts')) fail(`${label}: run text reaches /artifacts`);
    });
    if (runs.length !== 5) {
      fail(`job ${name}: ${runs.length} run steps, not exactly five (${runs.map(([i, s]) => stepLabel(name, i, s)).join(', ')})`);
    }
    const logins = runs.filter(([, s]) => runOf(s).includes('docker login'));
    const signs = runs.filter(([, s]) => runOf(s).includes('cosign sign'));
    if (logins.length !== 2) fail(`job ${name}: ${logins.length} run steps contain docker login, not 2`);
    if (signs.length !== 2) fail(`job ${name}: ${signs.length} run steps contain cosign sign, not 2`);
    const [gi, guard] = runs[0];
    if (runOf(guard).includes('docker login') || runOf(guard).includes('cosign sign')) {
      fail(`${stepLabel(name, gi, guard)}: the first run step is a login or signing step, not the release guard`);
    }
  }
}

// ---------------------------------------------------------------------------
// AC6 — the only tags a run can compute are a release's own
// ---------------------------------------------------------------------------

function assertAC6(doc: Workflow): void {
  for (const name of PUBLISH_JOBS) {
    const job = requirePublish(doc, name);
    const steps = stepsOf(name, job);
    const metas = steps.map((s, i) => [i, s] as [number, Step]).filter(([, s]) => usesOf(s).startsWith('docker/metadata-action'));
    if (metas.length !== 2) fail(`job ${name}: ${metas.length} docker/metadata-action steps, not exactly 2`);
    const expected = [
      { id: 'meta-ghcr', images: IMAGES_OF[name].ghcr },
      { id: 'meta-hub', images: IMAGES_OF[name].hub },
    ];
    metas.forEach(([i, step], k) => {
      const label = stepLabel(name, i, step);
      if (step.id !== expected[k].id) fail(`${label}: id is ${JSON.stringify(step.id)}, not ${expected[k].id} (metadata steps must be meta-ghcr then meta-hub)`);
      const w = withOf(step);
      if (w.images !== expected[k].images) fail(`${label}: with.images is ${JSON.stringify(w.images)}, not ${expected[k].images}`);
      const lines = nonBlankLines(w.tags);
      if (!deepEqual(lines, META_TAG_LINES)) {
        fail(`${label}: with.tags lines are ${JSON.stringify(lines)}, not exactly the three anchored match lines`);
      }
      if ('flavor' in w) fail(`${label}: carries with.flavor`);
    });
    const [bi, build] = theBuildStep(name, job);
    const label = stepLabel(name, bi, build);
    const w = withOf(build);
    const tagLines = nonBlankLines(w.tags);
    if (!deepEqual(tagLines, BUILD_TAG_LINES)) {
      fail(`${label}: with.tags lines are ${JSON.stringify(tagLines)}, not exactly the two metadata outputs`);
    }
    if (w.labels !== BUILD_LABELS) fail(`${label}: with.labels is ${JSON.stringify(w.labels)}, not ${BUILD_LABELS}`);
  }
  for (const [name, job] of verifyJobs(doc)) {
    stepsOf(name, job).forEach((step, i) => {
      if (usesOf(step).startsWith('docker/metadata-action')) {
        fail(`${stepLabel(name, i, step)}: a docker/metadata-action step outside a publish job`);
      }
    });
  }
}

// ---------------------------------------------------------------------------
// AC7 — the release guard
// ---------------------------------------------------------------------------

function guardStep(doc: Workflow, name: string): [number, Step] {
  const job = requirePublish(doc, name);
  const runs = stepsOf(name, job).map((s, i) => [i, s] as [number, Step]).filter(([, s]) => 'run' in s);
  if (runs.length === 0) fail(`job ${name}: has no run step, so no release guard`);
  return runs[0];
}

function assertAC7Shape(doc: Workflow): void {
  for (const name of PUBLISH_JOBS) {
    const job = requirePublish(doc, name);
    const steps = stepsOf(name, job);
    const [gi, guard] = guardStep(doc, name);
    const label = stepLabel(name, gi, guard);
    const run = runOf(guard);
    if (run.includes('docker login')) {
      fail(`${label}: the first run step is a docker login; the release guard must precede every login`);
    }
    steps.forEach((step, i) => {
      if (runOf(step).includes('docker login') && i < gi) {
        fail(`${label}: ${stepLabel(name, i, step)} logs in before the release guard`);
      }
    });
    if ('if' in guard) fail(`${label}: the release guard carries if`);
    walkStrings(guard, [], (s, path) => {
      if (/secrets\./i.test(s)) fail(`${label} ${path.join('.')}: the release guard references secrets.`);
    });
    if (run.includes('${{')) fail(`${label}: the release guard's run text contains an inline expression`);
    if (!isDict(guard.env)) fail(`${label}: the release guard has no env map`);
    const gits = [...run.matchAll(/\bgit\s+([a-z-]+)/g)].map((m) => m[1]);
    if (gits.length === 0 || !run.includes('git ls-remote --tags')) {
      fail(`${label}: the release guard does not read the tag list with git ls-remote --tags`);
    }
    for (const sub of gits) {
      if (sub !== 'ls-remote') fail(`${label}: the release guard runs git ${sub}, reading the checkout rather than the forge`);
    }
    if (!/git ls-remote --tags "?https:\/\/github\.com\//.test(run)) {
      fail(`${label}: git ls-remote --tags is not aimed at https://github.com/ plus the repository`);
    }
    if (!run.includes('docker buildx imagetools inspect')) {
      fail(`${label}: the release guard does not read the registry with docker buildx imagetools inspect`);
    }
  }
}

interface RegistryAnswer {
  exit: number;
  out: string;
}
interface GuardCase {
  id: string;
  tag: string;
  gitExit?: number;
  tags: string[];
  /** Registry answers keyed by image reference (with version); absent refs make the stub exit 99. */
  registry: (refs: { ghcr: string; hub: string }) => Record<string, RegistryAnswer>;
  exitZero: boolean;
  prints?: string[];
  notPrints?: string[];
  printsRegistryOutput?: boolean;
  askedExactly?: boolean;
}

const PRESENT = (ref: string): RegistryAnswer => ({
  exit: 0,
  out: `Name:      ${ref}\nMediaType: application/vnd.oci.image.index.v1+json\nDigest:    sha256:0000000000000000000000000000000000000000000000000000000000000000`,
});
const ABSENT = (ref: string): RegistryAnswer => ({ exit: 1, out: `ERROR: ${ref}: not found` });
const TIMEOUT = (ref: string): RegistryAnswer => ({
  exit: 1,
  out: `ERROR: failed to do request: Head "https://${ref.split('/')[0]}/v2/": dial tcp 140.82.121.34:443: i/o timeout`,
});
const NOT_FOUND_404: RegistryAnswer = {
  exit: 1,
  out: 'ERROR: failed to authorize: failed to fetch anonymous token: unexpected status: 404 Not Found',
};
const NO_DOCKER: RegistryAnswer = { exit: 127, out: 'bash: docker: command not found' };

const V = '1.1.0';
const withVersion = (refs: { ghcr: string; hub: string }, v: string): { ghcr: string; hub: string } => ({
  ghcr: `${refs.ghcr}:${v}`,
  hub: `${refs.hub}:${v}`,
});
const both = (refs: { ghcr: string; hub: string }, v: string, a: (ref: string) => RegistryAnswer): Record<string, RegistryAnswer> => {
  const r = withVersion(refs, v);
  return { [r.ghcr]: a(r.ghcr), [r.hub]: a(r.hub) };
};
const bothFixed = (refs: { ghcr: string; hub: string }, v: string, a: RegistryAnswer): Record<string, RegistryAnswer> => {
  const r = withVersion(refs, v);
  return { [r.ghcr]: a, [r.hub]: a };
};
const NEXT_FIX = 'push platform-v1.1.1 from the fixed commit';
const RE_RUN = 're-run once the registry answers; the check repeats';

const GUARD_CASES: GuardCase[] = [
  { id: '(a) pre-release suffix', tag: 'platform-v1.1.0-rc.1', tags: ['platform-v1.0.0', 'platform-v1.1.0'], registry: (r) => both(r, V, ABSENT), exitZero: false, prints: ['platform-vX.Y.Z'] },
  { id: '(a) two-part version', tag: 'platform-v1.1', tags: ['platform-v1.0.0', 'platform-v1.1.0'], registry: (r) => both(r, V, ABSENT), exitZero: false, prints: ['platform-vX.Y.Z'] },
  { id: '(b) not the newest tag', tag: 'platform-v1.0.1', tags: ['platform-v1.0.0', 'platform-v1.0.1', 'platform-v1.1.0'], registry: (r) => ({ ...both(r, V, ABSENT), ...both(r, '1.0.1', ABSENT) }), exitZero: false, prints: [NEXT_FIX] },
  { id: '(c) version resolves on GHCR', tag: 'platform-v1.1.0', tags: ['platform-v1.0.0', 'platform-v1.1.0'], registry: (r) => { const v = withVersion(r, V); return { [v.ghcr]: PRESENT(v.ghcr), [v.hub]: ABSENT(v.hub) }; }, exitZero: false, prints: [NEXT_FIX] },
  { id: '(c) version resolves only on Docker Hub', tag: 'platform-v1.1.0', tags: ['platform-v1.0.0', 'platform-v1.1.0'], registry: (r) => { const v = withVersion(r, V); return { [v.ghcr]: ABSENT(v.ghcr), [v.hub]: PRESENT(v.hub) }; }, exitZero: false, prints: [NEXT_FIX] },
  { id: '(d) registry answers neither presence nor absence', tag: 'platform-v1.1.0', tags: ['platform-v1.0.0', 'platform-v1.1.0'], registry: (r) => both(r, V, TIMEOUT), exitZero: false, prints: [RE_RUN], printsRegistryOutput: true },
  { id: '(d) anonymous token 404', tag: 'platform-v1.1.0', tags: ['platform-v1.0.0', 'platform-v1.1.0'], registry: (r) => bothFixed(r, V, NOT_FOUND_404), exitZero: false, prints: [RE_RUN], printsRegistryOutput: true },
  { id: '(ok) newest unpublished 1.1.0', tag: 'platform-v1.1.0', tags: ['platform-v1.0.0', 'platform-v1.1.0'], registry: (r) => both(r, V, ABSENT), exitZero: true, askedExactly: true },
  { id: '(ok) newest unpublished 1.10.0 sorts above 1.9.0', tag: 'platform-v1.10.0', tags: ['platform-v1.9.0', 'platform-v1.10.0'], registry: (r) => both(r, '1.10.0', ABSENT), exitZero: true, askedExactly: true },
  { id: '(e) git ls-remote fails', tag: 'platform-v1.1.0', gitExit: 128, tags: ['platform-v1.0.0', 'platform-v1.1.0'], registry: (r) => both(r, V, ABSENT), exitZero: false, notPrints: ['from the fixed commit'] },
  { id: '(e) tag list lists only 1.0.0', tag: 'platform-v1.1.0', tags: ['platform-v1.0.0'], registry: (r) => both(r, V, ABSENT), exitZero: false, notPrints: ['from the fixed commit'] },
  { id: '(e) tag list lists nothing', tag: 'platform-v1.1.0', tags: [], registry: (r) => both(r, V, ABSENT), exitZero: false, notPrints: ['from the fixed commit'] },
  { id: '(f) docker is not a command', tag: 'platform-v1.1.0', tags: ['platform-v1.0.0', 'platform-v1.1.0'], registry: (r) => bothFixed(r, V, NO_DOCKER), exitZero: false, printsRegistryOutput: true },
  { id: '(g) warning line before the not-found line', tag: 'platform-v1.1.0', tags: ['platform-v1.0.0', 'platform-v1.1.0'], registry: (r) => { const v = withVersion(r, V); return { [v.ghcr]: { exit: 1, out: `WARNING: example extra line\nERROR: ${v.ghcr}: not found` }, [v.hub]: ABSENT(v.hub) }; }, exitZero: false, prints: [RE_RUN], printsRegistryOutput: true },
  { id: '(g) warning line after the not-found line', tag: 'platform-v1.1.0', tags: ['platform-v1.0.0', 'platform-v1.1.0'], registry: (r) => { const v = withVersion(r, V); return { [v.ghcr]: { exit: 1, out: `ERROR: ${v.ghcr}: not found\nWARNING: example extra line` }, [v.hub]: ABSENT(v.hub) }; }, exitZero: false, prints: [RE_RUN], printsRegistryOutput: true },
  { id: '(g) not-found line for another version', tag: 'platform-v1.1.0', tags: ['platform-v1.0.0', 'platform-v1.1.0'], registry: (r) => { const v = withVersion(r, V); return { [v.ghcr]: { exit: 1, out: `ERROR: ${r.ghcr}:1.0.0: not found` }, [v.hub]: ABSENT(v.hub) }; }, exitZero: false, prints: [RE_RUN], printsRegistryOutput: true },
  { id: '(g) manifest unknown', tag: 'platform-v1.1.0', tags: ['platform-v1.0.0', 'platform-v1.1.0'], registry: (r) => { const v = withVersion(r, V); return { [v.ghcr]: { exit: 1, out: 'ERROR: manifest unknown' }, [v.hub]: ABSENT(v.hub) }; }, exitZero: false, prints: [RE_RUN], printsRegistryOutput: true },
];

const GIT_STUB = `#!/usr/bin/env bash
# QGF-273 stub git: answers "git ls-remote --tags <url>" from $QGF_CASE_DIR; any other invocation is refused.
set -u
if [ "\${1:-}" != "ls-remote" ] || [ "\${2:-}" != "--tags" ] || [ -z "\${3:-}" ] || [ -n "\${4:-}" ]; then
  echo "stub git: unexpected invocation: $*" >&2
  exit 99
fi
case "$3" in
  https://github.com/${REPOSITORY}|https://github.com/${REPOSITORY}.git) ;;
  *) echo "stub git: unexpected remote: $3" >&2; exit 99 ;;
esac
code=$(cat "$QGF_CASE_DIR/git-exit")
if [ "$code" != "0" ]; then
  echo "fatal: unable to access 'https://github.com/${REPOSITORY}/': Could not resolve host: github.com" >&2
  exit "$code"
fi
n=0
while IFS= read -r t; do
  [ -z "$t" ] && continue
  n=$((n+1))
  printf '%040d\\trefs/tags/%s\\n' "$n" "$t"
  printf '%040d\\trefs/tags/%s^{}\\n' "$n" "$t"
done < "$QGF_CASE_DIR/tags"
exit 0
`;

const DOCKER_STUB = `#!/usr/bin/env bash
# QGF-273 stub docker: answers "docker buildx imagetools inspect <ref>" from $QGF_CASE_DIR/registry; logs every ref asked.
set -u
if [ "\${1:-}" != "buildx" ] || [ "\${2:-}" != "imagetools" ] || [ "\${3:-}" != "inspect" ] || [ -z "\${4:-}" ] || [ -n "\${5:-}" ]; then
  echo "stub docker: unexpected invocation: $*" >&2
  exit 99
fi
ref="$4"
printf '%s\\n' "$ref" >> "$QGF_CASE_DIR/log"
f="$QGF_CASE_DIR/registry/$(printf '%s' "$ref" | tr '/:' '__')"
if [ ! -f "$f" ]; then
  echo "stub docker: no answer configured for $ref" >&2
  exit 99
fi
code=$(head -n 1 "$f")
if [ "$code" = "0" ]; then
  tail -n +2 "$f"
else
  tail -n +2 "$f" >&2
fi
exit "$code"
`;

interface GuardRun {
  status: number | null;
  output: string;
  asked: string[];
}

let scratch = '';
let stubDir = '';
let caseCount = 0;

function mintStubs(): void {
  const cacheRoot = join(SDK_ROOT, 'node_modules', '.cache');
  mkdirSync(cacheRoot, { recursive: true });
  scratch = mkdtempSync(join(cacheRoot, 'qgf-273-'));
  stubDir = join(scratch, 'bin');
  mkdirSync(stubDir);
  for (const [name, text] of [
    ['git', GIT_STUB],
    ['docker', DOCKER_STUB],
  ] as const) {
    const p = join(stubDir, name);
    writeFileSync(p, text, { mode: 0o755 });
    chmodSync(p, 0o755);
  }
}

function substitutedEnv(step: Step, tag: string): Record<string, string> {
  const env = isDict(step.env) ? step.env : {};
  const out: Record<string, string> = {};
  for (const [k, v] of Object.entries(env)) {
    let s = String(v);
    s = s.replace(/\$\{\{\s*github\.ref_name\s*\}\}/g, tag);
    s = s.replace(/\$\{\{\s*github\.ref\s*\}\}/g, `refs/tags/${tag}`);
    s = s.replace(/\$\{\{\s*github\.repository\s*\}\}/g, REPOSITORY);
    if (s.includes('${{')) throw new Error(`guard env ${k} carries an expression the harness does not substitute: ${s}`);
    out[k] = s;
  }
  return out;
}

function runGuard(doc: Workflow, jobName: string, c: GuardCase): GuardRun {
  const [, guard] = guardStep(doc, jobName);
  const caseDir = join(scratch, `case-${++caseCount}`);
  mkdirSync(join(caseDir, 'registry'), { recursive: true });
  writeFileSync(join(caseDir, 'git-exit'), String(c.gitExit ?? 0));
  writeFileSync(join(caseDir, 'tags'), c.tags.map((t) => `${t}\n`).join(''));
  writeFileSync(join(caseDir, 'log'), '');
  for (const [ref, answer] of Object.entries(c.registry(REGISTRY_REFS_OF[jobName]))) {
    writeFileSync(join(caseDir, 'registry', ref.replace(/[/:]/g, '_')), `${answer.exit}\n${answer.out}\n`);
  }
  const script = join(caseDir, 'guard.sh');
  writeFileSync(script, runOf(guard));
  const result = spawnSync('bash', ['-e', '-o', 'pipefail', script], {
    encoding: 'utf-8',
    env: {
      PATH: `${stubDir}:${process.env.PATH ?? ''}`,
      QGF_CASE_DIR: caseDir,
      ...substitutedEnv(guard, c.tag),
    },
  });
  if (result.error) throw result.error;
  const asked = readFileSync(join(caseDir, 'log'), 'utf-8').split('\n').filter((l) => l !== '');
  return { status: result.status, output: `${result.stdout}${result.stderr}`, asked };
}

function assertGuardCase(doc: Workflow, jobName: string, c: GuardCase): void {
  const r = runGuard(doc, jobName, c);
  const where = `${jobName} ${c.id}`;
  if (c.exitZero) {
    expect(r.status, `${where}: exit status (output: ${r.output})`).toBe(0);
  } else {
    expect(r.status, `${where}: exit status (output: ${r.output})`).not.toBe(0);
    expect(r.output, `${where}: output names the tag under test`).toContain(c.tag);
  }
  for (const p of c.prints ?? []) expect(r.output, `${where}: output`).toContain(p);
  for (const p of c.notPrints ?? []) expect(r.output, `${where}: output`).not.toContain(p);
  if (c.printsRegistryOutput) {
    const answers = Object.values(c.registry(REGISTRY_REFS_OF[jobName]));
    const lines = answers.flatMap((a) => a.out.split('\n')).filter((l) => l !== '');
    const printed = lines.filter((l) => r.output.includes(l));
    expect(printed.length, `${where}: the registry command's output is printed (output: ${r.output})`).toBeGreaterThan(0);
  }
  // Whatever the case, the guard asks docker about nothing outside the job's own pair at the tag's version.
  const refs = withVersion(REGISTRY_REFS_OF[jobName], c.tag.replace(/^platform-v/, ''));
  for (const a of r.asked) expect([refs.ghcr, refs.hub], `${where}: asked about a reference outside the job's pair`).toContain(a);
  if (c.askedExactly) {
    expect([...r.asked].sort(), `${where}: docker was asked about exactly the job's two references`).toEqual([refs.ghcr, refs.hub].sort());
  }
}

// ---------------------------------------------------------------------------
// AC8 — planted regressions
// ---------------------------------------------------------------------------

/** docker-publish.yml as at agent-identity-management c2a8ac5e21f129a47e84da177c42dcd1ef00572b (the base of this unit). */
const BASE_TEXT = `name: Docker Publish

on:
  push:
    tags:
      # Per CONTRIBUTING.md "Tag conventions" (PR #249). Bare \`v*\` tags
      # are retired for new releases — they matched the legacy Python SDK
      # path but never matched \`platform-v*\` which is the actual platform
      # release tag prefix today. SDK tags (\`sdk-py-v*\`, \`sdk-ts-v*\`,
      # \`sdk-java-v*\`) don't ship the platform images; their publish
      # workflows live in release.yml.
      - 'platform-v*'
    branches:
      - main
    paths:
      - 'apps/backend/**'
      - 'apps/web/**'
      - 'sdk/**'
      - 'infrastructure/docker/Dockerfile.backend'
      - 'infrastructure/docker/Dockerfile.frontend'
  workflow_dispatch:

permissions:
  contents: read
  packages: write
  id-token: write
  attestations: write

env:
  REGISTRY: ghcr.io

jobs:
  build-backend:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      # Registry logins are wrapped in a retry loop: transient registry 500s
      # (e.g. registry-1.docker.io HTTP 500, observed on run 27146602888) have
      # no built-in retry in docker/login-action and otherwise paint the repo
      # red until a manual re-run. Backoff is linear (10s, 20s, 30s, 40s).
      - name: Log in to Docker Hub
        env:
          DOCKERHUB_USERNAME: \${{ secrets.DOCKERHUB_USERNAME }}
          DOCKERHUB_TOKEN: \${{ secrets.DOCKERHUB_TOKEN }}
        run: |
          for attempt in 1 2 3 4 5; do
            if echo "$DOCKERHUB_TOKEN" | docker login -u "$DOCKERHUB_USERNAME" --password-stdin; then
              echo "Docker Hub login succeeded on attempt $attempt"
              exit 0
            fi
            echo "Docker Hub login attempt $attempt failed; retrying in $((attempt * 10))s..."
            sleep $((attempt * 10))
          done
          echo "Docker Hub login failed after 5 attempts"
          exit 1

      - name: Log in to GitHub Container Registry
        env:
          GHCR_TOKEN: \${{ secrets.GITHUB_TOKEN }}
          GHCR_USER: \${{ github.actor }}
        run: |
          for attempt in 1 2 3 4 5; do
            if echo "$GHCR_TOKEN" | docker login "$REGISTRY" -u "$GHCR_USER" --password-stdin; then
              echo "GHCR login succeeded on attempt $attempt"
              exit 0
            fi
            echo "GHCR login attempt $attempt failed; retrying in $((attempt * 10))s..."
            sleep $((attempt * 10))
          done
          echo "GHCR login failed after 5 attempts"
          exit 1

      - name: Extract metadata (GHCR)
        id: meta-ghcr
        uses: docker/metadata-action@v5
        with:
          images: \${{ env.REGISTRY }}/opena2a-org/aim-server
          tags: |
            type=raw,value=latest,enable={{is_default_branch}}
            type=raw,value=latest,enable=\${{ startsWith(github.ref, 'refs/tags/platform-v') }}
            type=match,pattern=platform-v(\\d+\\.\\d+\\.\\d+),group=1
            type=match,pattern=platform-v(\\d+\\.\\d+),group=1
            type=match,pattern=platform-v(\\d+),group=1
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=semver,pattern={{major}}
            type=edge,branch=main

      - name: Extract metadata (Docker Hub)
        id: meta-hub
        uses: docker/metadata-action@v5
        with:
          images: opena2a/aim-server
          tags: |
            type=raw,value=latest,enable={{is_default_branch}}
            type=raw,value=latest,enable=\${{ startsWith(github.ref, 'refs/tags/platform-v') }}
            type=match,pattern=platform-v(\\d+\\.\\d+\\.\\d+),group=1
            type=match,pattern=platform-v(\\d+\\.\\d+),group=1
            type=match,pattern=platform-v(\\d+),group=1
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=semver,pattern={{major}}
            type=edge,branch=main

      - name: Build and push image
        id: push
        uses: docker/build-push-action@v6
        with:
          context: .
          file: infrastructure/docker/Dockerfile.backend
          platforms: linux/amd64,linux/arm64
          push: true
          tags: |
            \${{ steps.meta-ghcr.outputs.tags }}
            \${{ steps.meta-hub.outputs.tags }}
          labels: \${{ steps.meta-ghcr.outputs.labels }}
          cache-from: type=gha,scope=backend
          cache-to: type=gha,mode=max,scope=backend
          provenance: true
          sbom: true
          build-args: |
            VERSION=\${{ startsWith(github.ref, 'refs/tags/') && github.ref_name || github.sha }}

      - name: Install cosign
        uses: sigstore/cosign-installer@v3

      - name: Sign GHCR image with cosign (keyless)
        env:
          DIGEST: \${{ steps.push.outputs.digest }}
        run: |
          cosign sign --yes "\${{ env.REGISTRY }}/opena2a-org/aim-server@\${DIGEST}"

      - name: Sign Docker Hub image with cosign (keyless)
        env:
          DIGEST: \${{ steps.push.outputs.digest }}
        run: |
          cosign sign --yes "docker.io/opena2a/aim-server@\${DIGEST}"

      - name: Attest build provenance
        uses: actions/attest-build-provenance@v2
        with:
          subject-name: \${{ env.REGISTRY }}/opena2a-org/aim-server
          subject-digest: \${{ steps.push.outputs.digest }}
          push-to-registry: true

  build-frontend:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      # See build-backend for rationale: retry transient registry 500s that
      # docker/login-action does not retry on its own.
      - name: Log in to Docker Hub
        env:
          DOCKERHUB_USERNAME: \${{ secrets.DOCKERHUB_USERNAME }}
          DOCKERHUB_TOKEN: \${{ secrets.DOCKERHUB_TOKEN }}
        run: |
          for attempt in 1 2 3 4 5; do
            if echo "$DOCKERHUB_TOKEN" | docker login -u "$DOCKERHUB_USERNAME" --password-stdin; then
              echo "Docker Hub login succeeded on attempt $attempt"
              exit 0
            fi
            echo "Docker Hub login attempt $attempt failed; retrying in $((attempt * 10))s..."
            sleep $((attempt * 10))
          done
          echo "Docker Hub login failed after 5 attempts"
          exit 1

      - name: Log in to GitHub Container Registry
        env:
          GHCR_TOKEN: \${{ secrets.GITHUB_TOKEN }}
          GHCR_USER: \${{ github.actor }}
        run: |
          for attempt in 1 2 3 4 5; do
            if echo "$GHCR_TOKEN" | docker login "$REGISTRY" -u "$GHCR_USER" --password-stdin; then
              echo "GHCR login succeeded on attempt $attempt"
              exit 0
            fi
            echo "GHCR login attempt $attempt failed; retrying in $((attempt * 10))s..."
            sleep $((attempt * 10))
          done
          echo "GHCR login failed after 5 attempts"
          exit 1

      - name: Extract metadata (GHCR)
        id: meta-ghcr
        uses: docker/metadata-action@v5
        with:
          images: \${{ env.REGISTRY }}/opena2a-org/aim-dashboard
          tags: |
            type=raw,value=latest,enable={{is_default_branch}}
            type=raw,value=latest,enable=\${{ startsWith(github.ref, 'refs/tags/platform-v') }}
            type=match,pattern=platform-v(\\d+\\.\\d+\\.\\d+),group=1
            type=match,pattern=platform-v(\\d+\\.\\d+),group=1
            type=match,pattern=platform-v(\\d+),group=1
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=semver,pattern={{major}}
            type=edge,branch=main

      - name: Extract metadata (Docker Hub)
        id: meta-hub
        uses: docker/metadata-action@v5
        with:
          images: opena2a/aim-dashboard
          tags: |
            type=raw,value=latest,enable={{is_default_branch}}
            type=raw,value=latest,enable=\${{ startsWith(github.ref, 'refs/tags/platform-v') }}
            type=match,pattern=platform-v(\\d+\\.\\d+\\.\\d+),group=1
            type=match,pattern=platform-v(\\d+\\.\\d+),group=1
            type=match,pattern=platform-v(\\d+),group=1
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=semver,pattern={{major}}
            type=edge,branch=main

      - name: Build and push image
        id: push
        uses: docker/build-push-action@v6
        with:
          context: .
          file: infrastructure/docker/Dockerfile.frontend
          platforms: linux/amd64,linux/arm64
          push: true
          tags: |
            \${{ steps.meta-ghcr.outputs.tags }}
            \${{ steps.meta-hub.outputs.tags }}
          labels: \${{ steps.meta-ghcr.outputs.labels }}
          cache-from: type=gha,scope=frontend
          cache-to: type=gha,mode=max,scope=frontend
          provenance: true
          sbom: true
          build-args: |
            NEXT_PUBLIC_API_URL=http://localhost:8080

      - name: Install cosign
        uses: sigstore/cosign-installer@v3

      - name: Sign GHCR image with cosign (keyless)
        env:
          DIGEST: \${{ steps.push.outputs.digest }}
        run: |
          cosign sign --yes "\${{ env.REGISTRY }}/opena2a-org/aim-dashboard@\${DIGEST}"

      - name: Sign Docker Hub image with cosign (keyless)
        env:
          DIGEST: \${{ steps.push.outputs.digest }}
        run: |
          cosign sign --yes "docker.io/opena2a/aim-dashboard@\${DIGEST}"

      - name: Attest build provenance
        uses: actions/attest-build-provenance@v2
        with:
          subject-name: \${{ env.REGISTRY }}/opena2a-org/aim-dashboard
          subject-digest: \${{ steps.push.outputs.digest }}
          push-to-registry: true
`;

type Assertion = (doc: Workflow) => void;
const ASSERTIONS: Record<string, Assertion> = {
  AC1: assertAC1,
  AC2: assertAC2,
  AC3: assertAC3,
  AC4: assertAC4,
  AC5: assertAC5,
  AC6: assertAC6,
  AC7: assertAC7Shape,
};

interface Plant {
  id: string;
  ac: keyof typeof ASSERTIONS;
  /** Every pattern the refusal must name (the job, the step, or the `on` key path). */
  names: RegExp[];
  apply: (doc: Workflow) => void;
}

function job(doc: Workflow, name: string): Job {
  const j = doc.jobs?.[name];
  if (!isDict(j)) throw new Error(`plant: job ${name} absent`);
  return j;
}

function firstVerifyJobName(doc: Workflow): string {
  const v = verifyJobs(doc);
  if (v.length === 0) throw new Error('plant: no verify job');
  return v[0][0];
}

function verifyJobBuilding(doc: Workflow, file: string): string {
  for (const [name, j] of verifyJobs(doc)) {
    for (const s of stepsOf(name, j)) {
      if (usesOf(s).startsWith('docker/build-push-action') && withOf(s).file === file) return name;
    }
  }
  throw new Error(`plant: no verify job builds ${file}`);
}

function buildStepOf(doc: Workflow, name: string): Step {
  return theBuildStep(name, job(doc, name))[1];
}

function metaStepOf(doc: Workflow, name: string, id: string): Step {
  const s = stepsOf(name, job(doc, name)).find((x) => x.id === id);
  if (!s) throw new Error(`plant: ${name} has no step ${id}`);
  return s;
}

function appendTagLine(step: Step, line: string): void {
  const w = withOf(step);
  w.tags = `${String(w.tags)}${line}\n`;
}

function firstRunIndex(doc: Workflow, name: string): number {
  const i = stepsOf(name, job(doc, name)).findIndex((s) => 'run' in s);
  if (i < 0) throw new Error(`plant: ${name} has no run step`);
  return i;
}

const CURL_ARTIFACT = 'curl -sL https://api.github.com/repos/opena2a-org/agent-identity-management/actions/artifacts/1/zip -o a.zip';

const PLANTS: Plant[] = [
  {
    id: 'M3 a verify job logs in with secrets and promotes edge to latest',
    ac: 'AC2',
    names: [/job verify-/, /step \d+ "/],
    apply: (d) => {
      const v = job(d, firstVerifyJobName(d));
      (v.steps as Step[]).push(
        { name: 'Log in to Docker Hub', uses: 'docker/login-action@v3', with: { username: '${{ secrets.DOCKERHUB_USERNAME }}', password: '${{ secrets.DOCKERHUB_TOKEN }}' } },
        { name: 'Promote', run: 'docker buildx imagetools create -t opena2a/aim-server:latest opena2a/aim-server:edge' },
      );
    },
  },
  { id: "M4 build-frontend's job-level if removed", ac: 'AC3', names: [/job build-frontend/], apply: (d) => { delete job(d, 'build-frontend').if; } },
  { id: 'M5 a verify job gains packages: write', ac: 'AC2', names: [/job verify-/], apply: (d) => { job(d, firstVerifyJobName(d)).permissions = { contents: 'read', packages: 'write' }; } },
  { id: 'M6 the workflow-level permissions gain packages: write', ac: 'AC2', names: [/^permissions/], apply: (d) => { (d.permissions as Dict).packages = 'write'; } },
  { id: "M7 build-backend's if drops the event_name clause", ac: 'AC3', names: [/job build-backend/], apply: (d) => { job(d, 'build-backend').if = "${{ startsWith(github.ref, 'refs/tags/platform-v') }}"; } },
  {
    id: 'M8 a verify build step gains an outputs push',
    ac: 'AC2',
    names: [/job verify-/, /step \d+ "/],
    apply: (d) => { withOf(buildStepOf(d, firstVerifyJobName(d))).outputs = 'type=image,name=ghcr.io/opena2a-org/aim-server:latest,push=true'; },
  },
  {
    id: "M9 a publish job's cosign sign step gains if: false",
    ac: 'AC3',
    names: [/job build-backend/, /step \d+ "Sign GHCR image/],
    apply: (d) => { const s = stepsOf('build-backend', job(d, 'build-backend')).find((x) => runOf(x).includes('cosign sign')) as Step; s.if = false; },
  },
  { id: 'M10 the workflow-level env gains a secret', ac: 'AC2', names: [/^env\.HUB/], apply: (d) => { (d.env as Dict).HUB = '${{ secrets.DOCKERHUB_TOKEN }}'; } },
  {
    id: "M12 the verify frontend build step's build-args drift",
    ac: 'AC4',
    names: [/job verify-/, /job build-frontend/, /step \d+ "/],
    apply: (d) => { withOf(buildStepOf(d, verifyJobBuilding(d, DOCKERFILE_OF['build-frontend'])))['build-args'] = 'NEXT_PUBLIC_API_URL=\n'; },
  },
  { id: "M13 build-backend's build step gains cache-from", ac: 'AC5', names: [/job build-backend/, /step \d+ "/], apply: (d) => { withOf(buildStepOf(d, 'build-backend'))['cache-from'] = 'type=gha,scope=backend'; } },
  { id: "M14 build-backend's build step gains target: prod", ac: 'AC4', names: [/job build-backend/, /step \d+ "/], apply: (d) => { withOf(buildStepOf(d, 'build-backend')).target = 'prod'; } },
  { id: 'M15 a type=raw,value=latest line added to a metadata tags block', ac: 'AC6', names: [/job build-backend/, /step \d+ "/], apply: (d) => { appendTagLine(metaStepOf(d, 'build-backend', 'meta-ghcr'), 'type=raw,value=latest'); } },
  {
    id: 'M16 the first metadata tags line loses its anchors',
    ac: 'AC6',
    names: [/job build-backend/, /step \d+ "/],
    apply: (d) => {
      const w = withOf(metaStepOf(d, 'build-backend', 'meta-ghcr'));
      const lines = String(w.tags).split('\n');
      lines[0] = 'type=match,pattern=platform-v(\\d+\\.\\d+\\.\\d+),group=1';
      w.tags = lines.join('\n');
    },
  },
  { id: 'M17 a type=semver line appended to a metadata tags block', ac: 'AC6', names: [/job build-frontend/, /step \d+ "/], apply: (d) => { appendTagLine(metaStepOf(d, 'build-frontend', 'meta-hub'), 'type=semver,pattern={{version}}'); } },
  { id: 'M18 flavor: latest=false added to one metadata step', ac: 'AC6', names: [/job build-backend/, /step \d+ "/], apply: (d) => { withOf(metaStepOf(d, 'build-backend', 'meta-hub')).flavor = 'latest=false'; } },
  { id: 'M19 a type=edge,branch=main line appended', ac: 'AC6', names: [/job build-backend/, /step \d+ "/], apply: (d) => { appendTagLine(metaStepOf(d, 'build-backend', 'meta-ghcr'), 'type=edge,branch=main'); } },
  { id: 'M19 a type=raw,value=edge line appended', ac: 'AC6', names: [/job build-frontend/, /step \d+ "/], apply: (d) => { appendTagLine(metaStepOf(d, 'build-frontend', 'meta-ghcr'), 'type=raw,value=edge'); } },
  { id: 'M20 a type=sha line appended', ac: 'AC6', names: [/job build-backend/, /step \d+ "/], apply: (d) => { appendTagLine(metaStepOf(d, 'build-backend', 'meta-hub'), 'type=sha'); } },
  {
    id: 'M21 the first run step of both publish jobs removed',
    ac: 'AC7',
    names: [/job build-backend/, /step \d+ "/],
    apply: (d) => { for (const n of PUBLISH_JOBS) (job(d, n).steps as Step[]).splice(firstRunIndex(d, n), 1); },
  },
  { id: "M22 a literal latest tag appended to build-backend's build tags", ac: 'AC6', names: [/job build-backend/, /step \d+ "Build and push image"/], apply: (d) => { appendTagLine(buildStepOf(d, 'build-backend'), 'ghcr.io/opena2a-org/aim-server:latest'); } },
  {
    id: 'M23 the first run step of build-backend gains an if',
    ac: 'AC3',
    names: [/job build-backend/, /step \d+ "/],
    apply: (d) => { (job(d, 'build-backend').steps as Step[])[firstRunIndex(d, 'build-backend')].if = "${{ github.ref_name != '' }}"; },
  },
  {
    id: 'M24 the first run step of build-backend moved after its two docker login steps',
    ac: 'AC7',
    names: [/job build-backend/, /step \d+ "/],
    apply: (d) => {
      const steps = job(d, 'build-backend').steps as Step[];
      const [guard] = steps.splice(firstRunIndex(d, 'build-backend'), 1);
      const logins = steps.map((s, i) => [i, s] as [number, Step]).filter(([, s]) => runOf(s).includes('docker login'));
      steps.splice(logins[logins.length - 1][0] + 1, 0, guard);
    },
  },
  { id: 'M25 a verify build step gains if: false', ac: 'AC2', names: [/job verify-/, /step \d+ "/], apply: (d) => { buildStepOf(d, firstVerifyJobName(d)).if = false; } },
  { id: 'M26 on.push.tags gains v*', ac: 'AC1', names: [/^on\.push\.tags/], apply: (d) => { (((d.on as Dict).push as Dict).tags as string[]).push('v*'); } },
  { id: 'M27 on gains pull_request', ac: 'AC1', names: [/^on:/], apply: (d) => { (d.on as Dict).pull_request = null; } },
  { id: "M28 build-backend's runs-on becomes self-hosted", ac: 'AC5', names: [/job build-backend/], apply: (d) => { job(d, 'build-backend')['runs-on'] = 'self-hosted'; } },
  { id: 'M29 build-backend gains an artifact-download action', ac: 'AC5', names: [/job build-backend/, /step \d+ "/], apply: (d) => { (job(d, 'build-backend').steps as Step[]).push({ name: 'Download', uses: 'dawidd6/action-download-artifact@v6' }); } },
  { id: "M30 build-backend's concurrency removed", ac: 'AC3', names: [/job build-backend/], apply: (d) => { delete job(d, 'build-backend').concurrency; } },
  { id: "M31 build-backend's concurrency group gains the ref", ac: 'AC3', names: [/job build-backend/], apply: (d) => { const c = job(d, 'build-backend').concurrency as Dict; c.group = `${String(c.group)}-\${{ github.ref }}`; } },
  { id: "M32 a verify job's first step gains a bracketed secrets expression", ac: 'AC2', names: [/job verify-/, /step 0 "/], apply: (d) => { (job(d, firstVerifyJobName(d)).steps as Step[])[0].env = { X: "${{ secrets['DOCKERHUB_TOKEN'] }}" }; } },
  { id: "M33 a verify job's checkout step gains with.token toJSON(secrets)", ac: 'AC2', names: [/job verify-/, /step \d+ "/], apply: (d) => { const s = (job(d, firstVerifyJobName(d)).steps as Step[]).find((x) => usesOf(x).startsWith('actions/checkout@')) as Step; s.with = { token: '${{ toJSON(secrets) }}' }; } },
  { id: "M34 a verify job's first step gains an upper-case SECRETS expression", ac: 'AC2', names: [/job verify-/, /step 0 "/], apply: (d) => { (job(d, firstVerifyJobName(d)).steps as Step[])[0].env = { X: '${{ SECRETS.DOCKERHUB_TOKEN }}' }; } },
  { id: 'M35 build-backend gains a run step that curls an artifact', ac: 'AC5', names: [/job build-backend/, /step \d+ "/], apply: (d) => { (job(d, 'build-backend').steps as Step[]).push({ name: 'Fetch artifact', run: CURL_ARTIFACT }); } },
  {
    id: "M36 the curl line appended to build-frontend's first docker login step",
    ac: 'AC5',
    names: [/job build-frontend/, /step \d+ "Log in to Docker Hub"/],
    apply: (d) => { const s = stepsOf('build-frontend', job(d, 'build-frontend')).find((x) => runOf(x).includes('docker login')) as Step; s.run = `${runOf(s)}${CURL_ARTIFACT}\n`; },
  },
];

function refusalOf(fn: () => void): string {
  try {
    fn();
  } catch (e) {
    return e instanceof Error ? e.message : String(e);
  }
  throw new Error('the assertion passed where a refusal was expected');
}

// ---------------------------------------------------------------------------
// AC9 — in-repo text
// ---------------------------------------------------------------------------

const AC9_FILES = ['README.md', 'infrastructure/DEPLOYMENT.md', 'sdk/python/README.md', 'sdk/python/CHANGELOG.md', 'docs/demo/stack/images.env'];
const AC9_FORBIDDEN = ['every build of `main`', 'Built from `main` on every push', 'image `edge`', '`edge` image', ':edge'];

// ---------------------------------------------------------------------------

describe('QGF-273 docker-publish.yml publishes only from a platform-v tag push', () => {
  beforeAll(() => {
    mintStubs();
  });
  afterAll(() => {
    if (scratch !== '') rmSync(scratch, { recursive: true, force: true });
  });

  it('QGF-273.AC1 docker-publish.yml keeps the base triggers (push on platform-v* tags, main under the five paths, workflow_dispatch) and REGISTRY ghcr.io', () => {
    const doc = readWorkflow();
    assertAC1(doc);
    const on = doc.on as Dict;
    expect(Object.keys(on).sort()).toEqual(['push', 'workflow_dispatch']);
    expect((on.push as Dict).tags).toEqual(['platform-v*']);
    expect((on.push as Dict).branches).toEqual(['main']);
    expect((on.push as Dict).paths).toEqual([
      'apps/backend/**',
      'apps/web/**',
      'sdk/**',
      'infrastructure/docker/Dockerfile.backend',
      'infrastructure/docker/Dockerfile.frontend',
    ]);
    expect((doc.env as Dict).REGISTRY).toBe('ghcr.io');
  });

  it('QGF-273.AC2 a run of docker-publish.yml that is not a publish run holds no write permission, no secret and no push', () => {
    const doc = readWorkflow();
    assertAC2(doc);
    expect(doc.permissions).toEqual({ contents: 'read' });
    const verify = verifyJobs(doc);
    expect(verify.length, 'the workflow carries verify jobs').toBeGreaterThan(0);
    for (const [name, j] of verify) {
      const steps = stepsOf(name, j);
      expect(steps.length, `${name} steps`).toBeGreaterThan(0);
      for (const s of steps) {
        expect('run' in s, `${name} step run`).toBe(false);
        expect('if' in s, `${name} step if`).toBe(false);
        if (usesOf(s).startsWith('docker/build-push-action')) expect(withOf(s).push).toBe(false);
      }
    }
  });

  it('QGF-273.AC3 only build-backend and build-frontend carry the publish if, with the publish permissions, logins, cosign, attest and a literal per-job concurrency group', () => {
    const doc = readWorkflow();
    assertAC3(doc);
    expect(publishJobs(doc).map(([n]) => n).sort()).toEqual(['build-backend', 'build-frontend']);
    for (const name of PUBLISH_JOBS) {
      const j = job(doc, name);
      expect(j.permissions).toEqual(PUBLISH_PERMISSIONS);
      expect(withOf(buildStepOf(doc, name)).push).toBe(true);
      expect((j.concurrency as Dict)['cancel-in-progress']).toBe(false);
    }
    expect((job(doc, 'build-backend').concurrency as Dict).group).not.toBe((job(doc, 'build-frontend').concurrency as Dict).group);
  });

  it('QGF-273.AC4 the verify build is the publish build: for both Dockerfile paths the with maps agree once push, cache-from, cache-to, tags and labels are excluded', () => {
    const doc = readWorkflow();
    assertAC4(doc);
    for (const name of PUBLISH_JOBS) {
      const file = DOCKERFILE_OF[name];
      const v = omit(withOf(buildStepOf(doc, verifyJobBuilding(doc, file))), ['push', 'cache-from', 'cache-to']);
      const p = omit(withOf(buildStepOf(doc, name)), ['push', 'tags', 'labels']);
      expect(v, `${file}`).toEqual(p);
    }
  });

  it('QGF-273.AC5 a publish build reads nothing a merge-path run can write: no cache, ubuntu-latest, a closed uses list, five run steps and no artifact download', () => {
    const doc = readWorkflow();
    assertAC5(doc);
    for (const name of PUBLISH_JOBS) {
      const w = withOf(buildStepOf(doc, name));
      expect('cache-from' in w).toBe(false);
      expect('cache-to' in w).toBe(false);
      expect(job(doc, name)['runs-on']).toBe('ubuntu-latest');
      expect(stepsOf(name, job(doc, name)).filter((s) => 'run' in s)).toHaveLength(5);
    }
  });

  it('QGF-273.AC6 the only tags a run can compute are a release\'s own: three anchored match lines per metadata step, no flavor, build tags from the two metadata outputs, no metadata step outside a publish job', () => {
    const doc = readWorkflow();
    assertAC6(doc);
    for (const name of PUBLISH_JOBS) {
      expect(nonBlankLines(withOf(metaStepOf(doc, name, 'meta-ghcr')).tags)).toEqual(META_TAG_LINES);
      expect(nonBlankLines(withOf(metaStepOf(doc, name, 'meta-hub')).tags)).toEqual(META_TAG_LINES);
      expect(nonBlankLines(withOf(buildStepOf(doc, name)).tags)).toEqual(BUILD_TAG_LINES);
      expect(withOf(buildStepOf(doc, name)).labels).toBe(BUILD_LABELS);
    }
  });

  it('QGF-273.AC7 the release guard is the first run step of each publish job, precedes every login, carries no if, no secrets and no inline expression, and reads tags only through git ls-remote', () => {
    const doc = readWorkflow();
    assertAC7Shape(doc);
    for (const name of PUBLISH_JOBS) {
      const [gi, guard] = guardStep(doc, name);
      const steps = stepsOf(name, job(doc, name));
      const loginIndexes = steps.map((s, i) => (runOf(s).includes('docker login') ? i : -1)).filter((i) => i >= 0);
      expect(loginIndexes).toHaveLength(2);
      for (const li of loginIndexes) expect(gi).toBeLessThan(li);
      expect(runOf(guard)).not.toContain('${{');
      expect(JSON.stringify(guard)).not.toMatch(/secrets\./i);
    }
  });

  for (const name of PUBLISH_JOBS) {
    it(`QGF-273.AC7 ${name}'s release guard, run under bash -e -o pipefail with stub git and docker, refuses every non-release case and admits the newest unpublished tag over the seventeen cases`, () => {
      const doc = readWorkflow();
      expect(GUARD_CASES).toHaveLength(17);
      for (const c of GUARD_CASES) assertGuardCase(doc, name, c);
    });
  }

  it('QGF-273.AC8 on the delivered file every workflow-shape assertion passes', () => {
    const doc = readWorkflow();
    for (const [ac, assertion] of Object.entries(ASSERTIONS)) {
      expect(() => assertion(doc), ac).not.toThrow();
    }
  });

  it('QGF-273.AC8 (i) the base text (two push: true, four edge lines, four is_default_branch lines, zero if keys, workflow-level packages: write) is refused by AC2, AC3, AC4, AC6 and AC7, each naming a job', () => {
    const lines = BASE_TEXT.split('\n');
    expect(lines.filter((l) => l.trim() === 'push: true')).toHaveLength(2);
    expect(lines.filter((l) => l.trim() === 'type=edge,branch=main')).toHaveLength(4);
    expect(lines.filter((l) => l.includes('enable={{is_default_branch}}'))).toHaveLength(4);
    const base = parseWorkflow(BASE_TEXT);
    let ifs = 0;
    walkStrings(base, [], (s, path) => {
      if (path[path.length - 1] === 'if' && s === 'if') ifs++;
    });
    expect(ifs).toBe(0);
    expect((base.permissions as Dict).packages).toBe('write');
    const named: Record<string, RegExp> = {
      AC2: /^permissions:|job build-(backend|frontend)/,
      AC3: /job build-(backend|frontend)/,
      AC4: /job build-(backend|frontend) step \d+ "/,
      AC6: /job build-(backend|frontend)/,
      AC7: /job build-(backend|frontend)/,
    };
    for (const [ac, re] of Object.entries(named)) {
      const message = refusalOf(() => ASSERTIONS[ac](parseWorkflow(BASE_TEXT)));
      expect(message, `${ac} refusal names what it found`).toMatch(re);
    }
  });

  for (const plant of PLANTS) {
    it(`QGF-273.AC8 (${plant.id}) is refused by ${plant.ac} naming what it found`, () => {
      const doc = readWorkflow();
      plant.apply(doc);
      const message = refusalOf(() => ASSERTIONS[plant.ac](doc));
      for (const re of plant.names) expect(message, `${plant.id}: refusal names ${String(re)}`).toMatch(re);
    });
  }

  it('QGF-273.AC9 no in-repo text still promises a merge-built image: the five files carry none of the retired phrases and each carries its new anchor', () => {
    const texts: Record<string, string> = {};
    for (const f of AC9_FILES) texts[f] = readFileSync(join(REPO_ROOT, f), 'utf-8');
    for (const f of AC9_FILES) {
      for (const phrase of AC9_FORBIDDEN) expect(texts[f], `${f} contains ${JSON.stringify(phrase)}`).not.toContain(phrase);
    }
    const sdkLine = texts['README.md'].split('\n').find((l) => l.includes('aim-sdk 2.0.3 against a self-hosted stack'));
    expect(sdkLine, 'README.md names the measured stack line').toBeDefined();
    expect(sdkLine as string).toMatch(/commit [0-9a-f]{7}/);
    expect(sdkLine as string).not.toMatch(/`edge`/);
    expect(texts['infrastructure/DEPLOYMENT.md']).toContain('1.0.0');
    expect(texts['infrastructure/DEPLOYMENT.md']).not.toContain('0.5.2');
    expect(texts['docs/demo/stack/images.env']).toContain('ghcr.io/opena2a-org/aim-server:latest');
    expect(texts['sdk/python/README.md']).toContain('1.1.0');
  });
});
