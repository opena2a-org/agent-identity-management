import { afterAll, describe, expect, it } from 'vitest';
import {
  mkdirSync,
  mkdtempSync,
  readdirSync,
  readFileSync,
  rmSync,
  symlinkSync,
  writeFileSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, relative, sep } from 'node:path';
import { load } from 'js-yaml';

/**
 * No workflow in this repository pushes to the hosted service's repository.
 *
 * The push-sync workflow `.github/workflows/sync-to-cloud.yml` is retired:
 * the sync to the hosted service runs from the hosted side, outside this
 * repository. This check walks every file under `.github/` and refuses
 *
 *   - the retired workflow path itself;
 *   - the retired token name `AIM_CLOUD_SYNC_TOKEN`, in the raw text of any
 *     file (a comment included) and in any string scalar of a parsed YAML
 *     document (which is what catches a YAML-escaped spelling);
 *   - any parsed string scalar naming `opena2a-org/aim-cloud`, whether it is a
 *     checkout `repository`, a `run` command, a `uses` reference or an `env`
 *     value.
 *
 * Matching is by lowercased containment, so a suffixed or re-cased variant is
 * refused too. A YAML file that does not parse is an error, not a clean file,
 * and a walk that visits no regular file is an error, not a clean tree.
 */

type FindingKind =
  | 'token-name'
  | 'workflow-path'
  | 'cloud-repository'
  | 'non-regular-entry';

interface Finding {
  file: string;
  kind: FindingKind;
  keyPath?: string;
}

interface CheckResult {
  findings: Finding[];
  visited: string[];
  parsed: string[];
}

const TOKEN_NAME = 'aim_cloud_sync_token';
const CLOUD_REPOSITORY = 'opena2a-org/aim-cloud';
const RETIRED_WORKFLOW_PATH = '.github/workflows/sync-to-cloud.yml';

function toPosix(path: string): string {
  return path.split(sep).join('/');
}

interface Scalar {
  keyPath: string;
  value: string;
}

function collectStringScalars(node: unknown, keyPath: string, out: Scalar[]): void {
  if (typeof node === 'string') {
    out.push({ keyPath, value: node });
    return;
  }
  if (Array.isArray(node)) {
    node.forEach((item, index) =>
      collectStringScalars(item, `${keyPath}[${index}]`, out),
    );
    return;
  }
  if (node !== null && typeof node === 'object') {
    for (const [key, value] of Object.entries(node as Record<string, unknown>)) {
      collectStringScalars(value, keyPath === '' ? key : `${keyPath}.${key}`, out);
    }
  }
}

function checkNoCloudPushWorkflow(root: string): CheckResult {
  const findings: Finding[] = [];
  const visited: string[] = [];
  const parsed: string[] = [];

  const walk = (dir: string): void => {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const absolute = join(dir, entry.name);
      const file = toPosix(relative(root, absolute));

      if (entry.isDirectory()) {
        walk(absolute);
        continue;
      }
      if (!entry.isFile()) {
        findings.push({ file, kind: 'non-regular-entry' });
        continue;
      }

      visited.push(file);
      const text = readFileSync(absolute, 'utf-8');

      if (text.toLowerCase().includes(TOKEN_NAME)) {
        findings.push({ file, kind: 'token-name' });
      }
      if (file === RETIRED_WORKFLOW_PATH) {
        findings.push({ file, kind: 'workflow-path' });
      }

      if (entry.name.endsWith('.yml') || entry.name.endsWith('.yaml')) {
        const document = load(text);
        parsed.push(file);
        const scalars: Scalar[] = [];
        collectStringScalars(document, '', scalars);
        for (const { keyPath, value } of scalars) {
          const lowered = value.toLowerCase();
          if (lowered.includes(TOKEN_NAME)) {
            findings.push({ file, kind: 'token-name', keyPath });
          }
          if (lowered.includes(CLOUD_REPOSITORY)) {
            findings.push({ file, kind: 'cloud-repository', keyPath });
          }
        }
      }
    }
  };

  walk(join(root, '.github'));

  if (visited.length === 0) {
    throw new Error(`no regular file visited under ${join(root, '.github')}`);
  }
  return { findings, visited, parsed };
}

