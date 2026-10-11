import { describe, it, expect } from 'vitest';
import { spawnSync } from 'child_process';
import { join } from 'path';

/**
 * The repository's documentation lints pass on the repository, and the
 * tests of the two shell lints pass.
 *
 * Each of these scripts failed on the repository while nothing ran it:
 * scripts/check-public-wording.py found seven barred terms in six pages,
 * scripts/test-lint-docs-keyvault-master-key.sh failed on a guide that set
 * ENVIRONMENT=test in a section that never named KEYVAULT_MASTER_KEY, and
 * scripts/lint-docs-repo-links.sh read only Markdown while a disabled Go
 * test, the Go SDK README the server generated and three Python files named
 * the opena2a GitHub account. Running them here keeps each one passing; a
 * failure prints the script's own report.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');

function run(command: string, args: string[]): { status: number | null; output: string } {
  const result = spawnSync(command, args, { cwd: REPO_ROOT, encoding: 'utf-8' });
  if (result.error) throw result.error;
  return { status: result.status, output: `${result.stdout}${result.stderr}` };
}

describe('repository documentation lints', () => {
  it('scripts/check-public-wording.py finds no barred wording in the public documents', () => {
    const { status, output } = run('python3', ['scripts/check-public-wording.py']);
    expect(status, output).toBe(0);
    expect(output).toMatch(/^0 hit\(s\) in 0 of [1-9]\d* file\(s\) scanned/m);
  });

  it('scripts/test-lint-docs-keyvault-master-key.sh passes, the repository\'s docs included', () => {
    const { status, output } = run('bash', ['scripts/test-lint-docs-keyvault-master-key.sh']);
    expect(status, output).toBe(0);
    expect(output).toContain('test-lint-docs-keyvault-master-key: all checks passed');
  }, 120_000);

  it('scripts/test-lint-docs-repo-links.sh passes, the repository\'s files included', () => {
    const { status, output } = run('bash', ['scripts/test-lint-docs-repo-links.sh']);
    expect(status, output).toBe(0);
    expect(output).toContain('test-lint-docs-repo-links: all checks passed');
  }, 120_000);
});
