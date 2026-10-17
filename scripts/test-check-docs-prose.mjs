// node:test cells for scripts/check-docs-prose.mjs. Run from the repository
// root:
//
//   node --test scripts/test-check-docs-prose.mjs
//
// Every document a cell needs is a string in this file, and every tree the
// command-line cells scan is a fresh git repository in a temporary directory.
// The dashes are written as escapes so this file reads the same in any editor.

import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { after, test } from 'node:test';
import { fileURLToPath } from 'node:url';

import { gitEnv, inScope, run, scanMarkdown } from './check-docs-prose.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const SCRIPT = path.join(HERE, 'check-docs-prose.mjs');
const REPO = path.resolve(HERE, '..');

const EM = '\u2014';
const EN = '\u2013';

// Every temporary directory a cell makes, removed once the cells have run.
const TEMP_DIRS = [];
after(() => {
  for (const dir of TEMP_DIRS) fs.rmSync(dir, { recursive: true, force: true });
});

function tempDir(prefix) {
  const dir = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), prefix)));
  TEMP_DIRS.push(dir);
  return dir;
}

function rules(text) {
  return scanMarkdown(text).map((f) => `${f.line}:${f.rule}`);
}

// A git repository whose index holds `files`. Every child runs without git's
// repository-local variables, so it uses the repository in `dir` even when
// the cells run inside a git hook.
function repoWith(files) {
  const dir = tempDir('docs-prose-');
  spawnSync('git', ['init', '-q'], { cwd: dir, env: gitEnv() });
  for (const [rel, text] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
    fs.writeFileSync(path.join(dir, rel), text);
  }
  spawnSync('git', ['add', '--', ...Object.keys(files)], { cwd: dir, env: gitEnv() });
  return dir;
}

function cli(root) {
  return spawnSync(process.execPath, [SCRIPT, '--root', root], {
    encoding: 'utf8',
    env: gitEnv(),
  });
}

test('the tracked documents of this repository are clean', () => {
  const result = run(REPO);
  assert.ok(result.files > 0, 'no Markdown file was scanned');
  assert.deepEqual(result.findings, []);
});

test('each dash rule is reported by line in prose', () => {
  const doc = [
    '# Title',
    '',
    'One clause -- and another.',
    `One clause ${EM} and another.`,
    `One clause${EM}and another.`,
    `One clause ${EN} and another.`,
    'A line that ends in a dash --',
    'plain line',
  ].join('\n');
  assert.deepEqual(rules(doc), [
    '3:spaced-double-hyphen',
    '4:em-dash',
    '5:em-dash',
    '6:spaced-en-dash',
    '7:spaced-double-hyphen',
  ]);
});

test('a "Why this exists" or "Why this matters" heading is reported', () => {
  const doc = [
    '## Why this exists',
    '### Why This Matters',
    '**Why this exists:**',
    '## Why this happens',
    'Why this exists is a phrase in a sentence.',
  ].join('\n');
  assert.deepEqual(rules(doc), ['1:why-heading', '2:why-heading', '3:why-heading']);
});

test('fenced code, inline code, HTML comments and front matter are not prose', () => {
  const doc = [
    '---',
    `title: A ${EM} B`,
    '---',
    '',
    '```bash',
    'npm test -- --coverage',
    `echo "a ${EM} b"`,
    '```',
    '',
    '  ~~~sql',
    '  -- a SQL comment',
    '  ```',
    '  still inside the tilde fence -- yes',
    '  ~~~',
    '',
    '````md',
    '```',
    'nested fence -- still code',
    '```',
    '````',
    '',
    `Run \`git log -- path\` and \`\`a ${EM} b\`\` inline.`,
    `<!-- a comment ${EM} here -->`,
    '<!--',
    'a multi-line comment -- here',
    '-->',
    'Prose after the comment.',
  ].join('\n');
  assert.deepEqual(rules(doc), []);
});

test('flags, compounds, ranges and rules are not dashes', () => {
  const doc = [
    'Pass `--flag` or --flag=value to the command.',
    'An end-to-end test of a well-known path.',
    `Scores range 0.0${EN}1.0 and 1${EN}50M parameters.`,
    '---',
    '| a | b |',
    '|---|---|',
    '- a list item',
  ].join('\n');
  assert.deepEqual(rules(doc), []);
});

test('a dash after a closed fence is reported at its own line', () => {
  const doc = ['```', 'code -- here', '```', `Prose ${EM} here.`].join('\n');
  assert.deepEqual(rules(doc), ['4:em-dash']);
});