// ---------------------------------------------------------------------------
// Planted trees: each cell writes its own tree into a fresh temporary
// directory and runs the check over it.

const plantedRoots: string[] = [];

afterAll(() => {
  for (const root of plantedRoots) {
    rmSync(root, { recursive: true, force: true });
  }
});

function plantTree(files: Record<string, string>): string {
  const root = mkdtempSync(join(tmpdir(), 'no-cloud-push-workflow-'));
  plantedRoots.push(root);
  mkdirSync(join(root, '.github'), { recursive: true });
  for (const [file, text] of Object.entries(files)) {
    const absolute = join(root, file);
    mkdirSync(dirname(absolute), { recursive: true });
    writeFileSync(absolute, text);
  }
  return root;
}

function workflowWithCheckout(withBlock: string): string {
  return [
    'name: planted',
    'on: workflow_dispatch',
    'jobs:',
    '  sync:',
    '    runs-on: ubuntu-latest',
    '    steps:',
    '      - uses: actions/checkout@v5',
    '        with:',
    withBlock,
    '',
  ].join('\n');
}

const CLEAN_CI = [
  'name: CI',
  'on: [push, pull_request]',
  'jobs:',
  '  build:',
  '    runs-on: ubuntu-latest',
  '    steps:',
  '      - uses: actions/checkout@v4',
  '      - run: npm test',
  '',
].join('\n');

function kindsFor(result: CheckResult, file: string): FindingKind[] {
  return result.findings.filter((f) => f.file === file).map((f) => f.kind);
}

// ---------------------------------------------------------------------------

const repositoryRoot = join(__dirname, '..', '..', '..');

describe('no-cloud-push-workflow: the repository tree', () => {
  it('QGF-271.AC1 has no retired push-sync workflow, no token name and no hosted repository name under .github/, after a real walk that parsed ci.yml', () => {
    const result = checkNoCloudPushWorkflow(repositoryRoot);
    expect(result.findings).toEqual([]);
    expect(result.visited.length).toBeGreaterThan(0);
    expect(result.parsed.length).toBeGreaterThan(0);
    expect(result.parsed).toContain('.github/workflows/ci.yml');
  });
});

describe('no-cloud-push-workflow: the check function', () => {
  it('QGF-271.AC2 returns findings, visited and parsed, and a scalar finding carries the key path to the scalar', () => {
    const root = plantTree({
      '.github/workflows/sync.yml': workflowWithCheckout(
        '          token: ${{ secrets.AIM_CLOUD_SYNC_TOKEN }}',
      ),
    });
    const result = checkNoCloudPushWorkflow(root);
    expect(Object.keys(result).sort()).toEqual(['findings', 'parsed', 'visited']);
    expect(result.visited).toEqual(['.github/workflows/sync.yml']);
    expect(result.parsed).toEqual(['.github/workflows/sync.yml']);
    expect(result.findings).toContainEqual({
      file: '.github/workflows/sync.yml',
      kind: 'token-name',
      keyPath: 'jobs.sync.steps[0].with.token',
    });
  });

  it('QGF-271.AC2 refuses the hosted repository name in a uses reference and in an env value with the one scalar rule', () => {
    const root = plantTree({
      '.github/workflows/uses.yml': [
        'name: planted',
        'on: workflow_dispatch',
        'jobs:',
        '  sync:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        '      - uses: opena2a-org/aim-cloud/.github/actions/sync@main',
        '',
      ].join('\n'),
      '.github/workflows/env.yml': [
        'name: planted',
        'on: workflow_dispatch',
        'env:',
        '  TARGET_REPO: opena2a-org/aim-cloud',
        'jobs:',
        '  sync:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        '      - run: echo "$TARGET_REPO"',
        '',
      ].join('\n'),
    });
    const result = checkNoCloudPushWorkflow(root);
    expect(result.findings).toContainEqual({
      file: '.github/workflows/uses.yml',
      kind: 'cloud-repository',
      keyPath: 'jobs.sync.steps[0].uses',
    });
    expect(result.findings).toContainEqual({
      file: '.github/workflows/env.yml',
      kind: 'cloud-repository',
      keyPath: 'env.TARGET_REPO',
    });
  });
});

