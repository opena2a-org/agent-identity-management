import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';
import { load } from 'js-yaml';

/**
 * Workflow-shape guard for ci.yml's example-java-manifest-audit job, the
 * blocking dependency scan over examples/ and sdk/java.
 *
 * The job only means something if it can go red, runs on every pull request,
 * and cannot be quietly weakened. These assertions pin that:
 *
 *   - the CI Gate needs the job and its verify step requires success outright;
 *   - no escape hatch: no if:, no continue-on-error, no `|| true`;
 *   - every action is pinned by a full commit sha, and Trivy is fetched from
 *     the upstream release and checked against a sha256 BEFORE extraction
 *     (no trivy-action / setup-trivy, which fetch by version);
 *   - Trivy reads only the config the job writes and the exception file
 *     under .github/, and reports findings with an exit code (3) distinct
 *     from a scanner crash (1).
 *
 * This file runs in the SDK test job, which the CI Gate also needs.
 */

interface WorkflowStep {
  name?: string;
  uses?: string;
  run?: string;
  if?: unknown;
  'continue-on-error'?: unknown;
}

interface WorkflowJob {
  name?: string;
  needs?: string[];
  if?: unknown;
  'continue-on-error'?: unknown;
  permissions?: Record<string, string>;
  env?: Record<string, unknown>;
  steps?: WorkflowStep[];
}

interface Workflow {
  jobs: Record<string, WorkflowJob>;
}

const JOB_ID = 'example-java-manifest-audit';

const workflow = load(
  readFileSync(
    join(__dirname, '..', '..', '..', '.github', 'workflows', 'ci.yml'),
    'utf-8',
  ),
) as Workflow;

function job(): WorkflowJob {
  const j = workflow.jobs[JOB_ID];
  expect(j, `ci.yml must define the ${JOB_ID} job`).toBeDefined();
  return j;
}

const allRun = (): string =>
  (job().steps ?? []).map((s) => s.run ?? '').join('\n');

describe(`ci.yml ${JOB_ID} is a blocking gate`, () => {
  it('the CI Gate needs the job and requires it to succeed outright', () => {
    const gate = workflow.jobs['ci-gate'];
    expect(gate.needs ?? []).toContain(JOB_ID);

    const verify = (gate.steps ?? []).find(
      (s) => s.name === 'Verify no required job failed',
    );
    const run = verify?.run ?? '';
    expect(run).toContain(`needs.${JOB_ID}.result`);
    const unconditionalLoop = run
      .split('\n')
      .find((l) => l.includes('for pair in') && l.includes('secrets-lint'));
    expect(unconditionalLoop).toContain(`"${JOB_ID}:$`);
  });

  it('has no if:, continue-on-error or || true on the job or any step', () => {
    const j = job();
    expect(j.if).toBeUndefined();
    expect(j['continue-on-error']).toBeUndefined();
    for (const s of j.steps ?? []) {
      const label = s.name ?? s.uses ?? 'unnamed';
      expect(s.if, `step "${label}" must be unconditional`).toBeUndefined();
      expect(
        s['continue-on-error'],
        `step "${label}" must not set continue-on-error`,
      ).toBeUndefined();
      expect(s.run ?? '', `step "${label}" must not contain || true`).not.toMatch(
        /\|\|\s*true/,
      );
    }
  });

  it('reads the repository only, with no write permission', () => {
    expect(job().permissions).toEqual({ contents: 'read' });
  });

  it('pins every action by a full commit sha and uses no Trivy action', () => {
    for (const s of job().steps ?? []) {
      if (!s.uses) continue;
      expect(s.uses, `${s.uses} must be pinned by a 40-hex sha`).toMatch(
        /@[0-9a-f]{40}$/,
      );
      expect(s.uses).not.toMatch(/aquasecurity\/(trivy-action|setup-trivy)/);
    }
  });

  it('fetches Trivy from the upstream release and verifies its sha256 before extracting it', () => {
    const env = job().env ?? {};
    expect(String(env.TRIVY_SHA256)).toMatch(/^[0-9a-f]{64}$/);
    expect(String(env.TRIVY_VERSION)).toMatch(/^\d+\.\d+\.\d+$/);

    const run = allRun();
    expect(run).toContain(
      'https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/',
    );
    const check = run.indexOf('sha256sum -c');
    const extract = run.indexOf('tar -xzf');
    expect(check, 'a sha256sum -c check must exist').toBeGreaterThanOrEqual(0);
    expect(extract, 'the tarball must be extracted after the check').toBeGreaterThan(
      check,
    );
  });

  it('isolates Trivy config, separates findings from scanner errors, and runs its controls', () => {
    const run = allRun();
    expect(run).toContain('--config "$RUNNER_TEMP/trivy-config.yaml"');
    expect(run).toContain(
      '--ignorefile "$GITHUB_WORKSPACE/.github/trivy-exceptions.txt"',
    );
    expect(run).toContain('--exit-code 3');
    expect(run).toContain('--offline-scan');
    // One positive control per manifest class, each checked by advisory ID.
    expect(run).toContain('"pom:pom.xml:CVE-');
    expect(run).toContain('"pip:requirements.txt:CVE-');
    expect(job().env?.SCAN_DIRS).toBe('examples sdk/java');
  });
});
