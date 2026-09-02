import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';
import { load } from 'js-yaml';

/**
 * Workflow-shape guard: the CI Gate must run this SDK's test suite.
 *
 * Until AIM-05, no job in .github/workflows/ci.yml ran the SDK's vitest
 * suite or its typecheck — release.yml ran them at publish time only, so 78
 * test files gated no pull request. These assertions parse ci.yml and pin
 * the repair in place:
 *
 *   AC1  a job runs the suite (npm ci → vitest with the zero-skip refusal
 *        release.yml uses at publish → npm run typecheck) in sdk/typescript;
 *   AC2  the ci-gate job `needs` that job AND its "Verify no required job
 *        failed" step checks the job's result — a needs entry alone reports
 *        nothing, the gate step is what fails closed;
 *   AC3  the job has no escape hatch: no continue-on-error, no `|| true`,
 *        no `if:` that could skip it on a pull request.
 *
 * This file runs inside the very job it describes (the suite is `vitest
 * run` under sdk/typescript), which the gate needs — so the guard itself
 * is load-bearing, not a test no required job executes.
 */

interface WorkflowStep {
  name?: string;
  run?: string;
  if?: unknown;
  env?: Record<string, unknown>;
  'working-directory'?: string;
  'continue-on-error'?: unknown;
}

interface WorkflowJob {
  name?: string;
  needs?: string[];
  if?: unknown;
  'continue-on-error'?: unknown;
  defaults?: { run?: { 'working-directory'?: string } };
  steps?: WorkflowStep[];
}

interface Workflow {
  jobs: Record<string, WorkflowJob>;
}

const workflowPath = join(
  __dirname,
  '..',
  '..',
  '..',
  '.github',
  'workflows',
  'ci.yml',
);
const workflow = load(readFileSync(workflowPath, 'utf-8')) as Workflow;

// The twelve entries the gate needed before the SDK test job existed. AC2
// requires the new job IN ADDITION to these — none may be traded away.
const BASELINE_GATE_NEEDS = [
  'secrets-lint',
  'tenant-scoping-lint',
  'workflow-lint',
  'sync-protect-guard',
  'docker-paths-filter',
  'sdk-dependency-audit',
  'changes',
  'backend',
  'integration',
  'atx-conformance',
  'frontend',
  'e2e-empty-state',
];

/**
 * Locate the SDK test job structurally rather than by a hard-coded id: the
 * job whose default working directory is sdk/typescript and whose steps run
 * the vitest suite. (sdk-dependency-audit also touches sdk/typescript but
 * runs `npm audit` only — it must not satisfy this.)
 */
function findSdkTestJob(): { id: string; job: WorkflowJob } {
  const entries = Object.entries(workflow.jobs ?? {}).filter(
    ([, job]) =>
      job?.defaults?.run?.['working-directory'] === 'sdk/typescript' &&
      (job.steps ?? []).some(
        (s) => typeof s.run === 'string' && s.run.includes('vitest run'),
      ),
  );
  expect(
    entries.length,
    'exactly one ci.yml job must run the SDK vitest suite under working-directory sdk/typescript',
  ).toBe(1);
  const [id, job] = entries[0];
  return { id, job };
}

