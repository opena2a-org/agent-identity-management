#!/usr/bin/env node
// Fails when the prose of a tracked Markdown file uses a dash this
// repository's documents no longer use, or names a section "Why this exists"
// or "Why this matters".
//
// Four rules, checked on prose only:
//
//   spaced-double-hyphen  a ` -- ` written as a dash
//   em-dash               U+2014 anywhere in prose
//   spaced-en-dash        U+2013 with a space on both sides (a range such as
//                         1-50 written with an unspaced en dash is fine)
//   why-heading           a heading, or a line that is only bold text, that
//                         reads "Why this exists" or "Why this matters"
//
// Prose is what is left after removing YAML front matter, fenced code blocks,
// HTML comments and inline code spans. Inside those, `--` is an
// end-of-options separator or a flag, and quoted program output is compared
// against what the program prints, so they are never reported.
//
// The candidate set is `git ls-files '*.md' '*.mdx'`, without:
//   - CHANGELOG.md and changelog.d/ fragments (history is not rewritten)
//   - LICENSE files
//   - node_modules/, vendor/, dist/, build/
//   - test, fixture, corpus, golden and snapshot directories (their text is
//     data a test compares against)
//
// Exit status:
//   0  clean
//   1  one or more findings, printed as `file:line: rule`
//   2  the tree could not be enumerated or a tracked file could not be read
//
// Node built-ins only, so no install step has to run before it.
//
// Usage:
//   node scripts/check-docs-prose.mjs [--root <dir>]

import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));

const EXCLUDED_DIRS = new Set([
  'changelog.d',
  'node_modules',
  'vendor',
  'dist',
  'build',
  'test',
  'tests',
  'testdata',
  'fixture',
  'fixtures',
  '__fixtures__',
  'corpus',
  'golden',
  'goldens',
  'snapshots',
  '__snapshots__',
]);

export const RULES = [
  { id: 'spaced-double-hyphen', pattern: /(^|\s)--(\s|$)/ },
  { id: 'em-dash', pattern: /\u2014/ },
  { id: 'spaced-en-dash', pattern: /(^|\s)\u2013(\s|$)/ },
];

const WHY_HEADING = /^\s{0,3}(#{1,6}\s+|\*\*|__)\s*why this (exists|matters)\b/i;
const FENCE_OPEN = /^\s*(`{3,}|~{3,})/;
const INLINE_CODE = /(`+)[\s\S]*?\1/g;

export function inScope(relPath) {
  const parts = relPath.split('/');
  const base = parts[parts.length - 1];
  if (!/\.mdx?$/i.test(base)) return false;
  if (base === 'CHANGELOG.md' || /^LICENSE/i.test(base)) return false;
  return !parts.slice(0, -1).some((dir) => EXCLUDED_DIRS.has(dir));
}

// Returns [{ line, rule }] for one Markdown document.
export function scanMarkdown(text) {
  const findings = [];
  const lines = text.split('\n');
  let i = 0;

  if (lines[0] !== undefined && lines[0].trim() === '---') {
    const end = lines.findIndex((l, n) => n > 0 && l.trim() === '---');
    if (end > 0) i = end + 1;
  }

  let fence = null;
  let inComment = false;
  for (; i < lines.length; i += 1) {
    let line = lines[i];

    if (fence) {
      const m = line.match(FENCE_OPEN);
      if (m && m[1][0] === fence[0] && m[1].length >= fence.length && line.trim() === m[1]) {
        fence = null;
      }
      continue;
    }
    if (!inComment) {
      const m = line.match(FENCE_OPEN);
      if (m) {
        fence = m[1];
        continue;
      }
    }

    // Blank out HTML comments, which may span lines.
    let prose = '';
    let rest = line;
    while (rest.length > 0) {
      if (inComment) {
        const close = rest.indexOf('-->');
        if (close === -1) {
          rest = '';
        } else {
          rest = rest.slice(close + 3);
          inComment = false;
        }
      } else {
        const open = rest.indexOf('<!--');
        if (open === -1) {
          prose += rest;
          rest = '';
        } else {
          prose += rest.slice(0, open);
          rest = rest.slice(open + 4);
          inComment = true;
        }
      }
    }
    prose = prose.replace(INLINE_CODE, '');

    if (WHY_HEADING.test(prose)) findings.push({ line: i + 1, rule: 'why-heading' });
    for (const r of RULES) {
      if (r.pattern.test(prose)) findings.push({ line: i + 1, rule: r.id });
    }
  }
  return findings;
}

// A copy of `env` without the variables git lists as local to a repository
// (GIT_DIR, GIT_INDEX_FILE, GIT_WORK_TREE and the rest). A git hook exports
// some of them, and a child git that inherits them reads and writes the
// hook's repository instead of the one its working directory is in.
export function gitEnv(env = process.env) {
  const res = spawnSync('git', ['rev-parse', '--local-env-vars'], { encoding: 'utf8' });
  const local = new Set((res.stdout || '').split('\n').filter(Boolean));
  return Object.fromEntries(Object.entries(env).filter(([name]) => !local.has(name)));
}

function listTracked(root) {
  const res = spawnSync('git', ['ls-files', '-z', '--', '*.md', '*.mdx'], {
    cwd: root,
    env: gitEnv(),
    encoding: 'utf8',
    maxBuffer: 64 * 1024 * 1024,
  });
  if (res.status !== 0) {
    throw new Error(`git ls-files failed in ${root}: ${(res.stderr || '').trim()}`);
  }
  return res.stdout.split('\0').filter(Boolean);
}

export function run(root) {
  const files = listTracked(root).filter(inScope);
  const findings = [];
  for (const rel of files) {
    const text = fs.readFileSync(path.join(root, rel), 'utf8');
    for (const f of scanMarkdown(text)) findings.push({ path: rel, ...f });
  }
  return { files: files.length, findings };
}

function main(argv) {
  let root = path.resolve(HERE, '..');
  for (let a = 0; a < argv.length; a += 1) {
    if (argv[a] === '--root' && argv[a + 1]) {
      root = path.resolve(argv[a + 1]);
      a += 1;
    } else {
      console.error(`check-docs-prose: unknown argument ${argv[a]}`);
      return 2;
    }
  }

  let result;
  try {
    result = run(root);
  } catch (err) {
    console.error(`check-docs-prose: ${err.message}`);
    return 2;
  }

  for (const f of result.findings) console.log(`${f.path}:${f.line}: ${f.rule}`);
  if (result.findings.length > 0) {
    console.error(
      `check-docs-prose: ${result.findings.length} finding(s) in ${result.files} file(s). ` +
        'Rewrite the dash as a period, comma, colon or parentheses; fold a "Why this" section into the prose above it.',
    );
    return 1;
  }
  console.log(`check-docs-prose: ${result.files} file(s) clean`);
  return 0;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  process.exitCode = main(process.argv.slice(2));
}