test('the candidate set leaves out history, licenses, installed code and test data', () => {
  for (const rel of [
    'README.md',
    'docs/guide.md',
    'docs/page.mdx',
    'examples/README.md',
    'sdk/python/README.md',
  ]) {
    assert.equal(inScope(rel), true, rel);
  }
  for (const rel of [
    'CHANGELOG.md',
    'sdk/python/CHANGELOG.md',
    'changelog.d/123.md',
    'LICENSE.md',
    'node_modules/pkg/README.md',
    'vendor/x/README.md',
    'dist/README.md',
    'build/notes.md',
    'tests/README.md',
    'sdk/python/tests/README.md',
    'sdk/typescript/test/fixtures/x/README.md',
    'scripts/public-wording/testdata/fail.md',
    'corpus/a.md',
    'golden/a.md',
    'src/__snapshots__/a.md',
    'notes.txt',
  ]) {
    assert.equal(inScope(rel), false, rel);
  }
});

test('the command exits 1 and names file, line and rule for each finding', () => {
  const dir = repoWith({
    'README.md': `# Title\n\nClean line.\nA dash ${EM} here.\n`,
    'docs/guide.md': '## Why this exists\n',
    'CHANGELOG.md': `History ${EM} untouched.\n`,
    'tests/README.md': 'Test data -- untouched.\n',
  });
  const res = cli(dir);
  assert.equal(res.status, 1, res.stderr);
  assert.deepEqual(res.stdout.trim().split('\n').sort(), [
    'README.md:4: em-dash',
    'docs/guide.md:1: why-heading',
  ]);
});

test('the command exits 0 on a clean tree and does not read untracked files', () => {
  const dir = repoWith({ 'README.md': '# Title\n\nClean: plain punctuation.\n' });
  fs.writeFileSync(path.join(dir, 'untracked.md'), `Not tracked ${EM} ignored.\n`);
  const res = cli(dir);
  assert.equal(res.status, 0, res.stdout + res.stderr);
  assert.match(res.stdout, /1 file\(s\) clean/);
});

test('the command exits 2 when the tree cannot be enumerated', () => {
  const dir = tempDir('docs-prose-nogit-');
  const res = spawnSync(process.execPath, [SCRIPT, '--root', dir], {
    encoding: 'utf8',
    env: { ...process.env, GIT_CEILING_DIRECTORIES: path.dirname(dir) },
  });
  assert.equal(res.status, 2, res.stdout + res.stderr);
});

// git in `cwd` with a scratch identity, finding its repository from `cwd`.
function git(cwd, ...args) {
  return spawnSync(
    'git',
    ['-c', 'user.name=docs-prose', '-c', 'user.email=docs-prose@example.invalid', ...args],
    { cwd, encoding: 'utf8', env: gitEnv() },
  );
}

test('a GIT_DIR exported by a hook in a linked worktree reaches no repository', () => {
  // A pre-push hook started from a linked worktree runs with GIT_DIR set to
  // the worktree's gitdir under the main checkout's .git/worktrees/.
  const base = tempDir('docs-prose-hook-');
  const main = path.join(base, 'main');
  const linked = path.join(base, 'linked');
  for (const args of [
    ['init', '-q', main],
    ['-C', main, 'commit', '-q', '--no-verify', '--no-gpg-sign', '--allow-empty', '-m', 'init'],
    ['-C', main, 'worktree', 'add', '-q', '--detach', linked],
  ]) {
    const res = git(base, ...args);
    assert.equal(res.status, 0, res.stderr);
  }
  const gitdir = git(linked, 'rev-parse', '--absolute-git-dir').stdout.trim();
  assert.equal(gitdir, path.join(main, '.git', 'worktrees', 'linked'));

  const saved = process.env.GIT_DIR;
  process.env.GIT_DIR = gitdir;
  let res;
  let direct;
  try {
    const dir = repoWith({ 'README.md': `A dash ${EM} here.\n` });
    res = cli(dir);
    direct = run(dir);
  } finally {
    if (saved === undefined) delete process.env.GIT_DIR;
    else process.env.GIT_DIR = saved;
  }

  assert.deepEqual(
    {
      bare: git(main, 'config', '--get', 'core.bare').stdout.trim(),
      staged: git(linked, 'diff', '--cached', '--name-only').stdout,
    },
    { bare: 'false', staged: '' },
  );
  assert.equal(res.status, 1, res.stderr);
  assert.equal(res.stdout.trim(), 'README.md:1: em-dash');
  assert.deepEqual(direct.findings, [{ path: 'README.md', line: 1, rule: 'em-dash' }]);
});
