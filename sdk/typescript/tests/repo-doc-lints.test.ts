import { describe, it, expect } from 'vitest';
import { spawnSync } from 'child_process';
import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * The repository's documentation lints pass on the repository, and the
 * tests of the two shell lints and of the docs prose check pass.
 *
 * Each of these scripts failed on the repository while nothing ran it:
 * scripts/check-public-wording.py found seven barred terms in six pages,
 * scripts/test-lint-docs-keyvault-master-key.sh failed on a guide that set
 * ENVIRONMENT=test in a section that never named KEYVAULT_MASTER_KEY, and
 * scripts/lint-docs-repo-links.sh read only Markdown while a disabled Go
 * test, the Go SDK README the server generated and three Python files named
 * the opena2a GitHub account. Running them here keeps each one passing; a
 * failure prints the script's own report.
 *
 * scripts/check-docs-prose.mjs runs here for the same reason: the root lint
 * script that also runs it is not a CI step, so a dash or a "Why this
 * exists" section added to a tracked Markdown file would otherwise reach
 * main unreported.
 *
 * The cases need git, bash and python3, and a git checkout: the link lint
 * asks git which files to read and stops in a tree without .git. The
 * contributing guide says so, and the last case keeps it saying so.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');

interface Lint {
  /** The program that runs the script, as CONTRIBUTING.md names it. */
  runner: string;
  /** The executable spawned: the runner, or this Node binary for node. */
  command: string;
  script: string;
}

const WORDING: Lint = { runner: 'python3', command: 'python3', script: 'scripts/check-public-wording.py' };
const KEYVAULT: Lint = { runner: 'bash', command: 'bash', script: 'scripts/test-lint-docs-keyvault-master-key.sh' };
const REPO_LINKS: Lint = { runner: 'bash', command: 'bash', script: 'scripts/test-lint-docs-repo-links.sh' };
const DOCS_PROSE: Lint = { runner: 'node', command: process.execPath, script: 'scripts/check-docs-prose.mjs' };
const DOCS_PROSE_TESTS: Lint = { runner: 'node', command: process.execPath, script: 'scripts/test-check-docs-prose.mjs' };

/** Every script the cases below run. */
const LINTS = [WORDING, KEYVAULT, REPO_LINKS, DOCS_PROSE, DOCS_PROSE_TESTS];
const COUNT_WORDS = ['no', 'one', 'two', 'three', 'four', 'five', 'six', 'seven', 'eight', 'nine'];

function run(lint: Lint, args: string[] = [lint.script]): { status: number | null; output: string } {
  const result = spawnSync(lint.command, args, { cwd: REPO_ROOT, encoding: 'utf-8' });
  if (result.error) throw result.error;
  return { status: result.status, output: `${result.stdout}${result.stderr}` };
}

describe('repository documentation lints', () => {
  it('scripts/check-public-wording.py finds no barred wording in the public documents', () => {
    const { status, output } = run(WORDING);
    expect(status, output).toBe(0);
    expect(output).toMatch(/^0 hit\(s\) in 0 of [1-9]\d* file\(s\) scanned/m);
  });

  it('scripts/test-lint-docs-keyvault-master-key.sh passes, the repository\'s docs included', () => {
    const { status, output } = run(KEYVAULT);
    expect(status, output).toBe(0);
    expect(output).toContain('test-lint-docs-keyvault-master-key: all checks passed');
  }, 120_000);

  it('scripts/test-lint-docs-repo-links.sh passes, the repository\'s files included', () => {
    const { status, output } = run(REPO_LINKS);
    expect(status, output).toBe(0);
    expect(output).toContain('test-lint-docs-repo-links: all checks passed');
  }, 120_000);

  it('scripts/check-docs-prose.mjs finds no dash or "Why this" section in the tracked Markdown', () => {
    const { status, output } = run(DOCS_PROSE);
    expect(status, output).toBe(0);
    expect(output).toMatch(/^check-docs-prose: [1-9]\d* file\(s\) clean$/m);
  });

  it('scripts/test-check-docs-prose.mjs passes', () => {
    const { status, output } = run(DOCS_PROSE_TESTS, ['--test', DOCS_PROSE_TESTS.script]);
    expect(status, output).toBe(0);
    // Node 20 prints TAP (`# fail 0`) when stdout is not a terminal, later
    // releases print `ℹ fail 0`.
    expect(output).toMatch(/^(?:#|ℹ) pass [1-9]\d*$/m);
    expect(output).toMatch(/^(?:#|ℹ) fail 0$/m);
  }, 120_000);

  it('CONTRIBUTING.md names every one of these scripts and says they need git, bash, python3 and a git checkout', () => {
    const guide = readFileSync(join(REPO_ROOT, 'CONTRIBUTING.md'), 'utf-8');
    const start = guide.indexOf('**TypeScript SDK**');
    expect(start, 'CONTRIBUTING.md has no TypeScript SDK passage under Testing Requirements').toBeGreaterThan(-1);
    const end = guide.indexOf('\n### ', start);
    const passage = guide.slice(start, end === -1 ? undefined : end).replace(/\s+/g, ' ');

    expect(passage).toContain('`tests/repo-doc-lints.test.ts`');
    expect(passage).toContain(`also runs ${COUNT_WORDS[LINTS.length]} of the repository's documentation lint scripts`);
    for (const lint of LINTS) {
      expect(passage).toContain(`\`${lint.script}\``);
      expect(passage).toContain(`\`${lint.runner}\``);
    }
    expect(passage).toContain('The suite needs `git`, `bash` and `python3` on the `PATH`, and a git checkout of the repository');

    // The guide quotes the link lint's own words for a tree that is not a checkout.
    const stop = 'is not the top of a git work tree';
    expect(readFileSync(join(REPO_ROOT, 'scripts', 'lint-docs-repo-links.sh'), 'utf-8')).toContain(stop);
    expect(passage).toContain(`"${stop}"`);
  });
});
