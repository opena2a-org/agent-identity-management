// node:test cells for scripts/lint-token-literals.mjs. Run from the
// repository root:
//
//   node --test scripts/test-lint-token-literals.mjs
//
// Every token a cell needs is assembled at run time, and every fixture tree is
// a fresh git repository in a temporary directory, so this file carries no
// literal the scanner would report.

import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';

import { RULES, fingerprint, plantedSamples, run, scanText } from './lint-token-literals.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const SCRIPT = path.join(HERE, 'lint-token-literals.mjs');
const REPO = path.resolve(HERE, '..');

const b64url = (s) => Buffer.from(s, 'utf8').toString('base64url');
const JWT =
  `${b64url('{"alg":"HS256","typ":"JWT"}')}.` +
  `${b64url('{"sub":"admin","role":"admin","exp":4102444800}')}.` +
  `${Buffer.alloc(32, 9).toString('base64url')}`;

function tmpdir(prefix) {
  return fs.mkdtempSync(path.join(os.tmpdir(), prefix));
}

// A git repository whose index holds `tracked`; `untracked` files are written
// but never added.
function fixtureRepo({ tracked = {}, untracked = {} } = {}) {
  const dir = tmpdir('token-literals-repo-');
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

function cli(root, allowlistText) {
  const allow = path.join(tmpdir('token-literals-allow-'), 'allowlist.txt');
  if (allowlistText !== undefined) fs.writeFileSync(allow, allowlistText);
  const r = spawnSync(process.execPath, [SCRIPT, '--root', root, '--allowlist', allow], { encoding: 'utf8' });
  return { status: r.status, stdout: r.stdout, stderr: r.stderr };
}

test('a tree with no token-shaped literal passes, and says the planted control was caught', () => {
  const root = fixtureRepo({ tracked: { 'src/app.py': 'TOKEN = os.environ.get("AIM_E2E_TOKEN")\n' } });
  const r = cli(root);
  assert.equal(r.status, 0, r.stderr);
  assert.match(r.stdout, /scanned 1 tracked files/);
  assert.match(r.stdout, new RegExp(`planted control caught ${RULES.length} of ${RULES.length} samples`));
});

test('a JWT literal in a tracked test file fails the run, by path and line, without printing it', () => {
  const root = fixtureRepo({
    tracked: { 'tests/e2e/test_drift_e2e.py': `import os\n\nTOKEN = "${JWT}"\n` },
  });
  const r = cli(root);
  assert.equal(r.status, 1);
  assert.match(r.stderr, /^tests\/e2e\/test_drift_e2e\.py:3: jwt \(fingerprint [0-9a-f]{16}\)$/m);
  assert.ok(!r.stderr.includes(JWT) && !r.stdout.includes(JWT), 'the token itself must not be printed');
  assert.ok(!r.stderr.includes(JWT.split('.')[1]), 'no segment of the token may be printed');
});

test('an unsigned three-segment token (empty signature) is still reported', () => {
  const unsigned = `${JWT.split('.').slice(0, 2).join('.')}.`;
  const root = fixtureRepo({ tracked: { 'conftest.py': `T = '${unsigned}'\n` } });
  const r = cli(root);
  assert.equal(r.status, 1);
  assert.match(r.stderr, /conftest\.py:1: jwt /);
});

test('a token compiled into a tracked binary file is reported', () => {
  const bin = Buffer.concat([Buffer.from([0, 1, 2, 0xff, 0]), Buffer.from(JWT), Buffer.from([0, 0xfe, 10])]);
  const root = fixtureRepo({ tracked: { 'bin/server': bin } });
  const r = cli(root);
  assert.equal(r.status, 1);
  assert.match(r.stderr, /bin\/server:1: jwt /);
});

test('only tracked files are in scope: an untracked file with a token is not read', () => {
  const root = fixtureRepo({
    tracked: { 'README.md': 'clean\n' },
    untracked: { 'scratch.txt': `${JWT}\n` },
  });
  const r = cli(root);
  assert.equal(r.status, 0, r.stderr);
  assert.match(r.stdout, /scanned 1 tracked files/);
});

test('every rule catches the planted sample written for it', () => {
  const samples = plantedSamples();
  assert.deepEqual(
    samples.map((s) => s.rule).sort(),
    RULES.map((r) => r.id).sort(),
    'one planted sample per rule',
  );
  samples.forEach((s) => {
    const rules = scanText('x', s.line).map((f) => f.rule);
    assert.ok(rules.includes(s.rule), `${s.rule} sample not caught`);
  });
});

test('placeholders and low-entropy test values are not reported', () => {
  const text = [
    'password: "Test123!"',
    'api_key="your-api-key-goes-here"',
    'token = "eyJhbGciOiJIUzI1NiIs..."',
    'AIM_API_KEY=aim_live_...',
    'password = "SecurePassword123!"',
    'client_secret: "${CLIENT_SECRET}"',
    'secret_key = "<your-secret-key-here-please>"',
    'github.com/opena2a-org/agent-identity-management/apps/backend',
    'const ident = some_long_identifier_name.another_identifier;',
  ].join('\n');
  assert.deepEqual(scanText('docs/x.md', text), []);
});

test('a high-entropy value assigned to a credential-named field is reported', () => {
  const value = `${createHash('sha256').update('credential-assignment-fixture').digest('base64url').slice(0, 28)}!`;
  const findings = scanText('config/app.yaml', `  dbPassword: "${value}"\n`);
  assert.deepEqual(findings.map((f) => f.rule), ['credential-assignment']);
});

test('an allowlist entry suppresses that literal at that path only', () => {
  const line = `const key = "${'s' + 'k-'}${b64url('synthetic-detector-input-0001')}";\n`;
  const root = fixtureRepo({ tracked: { 'tests/detector.test.ts': line, 'src/client.ts': line } });
  const fp = fingerprint(line.match(/sk-[A-Za-z0-9_-]+/)[0]);
  const r = cli(root, `sk-api-key ${fp} tests/detector.test.ts  # detector input\n`);
  assert.equal(r.status, 1);
  assert.match(r.stderr, /src\/client\.ts:1: sk-api-key /);
  assert.doesNotMatch(r.stderr, /tests\/detector\.test\.ts/);
});

test('an allowlist entry that matches nothing fails the run', () => {
  const root = fixtureRepo({ tracked: { 'README.md': 'clean\n' } });
  const r = cli(root, 'jwt 0123456789abcdef tests/gone.py  # removed fixture\n');
  assert.equal(r.status, 1);
  assert.match(r.stderr, /allowlist line 1 matches nothing/);
});

test('an allowlist entry without a reason is refused', () => {
  const root = fixtureRepo({ tracked: { 'README.md': 'clean\n' } });
  const r = cli(root, 'jwt 0123456789abcdef README.md\n');
  assert.equal(r.status, 1);
  assert.match(r.stderr, /needs a "# reason" comment/);
});

test('a rule that cannot fire makes the run inconclusive, not clean', () => {
  const root = fixtureRepo({ tracked: { 'README.md': 'clean\n' } });
  const broken = RULES.map((r) => (r.id === 'jwt' ? { ...r, pattern: /(?!)/g } : r));
  const errors = [];
  const status = run({
    root,
    allowlistPath: path.join(root, 'no-allowlist.txt'),
    rules: broken,
    out: () => {},
    err: (m) => errors.push(m),
  });
  assert.equal(status, 2);
  assert.ok(errors.some((m) => /planted jwt sample was not caught/.test(m)), errors.join('\n'));
  assert.ok(errors.some((m) => /INCONCLUSIVE: planted control caught/.test(m)), errors.join('\n'));
});

test('a directory that is not a git work tree is inconclusive', () => {
  const r = cli(tmpdir('token-literals-nogit-'));
  assert.equal(r.status, 2);
  assert.match(r.stderr, /INCONCLUSIVE: git ls-files failed/);
});

test('a repository with no tracked files is inconclusive', () => {
  const r = cli(fixtureRepo());
  assert.equal(r.status, 2);
  assert.match(r.stderr, /INCONCLUSIVE: git ls-files returned no tracked files/);
});

test('a tracked path that cannot be read is inconclusive', () => {
  const root = fixtureRepo({ tracked: { 'a.txt': 'clean\n', 'b.txt': 'clean\n' } });
  fs.rmSync(path.join(root, 'b.txt'));
  const r = cli(root);
  assert.equal(r.status, 2);
  assert.match(r.stderr, /cannot read tracked path b\.txt/);
  assert.match(r.stderr, /INCONCLUSIVE: 1 of 2 tracked paths could not be read/);
});

test('the scanner and this test file carry no literal the scanner reports', () => {
  for (const file of [SCRIPT, fileURLToPath(import.meta.url)]) {
    assert.deepEqual(scanText(path.basename(file), fs.readFileSync(file, 'latin1')), [], file);
  }
});

test("this repository's tracked tree is clean under the committed allowlist", () => {
  const r = spawnSync(process.execPath, [SCRIPT, '--root', REPO], { encoding: 'utf8' });
  assert.equal(r.status, 0, r.stderr);
  assert.match(r.stdout, /0 findings/);
});
