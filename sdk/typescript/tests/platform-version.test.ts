import { describe, it, expect } from 'vitest';
import { spawnSync } from 'child_process';
import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * The root package.json, apps/web/package.json and the newest release section
 * of CHANGELOG.md name one platform version, and the tests of the check that
 * says so pass.
 *
 * The root package.json holds the platform's version. A platform release is
 * tagged platform-v<version> from a commit whose two manifests carry that
 * version and whose CHANGELOG.md has opened the version's section.
 * scripts/check-platform-version.mjs refuses a tree where the three disagree,
 * and, given a tag, a tag that names another version. This suite runs in CI on
 * every pull request to main and every push to it, so a commit that opens a
 * release section without setting both manifests to its version fails here.
 *
 * The check and its tests need only node.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');

const CHECK = 'scripts/check-platform-version.mjs';
const CHECK_TESTS = 'scripts/test-check-platform-version.mjs';

function node(args: string[]): { status: number | null; output: string } {
  const result = spawnSync(process.execPath, args, { cwd: REPO_ROOT, encoding: 'utf-8' });
  if (result.error) throw result.error;
  return { status: result.status, output: `${result.stdout}${result.stderr}` };
}

describe('platform version', () => {
  it('scripts/check-platform-version.mjs finds the two manifests and the newest CHANGELOG.md release section at one version', () => {
    const { status, output } = node([CHECK]);
    expect(status, output).toBe(0);
    expect(output).toMatch(
      /^check-platform-version: ok: package\.json, apps\/web\/package\.json and CHANGELOG\.md's newest release section read \S+$/m,
    );
  });

  it('scripts/test-check-platform-version.mjs passes', () => {
    const { status, output } = node(['--test', CHECK_TESTS]);
    expect(status, output).toBe(0);
    // Node 20 prints TAP (`# fail 0`) when stdout is not a terminal, later
    // releases print `ℹ fail 0`.
    expect(output).toMatch(/^(?:#|ℹ) pass [1-9]\d*$/m);
    expect(output).toMatch(/^(?:#|ℹ) fail 0$/m);
  }, 120_000);

  it('CONTRIBUTING.md names the check among the scripts the TypeScript SDK suite runs, and runs it before a platform tag', () => {
    const guide = readFileSync(join(REPO_ROOT, 'CONTRIBUTING.md'), 'utf-8');
    const start = guide.indexOf('**TypeScript SDK**');
    expect(start, 'CONTRIBUTING.md has no TypeScript SDK passage under Testing Requirements').toBeGreaterThan(-1);
    const end = guide.indexOf('\n### ', start);
    const passage = guide.slice(start, end === -1 ? undefined : end).replace(/\s+/g, ' ');

    expect(passage).toContain('`tests/platform-version.test.ts`');
    expect(passage).toContain(`\`${CHECK}\` and its tests, \`${CHECK_TESTS}\`, with \`node\``);

    const tagging = guide.indexOf('**Cutting a release:**');
    expect(tagging, 'CONTRIBUTING.md has no "Cutting a release" steps').toBeGreaterThan(-1);
    const next = guide.indexOf('\n## ', tagging);
    const steps = guide.slice(tagging, next === -1 ? undefined : next);
    const check = steps.indexOf(`node ${CHECK} platform-v`);
    expect(check, 'the platform release steps do not run the check').toBeGreaterThan(-1);
    expect(check).toBeLessThan(steps.indexOf('git tag -a platform-v'));
  });
});
