#!/usr/bin/env node
// Fails when the places that state the platform's version disagree.
//
// The platform's version is the `version` of the root package.json. A
// platform release is tagged platform-v<version> from a commit whose
// apps/web/package.json carries the same version and whose newest
// CHANGELOG.md release section is that version's (`## [1.1.0] - <date>`).
// docker-publish.yml builds the images from the tag: the image's X.Y.Z tag
// and the version the backend's /health/ready reports are the X.Y.Z the tag
// carries, as written.
//
// On a tree, the check refuses:
//
//   - an apps/web/package.json version other than the root package.json's;
//   - a newest release section of CHANGELOG.md, the first `## [<version>]`
//     heading other than `## [Unreleased]` outside a fenced code block, for
//     another version than the root package.json's.
//
// Given a tag, it also refuses:
//
//   - a tag that is not platform-vX.Y.Z, three numbers and no suffix, the
//     form the release guard of docker-publish.yml accepts;
//   - a tag whose X.Y.Z, as written, is not the root package.json's version:
//     platform-v01.1.0 does not match 1.1.0.
//
// The tag comes from the argument alone, never from the environment: CI also
// runs this check, through the TypeScript SDK's tests, on pull requests, on
// main and on TypeScript SDK tags, where GITHUB_REF_NAME names another ref.
//
// Exit status:
//   0  the versions agree
//   1  one or more disagreements, one `refused:` line each
//   2  a file could not be read or parsed, a manifest carries no version,
//      CHANGELOG.md has no release section, or the arguments are unusable
//
// Node built-ins only, so no install step has to run before it.
//
// Usage:
//   node scripts/check-platform-version.mjs [--root <dir>] [platform-vX.Y.Z]

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const NAME = 'check-platform-version';
const USAGE = `usage: node scripts/${NAME}.mjs [--root <dir>] [platform-vX.Y.Z]`;

// The tag form the release guard of docker-publish.yml accepts.
export const RELEASE_TAG = /^platform-v([0-9]+)\.([0-9]+)\.([0-9]+)$/;

const RELEASE_HEADING = /^## \[([^\]]+)\]/;
const FENCE_OPEN = /^\s*(`{3,}|~{3,})/;

// The version of the newest release section of a CHANGELOG.md text: the first
// `## [<version>]` heading outside a fenced code block that is not
// `## [Unreleased]`. null when the text has none.
export function newestRelease(changelog) {
  let fence = null;
  for (const raw of changelog.split('\n')) {
    const line = raw.replace(/\r$/, '');
    const m = line.match(FENCE_OPEN);
    if (fence) {
      if (m && m[1][0] === fence[0] && m[1].length >= fence.length && line.trim() === m[1]) fence = null;
      continue;
    }
    if (m) {
      fence = m[1];
      continue;
    }
    const heading = line.match(RELEASE_HEADING);
    if (heading && heading[1].toLowerCase() !== 'unreleased') return heading[1];
  }
  return null;
}

function readText(root, rel, errors) {
  try {
    return fs.readFileSync(path.join(root, rel), 'utf8');
  } catch (e) {
    errors.push(`cannot read ${rel}: ${e.code || e.message}`);
    return null;
  }
}

function manifestVersion(root, rel, errors) {
  const text = readText(root, rel, errors);
  if (text === null) return null;
  let manifest;
  try {
    manifest = JSON.parse(text);
  } catch (e) {
    errors.push(`${rel} is not JSON: ${e.message}`);
    return null;
  }
  const version = manifest !== null && typeof manifest === 'object' ? manifest.version : undefined;
  if (typeof version !== 'string' || version === '') {
    errors.push(`${rel} carries no "version" string`);
    return null;
  }
  return version;
}

// Checks the tree at `root`, and `tag` when it is not null. Returns the exit
// status and the lines to print on stdout and stderr.
export function check({ root, tag = null }) {
  const errors = [];
  const version = manifestVersion(root, 'package.json', errors);
  const web = manifestVersion(root, 'apps/web/package.json', errors);
  const changelog = readText(root, 'CHANGELOG.md', errors);
  const released = changelog === null ? null : newestRelease(changelog);
  if (changelog !== null && released === null) {
    errors.push('CHANGELOG.md has no release section: no `## [<version>]` heading other than `## [Unreleased]` outside a fenced code block');
  }
  if (errors.length) {
    return { status: 2, stdout: [], stderr: errors.map((e) => `${NAME}: ${e}`) };
  }

  const refusals = [];
  if (web !== version) {
    refusals.push(
      `apps/web/package.json reads ${web} and package.json reads ${version}; ` +
        'set both to the platform version',
    );
  }
  if (released !== version) {
    refusals.push(
      `CHANGELOG.md's newest release section is [${released}] and package.json reads ${version}; ` +
        `the commit that opens the [${released}] section sets package.json and apps/web/package.json to ${released}`,
    );
  }
  if (tag !== null) {
    const m = tag.match(RELEASE_TAG);
    if (!m) {
      refusals.push(`${tag}: a platform release tag is exactly platform-vX.Y.Z, three numbers and no suffix`);
    } else if (`${m[1]}.${m[2]}.${m[3]}` !== version) {
      const tagged = `${m[1]}.${m[2]}.${m[3]}`;
      refusals.push(
        `${tag}: package.json reads ${version}; tag a platform release from the commit whose package.json, ` +
          `apps/web/package.json and newest CHANGELOG.md release section read ${tagged}`,
      );
    }
  }
  if (refusals.length) {
    return { status: 1, stdout: [], stderr: refusals.map((r) => `${NAME}: refused: ${r}`) };
  }
  const subject = tag === null ? '' : `${tag}: `;
  return {
    status: 0,
    stdout: [
      `${NAME}: ok: ${subject}package.json, apps/web/package.json and CHANGELOG.md's newest release section read ${version}`,
    ],
    stderr: [],
  };
}

function main(argv) {
  let root = path.resolve(HERE, '..');
  let tag = null;
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '--root' && argv[i + 1]) {
      root = path.resolve(argv[++i]);
    } else if (arg !== '' && !arg.startsWith('-') && tag === null) {
      tag = arg;
    } else {
      console.error(USAGE);
      return 2;
    }
  }
  const result = check({ root, tag });
  for (const line of result.stdout) console.log(line);
  for (const line of result.stderr) console.error(line);
  return result.status;
}

// Compared by real path: a symlinked checkout path must not turn the CLI into
// a silent no-op that exits 0.
function invokedDirectly() {
  if (!process.argv[1]) return false;
  try {
    return fs.realpathSync(process.argv[1]) === fs.realpathSync(fileURLToPath(import.meta.url));
  } catch {
    return false;
  }
}

if (invokedDirectly()) {
  process.exitCode = main(process.argv.slice(2));
}