describe('no-cloud-push-workflow: planted forbidden shapes', () => {
  it('QGF-271.AC3 (a) refuses the token name in a step env and in a checkout with.token', () => {
    const root = plantTree({
      '.github/workflows/env-token.yml': [
        'name: planted',
        'on: workflow_dispatch',
        'jobs:',
        '  sync:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        '      - run: gh pr list',
        '        env:',
        '          GH_TOKEN: ${{ secrets.AIM_CLOUD_SYNC_TOKEN }}',
        '',
      ].join('\n'),
      '.github/workflows/with-token.yml': workflowWithCheckout(
        '          token: ${{ secrets.AIM_CLOUD_SYNC_TOKEN }}',
      ),
    });
    const result = checkNoCloudPushWorkflow(root);
    expect(kindsFor(result, '.github/workflows/env-token.yml')).toContain('token-name');
    expect(kindsFor(result, '.github/workflows/with-token.yml')).toContain('token-name');
  });

  it('QGF-271.AC3 (b) refuses the hosted repository name under checkout with.repository', () => {
    const root = plantTree({
      '.github/workflows/repo.yml': workflowWithCheckout(
        '          repository: opena2a-org/aim-cloud',
      ),
    });
    const result = checkNoCloudPushWorkflow(root);
    expect(kindsFor(result, '.github/workflows/repo.yml')).toContain('cloud-repository');
  });

  it('QGF-271.AC3 (c) refuses the hosted repository name in a run string', () => {
    const root = plantTree({
      '.github/workflows/run.yml': [
        'name: planted',
        'on: workflow_dispatch',
        'jobs:',
        '  sync:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        '      - run: gh pr list --repo opena2a-org/aim-cloud --state open',
        '',
      ].join('\n'),
    });
    const result = checkNoCloudPushWorkflow(root);
    expect(kindsFor(result, '.github/workflows/run.yml')).toContain('cloud-repository');
  });

  it('QGF-271.AC3 (d) refuses a suffixed token name by containment, never equality', () => {
    const root = plantTree({
      '.github/workflows/v2.yml': workflowWithCheckout(
        '          token: ${{ secrets.AIM_CLOUD_SYNC_TOKEN_V2 }}',
      ),
    });
    const result = checkNoCloudPushWorkflow(root);
    expect(kindsFor(result, '.github/workflows/v2.yml')).toContain('token-name');
  });

  it('QGF-271.AC3 (e) refuses the token name in a comment line, because raw text is read before parsing', () => {
    const root = plantTree({
      '.github/workflows/comment.yml': `# needs AIM_CLOUD_SYNC_TOKEN\n${CLEAN_CI}`,
    });
    const result = checkNoCloudPushWorkflow(root);
    expect(kindsFor(result, '.github/workflows/comment.yml')).toContain('token-name');
  });

  it('QGF-271.AC3 (f) refuses the token name in a non-YAML file', () => {
    const root = plantTree({
      '.github/CODEOWNERS': '# AIM_CLOUD_SYNC_TOKEN\n/.github/ @owner\n',
    });
    const result = checkNoCloudPushWorkflow(root);
    expect(kindsFor(result, '.github/CODEOWNERS')).toContain('token-name');
  });

  it('QGF-271.AC3 (g) refuses the retired workflow path even when its text carries neither string', () => {
    const root = plantTree({
      '.github/workflows/sync-to-cloud.yml': CLEAN_CI,
    });
    const result = checkNoCloudPushWorkflow(root);
    expect(kindsFor(result, '.github/workflows/sync-to-cloud.yml')).toContain('workflow-path');
  });

  it('QGF-271.AC3 (h) reports a symbolic link under .github/ as a non-regular entry without following it', () => {
    const root = plantTree({
      '.github/workflows/ci.yml': CLEAN_CI,
      'outside.yml': workflowWithCheckout(
        '          repository: opena2a-org/aim-cloud',
      ),
    });
    symlinkSync(join(root, 'outside.yml'), join(root, '.github', 'workflows', 'sync2.yml'));
    const result = checkNoCloudPushWorkflow(root);
    expect(kindsFor(result, '.github/workflows/sync2.yml')).toContain('non-regular-entry');
    expect(result.visited).not.toContain('.github/workflows/sync2.yml');
    expect(result.parsed).not.toContain('.github/workflows/sync2.yml');
  });
});

