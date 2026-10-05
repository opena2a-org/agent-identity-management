// node:test cells for scripts/lint-public-surface.mjs. Run from the
// repository root:
//
//   node --test scripts/test-lint-public-surface.mjs
//
// Every home directory path a cell needs is assembled at run time, and every
// fixture tree is a fresh git repository in a temporary directory, so this
// file carries no line the scanner would report.

import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';

import { BUILT_IN, parseForbidden, run, scanText } from './lint-public-surface.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const SCRIPT = path.join(HERE, 'lint-public-surface.mjs');
const REPO = path.resolve(HERE, '..');

const HOME_PATH = ['', 'Users', 'someone', 'workspace', 'aim', 'deploy.sh'].join('/');

function tmpdir(prefix) {
  return fs.mkdtempSync(path.join(os.tmpdir(), prefix));
}

// A git repository whose index holds `tracked`; `untracked` files are written
// but never added.
function fixtureRepo({ tracked = {}, untracked = {} } = {}) {
  const dir = tmpdir('public-surface-repo-');
  const git = (...args) => {
    const r = spawnSync('git', ['-C', dir, '-c', 'init.defaultBranch=main', ...args], { encoding: 'utf8' });
    assert.equal(r.status, 0, r.stderr);
  };
  git('init', '-q');
  const write = (files) => {
    for (const [rel, content] of Object.entries(files)) {
      fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
      fs.writeFileSync(path.join(dir, rel), content);
    }
  };
  write(tracked);
  write(untracked);
  if (Object.keys(tracked).length) git('add', '--', ...Object.keys(tracked));
  return dir;
}

function forbiddenFile(text) {
  const file = path.join(tmpdir('public-surface-forbidden-'), 'forbidden.tsv');
  fs.writeFileSync(file, text);
  return file;
}

function cli(root, forbidden) {
  const args = [SCRIPT, '--root', root];
  if (forbidden) args.push('--forbidden', forbidden);
  const env = { ...process.env, PUBLIC_SURFACE_FORBIDDEN_FILE: '' };
  const r = spawnSync(process.execPath, args, { encoding: 'utf8', env });
  return { status: r.status, stdout: r.stdout, stderr: r.stderr };
}

test('no tracked file in this repository names a home directory path', () => {
  const lines = [];
  const status = run({ root: REPO, out: (s) => lines.push(s), err: (s) => lines.push(s) });
  assert.equal(status, 0, lines.join('\n'));
});

test('a home directory path in a tracked doc fails the run by path, line and class, without printing it', () => {
  const root = fixtureRepo({
    tracked: { 'docs/deploy.md': `# Deploy\n\nRun:\n\n    bash ${HOME_PATH}\n` },
  });
  const r = cli(root);
  assert.equal(r.status, 1, r.stdout + r.stderr);
  assert.match(r.stderr, /^docs\/deploy\.md:5: local-home-path$/m);
  assert.doesNotMatch(r.stdout + r.stderr, /someone/);
});

test('a tree with no finding passes, and says every canary was caught', () => {
  const root = fixtureRepo({ tracked: { 'README.md': 'Run `bash ~/aim/deploy.sh` from your checkout.\n' } });
  const r = cli(root);
  assert.equal(r.status, 0, r.stderr);
  assert.match(r.stdout, /scanned 1 tracked files; planted control caught 1 of 1 canaries/);
});

test('an untracked file is not read', () => {
  const root = fixtureRepo({
    tracked: { 'README.md': 'clean\n' },
    untracked: { 'notes.md': `${HOME_PATH}\n` },
  });
  assert.equal(cli(root).status, 0);
});

test('a Users segment inside an API path is not a home directory', () => {
  const route = ['', 'scim', 'v2', 'Users', ':id'].join('/');
  const findings = scanText('api.go', `router.Get("${route}", h.GetUser)\n`, BUILT_IN);
  assert.deepEqual(findings, []);
});

test('a class from the forbidden file is reported by class name, and its pattern is never printed', () => {
  const root = fixtureRepo({
    tracked: { 'src/handler.go': '// see plans/internal/rate-limits.md for the numbers\nfunc h() {}\n' },
  });
  const r = cli(root, forbiddenFile('internal-plan\tplans/internal/\tsee plans/internal/x.md\n'));
  assert.equal(r.status, 1, r.stdout + r.stderr);
  assert.match(r.stderr, /^src\/handler\.go:1: internal-plan$/m);
  assert.doesNotMatch(r.stdout + r.stderr, /plans\/internal/);
});

