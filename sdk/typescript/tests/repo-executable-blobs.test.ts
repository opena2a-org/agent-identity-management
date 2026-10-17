import { describe, it, expect } from 'vitest';
import { spawnSync } from 'child_process';
import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * No tracked file of the repository is a compiled executable, and the tests of
 * the lint that checks it pass.
 *
 * scripts/lint-no-executable-blobs.py refuses a tracked file that starts with
 * a compiled executable's magic bytes (ELF, Mach-O, PE, 0xCAFEBABE), and
 * scripts/test-lint-no-executable-blobs.sh tests it, but no workflow, package
 * script or test ran either of them, so a pull request that added a compiled
 * executable was not checked. This suite runs in CI on every pull request.
 *
 * The lint reads the tree of HEAD, so it checks what is committed, and it
 * needs git, python3 and a git checkout; its tests need bash and git.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');

interface Lint {
  command: string;
  script: string;
}

const BLOB_LINT: Lint = { command: 'python3', script: 'scripts/lint-no-executable-blobs.py' };
const BLOB_LINT_TESTS: Lint = { command: 'bash', script: 'scripts/test-lint-no-executable-blobs.sh' };

/**
 * The environment without the variables git reads to find a repository, as
 * `git rev-parse --local-env-vars` names them. A hook exports GIT_DIR, and the
 * lint's tests run git init in temporary directories, which with GIT_DIR
 * inherited would write to the repository the hook runs for. Without them,
 * each git the scripts run finds its repository from its working directory.
 */
function gitEnv(): NodeJS.ProcessEnv {
  const res = spawnSync('git', ['rev-parse', '--local-env-vars'], { encoding: 'utf-8' });
  const local = new Set((res.stdout || '').split('\n').filter(Boolean));
  return Object.fromEntries(Object.entries(process.env).filter(([name]) => !local.has(name)));
}

function run(lint: Lint): { status: number | null; output: string } {
  const result = spawnSync(lint.command, [lint.script], { cwd: REPO_ROOT, env: gitEnv(), encoding: 'utf-8' });
  if (result.error) throw result.error;
  return { status: result.status, output: `${result.stdout}${result.stderr}` };
}

describe('no compiled executable is tracked', () => {
  it('scripts/lint-no-executable-blobs.py finds no compiled executable in the tree of HEAD', () => {
    const { status, output } = run(BLOB_LINT);
    expect(status, output).toBe(0);
    expect(output).toMatch(
      /^lint-no-executable-blobs: [1-9]\d* tracked files at HEAD \(tree [0-9a-f]{12}\), none is a compiled executable$/m,
    );
  });

  it('scripts/test-lint-no-executable-blobs.sh passes', () => {
    const { status, output } = run(BLOB_LINT_TESTS);
    expect(status, output).toBe(0);
    expect(output).toContain('test-lint-no-executable-blobs: all checks passed');
  }, 120_000);

  it('CONTRIBUTING.md names the lint and its tests among the scripts the TypeScript SDK suite runs', () => {
    const guide = readFileSync(join(REPO_ROOT, 'CONTRIBUTING.md'), 'utf-8');
    const start = guide.indexOf('**TypeScript SDK**');
    expect(start, 'CONTRIBUTING.md has no TypeScript SDK passage under Testing Requirements').toBeGreaterThan(-1);
    const end = guide.indexOf('\n### ', start);
    const passage = guide.slice(start, end === -1 ? undefined : end).replace(/\s+/g, ' ');

    expect(passage).toContain('`tests/repo-executable-blobs.test.ts`');
    for (const lint of [BLOB_LINT, BLOB_LINT_TESTS]) {
      expect(passage).toContain(`\`${lint.script}\` with \`${lint.command}\``);
    }
  });
});
