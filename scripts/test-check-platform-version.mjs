// node:test cells for scripts/check-platform-version.mjs. Run from the
// repository root:
//
//   node --test scripts/test-check-platform-version.mjs
//
// The first cell checks this repository. Every other cell writes the three
// files the check reads, package.json, apps/web/package.json and
// CHANGELOG.md, into a fresh temporary directory and points the check at it.

import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { after, test } from 'node:test';
import { fileURLToPath } from 'node:url';

import { check, newestRelease } from './check-platform-version.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const SCRIPT = path.join(HERE, 'check-platform-version.mjs');
const REPO = path.resolve(HERE, '..');

const trees = [];
after(() => {
  for (const dir of trees) fs.rmSync(dir, { recursive: true, force: true });
});

// A CHANGELOG.md with an Unreleased section above one section per release,
// newest first.
function changelog(...releases) {
  const sections = releases.map((r) => `## [${r}] - 2026-06-01\n\n### Added — the ${r} entry\n`);
  return ['# Changelog\n', '## [Unreleased]\n\n### Fixed — an entry for the next release\n', ...sections].join('\n');
}

const manifest = (name, version) => `${JSON.stringify({ name, version, private: true }, null, 2)}\n`;

// A tree of the three files. `files` replaces a file's text, or removes the
// file when its value is null.
function tree({ root = '1.0.0', web = '1.0.0', releases = ['1.0.0'], files = {} } = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'check-platform-version-'));
  trees.push(dir);
  const texts = {
    'package.json': manifest('agent-identity-management', root),
    'apps/web/package.json': manifest('@aim/web', web),
    'CHANGELOG.md': changelog(...releases),
    ...files,
  };
  for (const [rel, text] of Object.entries(texts)) {
    if (text === null) continue;
    fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
    fs.writeFileSync(path.join(dir, rel), text);
  }
  return dir;
}

function cli(args, env = process.env) {
  const r = spawnSync(process.execPath, [SCRIPT, ...args], { encoding: 'utf8', env });
  return { status: r.status, stdout: r.stdout, stderr: r.stderr };
}

const refusals = (stderr) => stderr.split('\n').filter((line) => line.startsWith('check-platform-version: refused: '));

test('this repository passes, with no tag and with the tag of its newest CHANGELOG.md release section', () => {
  const released = newestRelease(fs.readFileSync(path.join(REPO, 'CHANGELOG.md'), 'utf8'));
  assert.ok(released, 'CHANGELOG.md has no release section');

  const bare = cli([]);
  assert.equal(bare.status, 0, bare.stderr);
  assert.equal(
    bare.stdout,
    `check-platform-version: ok: package.json, apps/web/package.json and CHANGELOG.md's newest release section read ${released}\n`,
  );

  const tagged = cli([`platform-v${released}`]);
  assert.equal(tagged.status, 0, tagged.stderr);
  assert.ok(tagged.stdout.startsWith(`check-platform-version: ok: platform-v${released}: `), tagged.stdout);
});

test('a tree whose two manifests and newest release section agree passes, with no tag and with its tag', () => {
  const dir = tree({ root: '1.1.0', web: '1.1.0', releases: ['1.1.0', '1.0.0'] });

  const bare = cli(['--root', dir]);
  assert.equal(bare.status, 0, bare.stderr);
  assert.equal(
    bare.stdout,
    "check-platform-version: ok: package.json, apps/web/package.json and CHANGELOG.md's newest release section read 1.1.0\n",
  );
  assert.equal(bare.stderr, '');

  const tagged = cli(['--root', dir, 'platform-v1.1.0']);
  assert.equal(tagged.status, 0, tagged.stderr);
  assert.equal(
    tagged.stdout,
    "check-platform-version: ok: platform-v1.1.0: package.json, apps/web/package.json and CHANGELOG.md's newest release section read 1.1.0\n",
  );
});

test('a release section opened without setting the manifests is refused: [1.1.0] over two manifests at 1.0.0', () => {
  const dir = tree({ root: '1.0.0', web: '1.0.0', releases: ['1.1.0', '1.0.0'] });

  const r = cli(['--root', dir]);
  assert.equal(r.status, 1);
  assert.equal(r.stdout, '');
  assert.deepEqual(refusals(r.stderr), [
    "check-platform-version: refused: CHANGELOG.md's newest release section is [1.1.0] and package.json reads 1.0.0; " +
      'the commit that opens the [1.1.0] section sets package.json and apps/web/package.json to 1.1.0',
  ]);
});

test('an apps/web/package.json version other than the root package.json version is refused', () => {
  const dir = tree({ root: '1.1.0', web: '1.0.0', releases: ['1.1.0'] });

  const r = cli(['--root', dir]);
  assert.equal(r.status, 1);
  assert.deepEqual(refusals(r.stderr), [
    'check-platform-version: refused: apps/web/package.json reads 1.0.0 and package.json reads 1.1.0; set both to the platform version',
  ]);
});

test('a tag naming another version than package.json is refused: platform-v1.1.0 on a tree at 1.0.0', () => {
  const dir = tree();

  const r = cli(['--root', dir, 'platform-v1.1.0']);
  assert.equal(r.status, 1);
  assert.deepEqual(refusals(r.stderr), [
    'check-platform-version: refused: platform-v1.1.0: package.json reads 1.0.0; tag a platform release from the commit ' +
      'whose package.json, apps/web/package.json and newest CHANGELOG.md release section read 1.1.0',
  ]);
});

test('a tag is compared as written: platform-v01.0.0 does not match 1.0.0', () => {
  const r = cli(['--root', tree(), 'platform-v01.0.0']);
  assert.equal(r.status, 1);
  assert.equal(refusals(r.stderr).length, 1);
  assert.match(r.stderr, /refused: platform-v01\.0\.0: package\.json reads 1\.0\.0;/);
});