describe('no-cloud-push-workflow: shapes a raw-text scan admits', () => {
  it('QGF-271.AC4 (a) refuses YAML-escaped spellings of the repository name and the token name', () => {
    const text = workflowWithCheckout(
      [
        '          repository: "opena2a-org/aim-\\x63loud"',
        '          token: "${{ secrets.AIM_CLOUD_SYNC_\\x54OKEN }}"',
      ].join('\n'),
    );
    expect(text).not.toContain('aim-cloud');
    expect(text).not.toContain('AIM_CLOUD_SYNC_TOKEN');
    const root = plantTree({ '.github/workflows/escaped.yml': text });
    const result = checkNoCloudPushWorkflow(root);
    const kinds = kindsFor(result, '.github/workflows/escaped.yml');
    expect(kinds).toContain('cloud-repository');
    expect(kinds).toContain('token-name');
  });

  it('QGF-271.AC4 (b) refuses re-cased spellings of the repository name and the token name', () => {
    const root = plantTree({
      '.github/workflows/cased.yml': workflowWithCheckout(
        [
          '          repository: OpenA2A-Org/AIM-Cloud',
          '          token: ${{ secrets.aim_cloud_sync_token }}',
        ].join('\n'),
      ),
    });
    const result = checkNoCloudPushWorkflow(root);
    const kinds = kindsFor(result, '.github/workflows/cased.yml');
    expect(kinds).toContain('cloud-repository');
    expect(kinds).toContain('token-name');
  });
});

describe('no-cloud-push-workflow: parse failure, empty walk and the admitted comment', () => {
  it('QGF-271.AC5 (a) throws on a YAML file that does not parse', () => {
    const root = plantTree({ '.github/workflows/broken.yml': 'jobs: [' });
    expect(() => checkNoCloudPushWorkflow(root)).toThrow();
  });

  it('QGF-271.AC5 (b) throws when .github/ holds no regular file', () => {
    const root = plantTree({});
    mkdirSync(join(root, '.github', 'workflows'), { recursive: true });
    expect(() => checkNoCloudPushWorkflow(root)).toThrow();
  });

  it('QGF-271.AC5 (c) admits a comment naming aim-cloud alone', () => {
    const root = plantTree({
      '.github/workflows/ci.yml': [
        'name: CI',
        'on: [push, pull_request]',
        'jobs:',
        "  # guarded is the script's behaviour on aim-cloud's tree, and a regression",
        '  build:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        '      - uses: actions/checkout@v4',
        '      - run: npm test',
        '',
      ].join('\n'),
    });
    const result = checkNoCloudPushWorkflow(root);
    expect(result.findings).toEqual([]);
    expect(result.visited).toHaveLength(1);
    expect(result.parsed).toHaveLength(1);
  });
});
