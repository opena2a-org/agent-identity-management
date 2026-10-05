#!/usr/bin/env node
// Fails when a tracked file carries a reference a public reader cannot
// follow: a path under a macOS home directory, or any class the caller
// supplies.
//
// A home directory path resolves on one machine only. In a doc, a comment or
// a test it is a dead end for every other reader, and it names the account
// that wrote it. This script reads every path `git ls-files` returns, binary
// files included, line by line.
//
// Extra classes are kept outside this repository, the same way
// docs/demo/lib/census.mjs keeps its own, and in the same format: a
// tab-separated file of `class<TAB>regex<TAB>canary` lines, passed as
// --forbidden <file> or $PUBLIC_SURFACE_FORBIDDEN_FILE. One file serves both
// scripts. Each regex is compiled case-insensitively.
//
// Base64 no reader reads is blanked before matching: the payload of a data:
// URI and the digest of a subresource-integrity value (`sha512-...` in a
// lockfile). Neither carries text, and a short class turns up inside a long
// payload by chance.
//
// It fails closed:
//
//   0  clean
//   1  one or more findings
//   2  inconclusive: the tree could not be enumerated, a tracked path could
//      not be read, the forbidden file is missing or malformed, or a class
//      did not catch its own canary
//
// The planted control: before the verdict, every class's canary is scanned
// through the same matcher and must be caught by that class. The built-in
// canary is assembled at run time, so this file holds no path it would
// report.
//
// Findings print path, line and class, never the matched text: CI logs of a
// public repository are public, and an extra class can itself be a name that
// should stay out of them.
//
// Node built-ins only, so no install step has to run before it.
//
// Usage:
//   node scripts/lint-public-surface.mjs [--root <dir>] [--forbidden <file>]

import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));

export const BUILT_IN = [
  {
    id: 'local-home-path',
    // Not after a word character, a dot or a hyphen: a Users segment deeper
    // in a path (SCIM's Users collection, say) is an API route, not a home
    // directory.
    pattern: /(?<![A-Za-z0-9_.-])\/Users\/[A-Za-z0-9_.-]/,
    canary: ['', 'Users', 'canary', 'project'].join('/'),
  },
];

const BASE64_PAYLOAD = /(;base64,|\bsha(?:1|256|384|512)-)[A-Za-z0-9+/]+=*/gi;

export function scanText(relPath, text, classes) {
  const findings = [];
  const lines = text.split('\n');
  for (let i = 0; i < lines.length; i += 1) {
    const line = lines[i].replace(BASE64_PAYLOAD, '$1');
    for (const c of classes) {
      if (c.pattern.test(line)) findings.push({ path: relPath, line: i + 1, class: c.id });
    }
  }
  return findings;
}

// The regex and canary are never echoed back: they are what the file keeps
// out of this repository.
export function parseForbidden(text) {
  const classes = [];
  const errors = [];
  text.split('\n').forEach((raw, i) => {
    const line = raw.replace(/\r$/, '');
    if (line.trim() === '' || line.startsWith('#')) return;
    const [id, re, canary] = line.split('\t');
    if (!id || !re || !canary) {
      errors.push(`forbidden line ${i + 1}: expected "class<TAB>regex<TAB>canary"`);
      return;
    }
    let pattern;
    try {
      pattern = new RegExp(re, 'i');
    } catch {
      errors.push(`forbidden line ${i + 1}: not a valid regular expression`);
      return;
    }
    classes.push({ id, pattern, canary });
  });
  return { classes, errors };
}

function readTracked(root, relPath) {
  const abs = path.join(root, relPath);
  const st = fs.lstatSync(abs);
  if (st.isSymbolicLink()) return fs.readlinkSync(abs);
  return fs.readFileSync(abs).toString('latin1');
}

export function run({ root, forbiddenPath = null, out = console.log, err = console.error }) {
  const classes = [...BUILT_IN];
  if (forbiddenPath) {
    if (!fs.existsSync(forbiddenPath)) {
      err(`public-surface: INCONCLUSIVE: forbidden file ${forbiddenPath} does not exist`);
      return 2;
    }
    const parsed = parseForbidden(fs.readFileSync(forbiddenPath, 'utf8'));
    if (parsed.errors.length) {
      for (const e of parsed.errors) err(`public-surface: ${e}`);
      err('public-surface: INCONCLUSIVE: the forbidden file could not be read in full');
      return 2;
    }
    classes.push(...parsed.classes);
  }

  const missed = classes.filter((c) => !scanText('<planted>', c.canary, [c]).length);
  if (missed.length) {
    for (const c of missed) err(`public-surface: class ${c.id} did not catch its own canary`);
    err(`public-surface: INCONCLUSIVE: planted control caught ${classes.length - missed.length} of ${classes.length} canaries, so a clean result would prove nothing`);
    return 2;
  }

  const ls = spawnSync('git', ['-C', root, 'ls-files', '-z'], { encoding: 'buffer', maxBuffer: 1 << 28 });
  if (ls.error || ls.status !== 0) {
    err(`public-surface: INCONCLUSIVE: git ls-files failed in ${root}`);
    if (ls.stderr && ls.stderr.length) err(ls.stderr.toString().trim());
    return 2;
  }
  const files = ls.stdout.toString('utf8').split('\0').filter(Boolean);
  if (files.length === 0) {
    err(`public-surface: INCONCLUSIVE: git ls-files returned no tracked files in ${root}`);
    return 2;
  }

  const unreadable = [];
  const findings = [];
  for (const rel of files) {
    let text;
    try {
      text = readTracked(root, rel);
    } catch (e) {
      unreadable.push(`${rel}: ${e.code || e.message}`);
      continue;
    }
    findings.push(...scanText(rel, text, classes));
  }

  for (const f of findings) err(`${f.path}:${f.line}: ${f.class}`);
  if (unreadable.length) {
    for (const u of unreadable) err(`public-surface: cannot read tracked path ${u}`);
    err(`public-surface: INCONCLUSIVE: ${unreadable.length} of ${files.length} tracked paths could not be read`);
    return 2;
  }
  if (findings.length) {
    err(
      `public-surface: ${findings.length} line(s) in tracked files carry a reference a public reader cannot follow. ` +
        'Replace each with a public link or a placeholder, or delete it.',
    );
    return 1;
  }
  out(
    `public-surface: scanned ${files.length} tracked files; planted control caught ${classes.length} of ${classes.length} canaries ` +
      `(${BUILT_IN.length} built in, ${classes.length - BUILT_IN.length} from the forbidden file); 0 findings`,
  );
  return 0;
}

function main(argv) {
  let root = null;
  let forbiddenPath = process.env.PUBLIC_SURFACE_FORBIDDEN_FILE || null;
  for (let i = 0; i < argv.length; i += 1) {
    if (argv[i] === '--root' && argv[i + 1]) root = argv[++i];
    else if (argv[i] === '--forbidden' && argv[i + 1]) forbiddenPath = argv[++i];
    else {
      console.error('usage: node scripts/lint-public-surface.mjs [--root <dir>] [--forbidden <file>]');
      return 2;
    }
  }
  if (root === null) {
    const top = spawnSync('git', ['-C', HERE, 'rev-parse', '--show-toplevel'], { encoding: 'utf8' });
    if (top.status !== 0) {
      console.error('public-surface: INCONCLUSIVE: not inside a git work tree');
      return 2;
    }
    root = top.stdout.trim();
  }
  return run({ root: path.resolve(root), forbiddenPath });
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