test('a tag that is not platform-vX.Y.Z is refused', () => {
  const dir = tree();
  for (const tag of [
    'v1.0.0',
    '1.0.0',
    'platform-v1.0',
    'platform-v1.0.0.1',
    'platform-v1.0.0-rc.1',
    'platform-v1.0.0+build.7',
    'sdk-ts-v1.0.0',
    'refs/tags/platform-v1.0.0',
    'platform-v1.0.0 ',
  ]) {
    const r = cli(['--root', dir, tag]);
    assert.equal(r.status, 1, `${JSON.stringify(tag)}: ${r.stderr}`);
    assert.deepEqual(refusals(r.stderr), [
      `check-platform-version: refused: ${tag}: a platform release tag is exactly platform-vX.Y.Z, three numbers and no suffix`,
    ]);
  }
});

test('each disagreement is reported on its own refused line', () => {
  const dir = tree({ root: '1.1.0', web: '1.0.0', releases: ['1.0.0'] });

  const r = cli(['--root', dir, 'platform-v1.2.0']);
  assert.equal(r.status, 1);
  const lines = refusals(r.stderr);
  assert.equal(lines.length, 3, r.stderr);
  assert.match(lines[0], /apps\/web\/package\.json reads 1\.0\.0 and package\.json reads 1\.1\.0/);
  assert.match(lines[1], /newest release section is \[1\.0\.0\] and package\.json reads 1\.1\.0/);
  assert.match(lines[2], /platform-v1\.2\.0: package\.json reads 1\.1\.0/);
});

test('the newest release section is the first heading after [Unreleased], outside fenced code blocks', () => {
  assert.equal(newestRelease(changelog('1.1.0', '1.0.0')), '1.1.0');
  assert.equal(newestRelease('# Changelog\n\n## [unreleased]\n\n## [1.0.0] - 2026-06-01\n'), '1.0.0');
  assert.equal(newestRelease('## [1.0.0] - 2026-06-01\r\n'), '1.0.0');
  assert.equal(
    newestRelease(
      [
        '## [Unreleased]',
        '',
        '```markdown',
        '## [9.9.9] - 2026-01-01',
        '```',
        '',
        '~~~~',
        '## [8.8.8] - 2026-01-01',
        '```',
        '~~~~',
        '',
        '## [1.0.0] - 2026-06-01',
      ].join('\n'),
    ),
    '1.0.0',
  );
  assert.equal(newestRelease('## [Unreleased]\n\n### Added — a change\n'), null);
  assert.equal(newestRelease('## [Unreleased]\n\n### [1.0.0]\n'), null);
});

test('a file that cannot be read or parsed, a manifest with no version, or a changelog with no release section exits 2', () => {
  const cases = [
    [{ 'apps/web/package.json': null }, /cannot read apps\/web\/package\.json: ENOENT/],
    [{ 'package.json': null }, /cannot read package\.json: ENOENT/],
    [{ 'CHANGELOG.md': null }, /cannot read CHANGELOG\.md: ENOENT/],
    [{ 'package.json': '{"version": "1.0.0",' }, /package\.json is not JSON/],
    [{ 'package.json': '{"name": "agent-identity-management"}' }, /package\.json carries no "version" string/],
    [{ 'apps/web/package.json': '{"version": 1}' }, /apps\/web\/package\.json carries no "version" string/],
    [{ 'apps/web/package.json': '{"version": ""}' }, /apps\/web\/package\.json carries no "version" string/],
    [{ 'package.json': 'null' }, /package\.json carries no "version" string/],
    [{ 'CHANGELOG.md': '# Changelog\n\n## [Unreleased]\n' }, /CHANGELOG\.md has no release section/],
  ];
  for (const [files, message] of cases) {
    const dir = tree({ files });
    const r = cli(['--root', dir, 'platform-v1.0.0']);
    assert.equal(r.status, 2, `${JSON.stringify(files)}: ${r.stderr}`);
    assert.match(r.stderr, message);
    assert.equal(r.stdout, '');
    assert.deepEqual(refusals(r.stderr), []);
  }
});

test('arguments the check cannot use exit 2 with the usage line', () => {
  const dir = tree();
  for (const args of [
    ['--root', dir, 'platform-v1.0.0', 'platform-v1.0.0'],
    ['--root', dir, '--tag', 'platform-v1.0.0'],
    ['--root', dir, ''],
    ['--root'],
  ]) {
    const r = cli(args);
    assert.equal(r.status, 2, `${JSON.stringify(args)}: ${r.stderr}`);
    assert.equal(r.stderr, 'usage: node scripts/check-platform-version.mjs [--root <dir>] [platform-vX.Y.Z]\n');
  }
});

test('the tag comes from the argument alone: GITHUB_REF_NAME naming another ref changes nothing', () => {
  const dir = tree();
  for (const ref of ['sdk-ts-v9.9.9', 'platform-v9.9.9', 'main']) {
    const r = cli(['--root', dir], { ...process.env, GITHUB_REF_NAME: ref });
    assert.equal(r.status, 0, `${ref}: ${r.stderr}`);
  }
});

test('check() returns the status and the lines the command prints', () => {
  const dir = tree({ releases: ['1.1.0', '1.0.0'] });
  const result = check({ root: dir, tag: 'platform-v1.0.0' });
  assert.equal(result.status, 1);
  assert.deepEqual(result.stdout, []);
  assert.equal(result.stderr.length, 1);
  assert.equal(check({ root: tree() }).status, 0);
});