test('the forbidden file can also come from PUBLIC_SURFACE_FORBIDDEN_FILE', () => {
  const root = fixtureRepo({ tracked: { 'a.md': 'see plans/internal/x.md\n' } });
  const env = { ...process.env, PUBLIC_SURFACE_FORBIDDEN_FILE: forbiddenFile('internal-plan\tplans/internal/\tplans/internal/x\n') };
  const r = spawnSync(process.execPath, [SCRIPT, '--root', root], { encoding: 'utf8', env });
  assert.equal(r.status, 1, r.stdout + r.stderr);
  assert.match(r.stderr, /^a\.md:1: internal-plan$/m);
});

test('a run of a class inside a data: URI base64 payload is not reported; the same run outside one is', () => {
  const run4 = 'zq9/';
  const payload = `iVBORw0KGgoAAAANSUhEUg${run4}AAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk==`;
  const svg = `<svg><image href="data:image/png;base64,${payload}"/></svg>\n`;
  const forbidden = forbiddenFile(`coincidence\t${run4}\tsee ${run4}notes\n`);

  const clean = fixtureRepo({ tracked: { 'public/logo.svg': svg } });
  const r1 = cli(clean, forbidden);
  assert.equal(r1.status, 0, r1.stderr);

  const dirty = fixtureRepo({ tracked: { 'public/logo.svg': svg, 'NOTES.md': `see ${run4}notes\n` } });
  const r2 = cli(dirty, forbidden);
  assert.equal(r2.status, 1, r2.stdout + r2.stderr);
  assert.match(r2.stderr, /^NOTES\.md:1: coincidence$/m);
  assert.doesNotMatch(r2.stderr, /logo\.svg/);
});

test('a run of a class inside a lockfile integrity digest is not reported', () => {
  const lock = `{\n  "integrity": "sha512-${'A'.repeat(40)}ZQ9/${'B'.repeat(40)}=="\n}\n`;
  const root = fixtureRepo({ tracked: { 'package-lock.json': lock } });
  const r = cli(root, forbiddenFile('coincidence\tzq9/\tsee zq9/x\n'));
  assert.equal(r.status, 0, r.stdout + r.stderr);
});

test('a class that does not catch its own canary makes the run inconclusive, not clean', () => {
  const root = fixtureRepo({ tracked: { 'README.md': 'clean\n' } });
  const r = cli(root, forbiddenFile('internal-plan\tplans/internal/\tnothing to see\n'));
  assert.equal(r.status, 2, r.stdout + r.stderr);
  assert.match(r.stderr, /class internal-plan did not catch its own canary/);
  assert.match(r.stderr, /INCONCLUSIVE: planted control caught 1 of 2 canaries/);
});

test('a missing or malformed forbidden file makes the run inconclusive', () => {
  const root = fixtureRepo({ tracked: { 'README.md': 'clean\n' } });
  const missing = cli(root, path.join(tmpdir('public-surface-none-'), 'absent.tsv'));
  assert.equal(missing.status, 2);
  assert.match(missing.stderr, /forbidden file .* does not exist/);

  const twoFields = cli(root, forbiddenFile('internal-plan\tplans/internal/\n'));
  assert.equal(twoFields.status, 2);
  assert.match(twoFields.stderr, /forbidden line 1: expected "class<TAB>regex<TAB>canary"/);

  const badRegex = cli(root, forbiddenFile('# comment\nbroken\t(unclosed\tx\n'));
  assert.equal(badRegex.status, 2);
  assert.match(badRegex.stderr, /forbidden line 2: not a valid regular expression/);
  assert.doesNotMatch(badRegex.stderr, /unclosed/);
});

test('parseForbidden reads the census format, skipping comments and blank lines', () => {
  const { classes, errors } = parseForbidden('# class\tregex\tcanary\n\ninternal-plan\tPLANS/internal/\tplans/internal/x\r\n');
  assert.deepEqual(errors, []);
  assert.equal(classes.length, 1);
  assert.equal(classes[0].id, 'internal-plan');
  assert.ok(classes[0].pattern.test('see plans/INTERNAL/x'), 'compiled case-insensitively');
});

test('a directory that is not a git work tree is inconclusive', () => {
  const r = cli(tmpdir('public-surface-plain-'));
  assert.equal(r.status, 2);
  assert.match(r.stderr, /INCONCLUSIVE: git ls-files failed/);
});