describe('ci.yml runs the SDK suite in a job the CI Gate needs', () => {
  it('AIM-05.AC1 a job runs npm ci, then vitest with the zero-skip refusal under CI=true, then npm run typecheck, in sdk/typescript', () => {
    const { job } = findSdkTestJob();
    const steps = job.steps ?? [];
    const runOf = (s: WorkflowStep) => s.run ?? '';

    const ciIdx = steps.findIndex((s) => runOf(s).trim().startsWith('npm ci'));
    const testIdx = steps.findIndex((s) => runOf(s).includes('vitest run'));
    const typecheckIdx = steps.findIndex((s) =>
      runOf(s).includes('npm run typecheck'),
    );

    expect(ciIdx, 'an `npm ci` step must exist').toBeGreaterThanOrEqual(0);
    expect(testIdx, 'a vitest step must exist').toBeGreaterThan(ciIdx);
    expect(
      typecheckIdx,
      'a typecheck step must follow the vitest step',
    ).toBeGreaterThan(testIdx);

    // The suite must run under CI so environment-gated cases execute
    // rather than silently skip (release.yml's publish-time rationale).
    const testStep = steps[testIdx];
    expect(testStep.env?.CI, 'the vitest step must set env CI: "true"').toBe(
      'true',
    );

    // The zero-skip refusal shape release.yml:219-235 uses at publish time:
    // a JSON reporter, then fail unless numTotalTests > 0 and
    // numFailedTests == 0 and numPendingTests + numTodoTests == 0.
    const testRun = runOf(testStep);
    expect(testRun).toContain('--reporter=json');
    expect(testRun).toContain('numTotalTests');
    expect(testRun).toContain('numFailedTests');
    expect(testRun).toContain('numPendingTests');
    expect(testRun).toContain('numTodoTests');
    expect(testRun).toMatch(/!\(total > 0\) \|\| failed > 0 \|\| skipped > 0/);
    expect(testRun).toContain('process.exit(1)');
  });

  it('AIM-05.AC2 the ci-gate needs the SDK test job on top of the twelve baseline entries, and its verify step checks the job result as an unconditional requirement', () => {
    const { id } = findSdkTestJob();
    const gate = workflow.jobs['ci-gate'];
    expect(gate, 'the ci-gate job must exist').toBeDefined();

    const needs = gate.needs ?? [];
    for (const baseline of BASELINE_GATE_NEEDS) {
      expect(needs, `gate must still need ${baseline}`).toContain(baseline);
    }
    expect(needs, `gate must need ${id}`).toContain(id);

    const verify = (gate.steps ?? []).find(
      (s) => s.name === 'Verify no required job failed',
    );
    expect(verify, 'the "Verify no required job failed" step must exist').toBeDefined();
    const verifyRun = verify?.run ?? '';

    // A needs entry without the result variable reports nothing — the shell
    // check is what fails the gate when the suite fails.
    expect(verifyRun).toContain(`needs.${id}.result`);

    // And the result must be checked in the same unconditional-jobs loop as
    // secrets-lint (success required outright — a skip must not pass).
    const unconditionalLoop = verifyRun
      .split('\n')
      .find((line) => line.includes('for pair in') && line.includes('secrets-lint'));
    expect(
      unconditionalLoop,
      'the unconditional-jobs loop must exist in the verify step',
    ).toBeDefined();
    expect(
      unconditionalLoop,
      `the unconditional-jobs loop must check ${id}`,
    ).toContain(`"${id}:$`);
  });

  it('AIM-05.AC3 the SDK test job has no continue-on-error at the job level or on any step', () => {
    const { job } = findSdkTestJob();
    expect(job['continue-on-error']).toBeUndefined();
    for (const step of job.steps ?? []) {
      expect(
        step['continue-on-error'],
        `step "${step.name ?? step.run}" must not set continue-on-error`,
      ).toBeUndefined();
    }
  });

  it('AIM-05.AC3 no step of the SDK test job masks its exit code with || true', () => {
    const { job } = findSdkTestJob();
    for (const step of job.steps ?? []) {
      expect(
        step.run ?? '',
        `step "${step.name ?? 'unnamed'}" must not contain || true`,
      ).not.toMatch(/\|\|\s*true/);
    }
  });

  it('AIM-05.AC3 the SDK test job has no if: condition on the job or its steps, so it cannot skip on pull requests', () => {
    const { job } = findSdkTestJob();
    expect(job.if, 'the job must be unconditional').toBeUndefined();
    for (const step of job.steps ?? []) {
      expect(
        step.if,
        `step "${step.name ?? step.run}" must be unconditional`,
      ).toBeUndefined();
    }
  });
});
