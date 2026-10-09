#!/usr/bin/env node
// Fails when a tracked file carries a token-shaped literal: a three-segment
// base64url token (the JWT shape), a private key block, a vendor credential
// with a recognisable prefix, an AIM API key, or a credential-named field
// assigned a high-entropy string.
//
// A bearer token that sits in a test file is still a bearer token. The
// dependency and container scanners read manifests and images, and the
// configuration scan reads config files, so nothing read the tracked tree
// line by line for this shape. This script does, over every path
// `git ls-files` returns, binary files included (a compiled-in literal is as
// usable as one in source).
//
// It fails closed. Every outcome other than "every tracked file was read,
// the planted control was caught, and nothing else was found" exits non-zero:
//
//   0  clean
//   1  one or more findings, an allowlist entry that matches nothing, or an
//      allowlist that does not parse
//   2  inconclusive: the tree could not be enumerated, a tracked path could
//      not be read, or the planted control was not caught
//
// Scanning is line by line: a token whose characters are split across lines,
// for example by string concatenation, is not seen.
//
// The planted control: before the verdict, the run writes a synthetic file
// into a fresh temporary directory carrying one sample per rule, assembled at
// run time so that no sample exists as a literal in this repository, and
// scans it through the same reader, rules and allowlist filter as the tracked
// files. Each sample must be caught by the rule it was written for. A run in
// which any sample goes uncaught proves nothing about the tree, and reads
// INCONCLUSIVE rather than clean.
//
// Findings print path, line, rule and a fingerprint (the first 16 hex digits
// of the SHA-256 of the matched text). The matched text itself is never
// printed: CI logs of a public repository are public.
//
// Allowlist: scripts/token-literal-allowlist.txt. One entry per line,
//   <rule> <fingerprint> <path>   # why this literal is not a credential
// An entry suppresses that literal at that path only. An entry that matches
// nothing fails the run, so the list cannot outlive what it describes. Prefer
// building a test token at run time over adding an entry.
//
// Node built-ins only, so no install step has to run before it.
//
// Usage:
//   node scripts/lint-token-literals.mjs [--root <dir>] [--allowlist <file>]

import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const DEFAULT_ALLOWLIST = path.join(HERE, 'token-literal-allowlist.txt');

// A base64url run not glued to another base64url character.
const B64 = '[A-Za-z0-9_-]';
const NB = `(?<!${B64})`;
const NA = `(?!${B64})`;

// Field names that hold a credential. Matched case-insensitively, with any
// prefix (`adminPassword`, `AIM_API_KEY`), so the right-hand value carries the
// precision: see isHighEntropySecret.
const CREDENTIAL_FIELD =
  '(?:password|passwd|passphrase|secret|api[_-]?key|access[_-]?key|private[_-]?key|' +
  'client[_-]?secret|signing[_-]?key|token)';

export const RULES = [
  {
    id: 'jwt',
    description: 'three-segment base64url token with JSON header and payload (JWT/JWS)',
    // `{"`, `{ ` and `{\n` all base64url-encode to `ey` or `ew`, so a JSON
    // header and payload start with one of those. The signature may be empty
    // (an unsigned token is still accepted by a verifier that skips checks).
    pattern: new RegExp(`${NB}e[wy]${B64}{10,}\\.e[wy]${B64}{10,}\\.${B64}*`, 'g'),
  },
  {
    id: 'base64url-three-segment',
    description: 'three long base64url segments joined by dots (opaque signed token)',
    pattern: new RegExp(`${NB}${B64}{20,}\\.${B64}{20,}\\.${B64}{20,}${NA}`, 'g'),
  },
  {
    id: 'private-key-block',
    description: 'PEM private key block',
    pattern: /-----BEGIN (?:[A-Z0-9]+ )*PRIVATE KEY(?: BLOCK)?-----/g,
  },
  {
    id: 'aim-api-key',
    description: 'AIM API key (aim_live_ followed by 32 random bytes, base64url)',
    pattern: new RegExp(`aim_live_${B64}{43}=?${NA}`, 'g'),
  },
  {
    id: 'aws-access-key-id',
    description: 'AWS access key id',
    pattern: /(?<![A-Z0-9])(?:AKIA|ASIA|ABIA|ACCA)[A-Z0-9]{16}(?![A-Z0-9])/g,
  },
  {
    id: 'github-token',
    description: 'GitHub token',
    pattern: /(?<![A-Za-z0-9_])(?:gh[pousr]_[A-Za-z0-9]{36,}|github_pat_[A-Za-z0-9_]{22,})/g,
  },
  {
    id: 'slack-token',
    description: 'Slack token',
    pattern: /(?<![A-Za-z0-9])xox[abposr]-[A-Za-z0-9-]{10,}/g,
  },
  {
    id: 'sk-api-key',
    description: 'sk- prefixed API key (model providers and similar)',
    pattern: new RegExp(`${NB}sk-(?:ant-|proj-)?${B64}{20,}`, 'g'),
  },
  {
    id: 'stripe-live-key',
    description: 'Stripe live secret or restricted key',
    pattern: /(?<![A-Za-z0-9])[rs]k_live_[A-Za-z0-9]{16,}/g,
  },
  {
    id: 'google-api-key',
    description: 'Google API key',
    pattern: new RegExp(`${NB}AIza${B64}{35}${NA}`, 'g'),
  },
  {
    id: 'credential-assignment',
    description: 'credential-named field assigned a high-entropy string literal',
    pattern: new RegExp(
      `${CREDENTIAL_FIELD}["']?\\s*(?::=|=>|[:=])\\s*["'\`]([^"'\`\\s]{16,})["'\`]`,
      'gi',
    ),
    // The field name alone is not enough: tests assign `password: "Test123!"`
    // and docs assign `api_key="your-api-key"` everywhere. A value counts
    // when it looks generated.
    accept: (match) => isHighEntropySecret(match[1]),
  },
];

// A generated secret: long, several character classes, no placeholder or
// interpolation markers, and high per-character entropy.
export function isHighEntropySecret(value) {
  if (value.length < 16) return false;
  if (/\.\.\.|<|>|\$\{|\$\(|\{\{|%[sdv]|xxxx|XXXX/.test(value)) return false;
  let classes = 0;
  if (/[a-z]/.test(value)) classes += 1;
  if (/[A-Z]/.test(value)) classes += 1;
  if (/[0-9]/.test(value)) classes += 1;
  if (/[^A-Za-z0-9]/.test(value)) classes += 1;
  if (classes < 3) return false;
  return shannonEntropy(value) >= 4.0;
}

export function shannonEntropy(value) {
  const counts = new Map();
  for (const ch of value) counts.set(ch, (counts.get(ch) || 0) + 1);
  let bits = 0;
  for (const n of counts.values()) {
    const p = n / value.length;
    bits -= p * Math.log2(p);
  }
  return bits;
}

export function fingerprint(text) {
  return createHash('sha256').update(text, 'utf8').digest('hex').slice(0, 16);
}

// Scans one file's bytes. Latin-1 maps every byte to one code unit, so binary
// content is read without a decode error and ASCII literals inside it match.
export function scanText(relPath, text, rules = RULES) {
  const findings = [];
  const lines = text.split('\n');
  for (let i = 0; i < lines.length; i += 1) {
    const line = lines[i];
    for (const rule of rules) {
      rule.pattern.lastIndex = 0;
      let match;
      while ((match = rule.pattern.exec(line)) !== null) {
        if (match[0].length === 0) {
          rule.pattern.lastIndex += 1;
          continue;
        }
        if (rule.accept && !rule.accept(match)) continue;
        findings.push({
          path: relPath,
          line: i + 1,
          rule: rule.id,
          fingerprint: fingerprint(match[0]),
        });
      }
    }
  }
  return findings;
}

function readTracked(root, relPath) {
  const abs = path.join(root, relPath);
  const st = fs.lstatSync(abs);
  if (st.isSymbolicLink()) return fs.readlinkSync(abs);
  return fs.readFileSync(abs).toString('latin1');
}

export function parseAllowlist(text) {
  const entries = [];
  const errors = [];
  text.split('\n').forEach((raw, i) => {
    const line = raw.replace(/#.*$/, '').trim();
    if (line === '') return;
    const parts = line.split(/\s+/);
    if (parts.length !== 3 || !/^[0-9a-f]{16}$/.test(parts[1])) {
      errors.push(`allowlist line ${i + 1}: expected "<rule> <16-hex fingerprint> <path>"`);
      return;
    }
    if (!/#\s*\S/.test(raw)) {
      errors.push(`allowlist line ${i + 1}: an entry needs a "# reason" comment`);
      return;
    }
    const [rule, fp, p] = parts;
    entries.push({ rule, fingerprint: fp, path: p, line: i + 1, used: false });
  });
  return { entries, errors };
}

function allowed(entries, finding) {
  let hit = false;
  for (const e of entries) {
    if (e.rule === finding.rule && e.fingerprint === finding.fingerprint && e.path === finding.path) {
      e.used = true;
      hit = true;
    }
  }
  return hit;
}

// One sample per rule, assembled from parts so this file never holds one
// whole. Each sample's line is attributed to the rule that must catch it.
export function plantedSamples() {
  const b64url = (s) => Buffer.from(s, 'utf8').toString('base64url');
  const rnd = (n) => createHash('sha256').update(`planted-control-${n}`).digest('base64url');
  const upper = (n) => createHash('sha256').update(`planted-upper-${n}`).digest('hex').toUpperCase();
  return [
    {
      rule: 'jwt',
      line: `const token = "${b64url('{"alg":"HS256","typ":"JWT"}')}.${b64url(
        '{"sub":"planted-control","role":"admin"}',
      )}.${rnd(1).slice(0, 43)}";`,
    },
    {
      rule: 'base64url-three-segment',
      line: `opaque = "${rnd(2).slice(0, 24)}.${rnd(3).slice(0, 24)}.${rnd(4).slice(0, 24)}"`,
    },
    { rule: 'private-key-block', line: ['-----BEGIN', 'PRIVATE', 'KEY-----'].join(' ') },
    { rule: 'aim-api-key', line: `AIM_KEY=${'aim_' + 'live_'}${Buffer.alloc(32, 7).toString('base64url')}` },
    { rule: 'aws-access-key-id', line: `aws_id = ${'AK' + 'IA'}${upper(5).slice(0, 16)}` },
    { rule: 'github-token', line: `gh = ${'gh' + 'p_'}${rnd(6).replace(/[-_]/g, 'a').slice(0, 36)}` },
    { rule: 'slack-token', line: `slack = ${'xo' + 'xb'}-${upper(7).slice(0, 12)}-${rnd(8).replace(/[-_]/g, 'b').slice(0, 24)}` },
    { rule: 'sk-api-key', line: `key = ${'s' + 'k-'}${rnd(9).slice(0, 40)}` },
    { rule: 'stripe-live-key', line: `stripe = ${'sk' + '_li' + 've_'}${rnd(10).replace(/[-_]/g, 'c').slice(0, 24)}` },
    { rule: 'google-api-key', line: `g = ${'AI' + 'za'}${rnd(11).slice(0, 35)}` },
    { rule: 'credential-assignment', line: `db_password = "${rnd(12).slice(0, 14)}!${upper(13).slice(0, 6)}9"` },
  ];
}

// Writes the planted control into a fresh temporary directory, scans it with
// the same rules and allowlist filter, and returns the samples that were not
// caught by the rule they were written for.
function runPlantedControl(rules, entries) {
  const samples = plantedSamples();
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'token-literal-control-'));
  const file = path.join(dir, 'planted-control.txt');
  try {
    fs.writeFileSync(file, samples.map((s) => s.line).join('\n') + '\n');
    const text = fs.readFileSync(file).toString('latin1');
    const relPath = `<planted>/${path.basename(file)}`;
    const found = scanText(relPath, text, rules).filter((f) => !allowed(entries, f));
    const missed = samples
      .map((s, i) => ({ ...s, lineNo: i + 1 }))
      .filter((s) => !found.some((f) => f.line === s.lineNo && f.rule === s.rule));
    return { planted: samples.length, missed };
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

export function run({ root, allowlistPath = DEFAULT_ALLOWLIST, rules = RULES, out = console.log, err = console.error }) {
  const ls = spawnSync('git', ['-C', root, 'ls-files', '-z'], { encoding: 'buffer', maxBuffer: 1 << 28 });
  if (ls.error || ls.status !== 0) {
    err(`token-literals: INCONCLUSIVE: git ls-files failed in ${root}`);
    if (ls.stderr && ls.stderr.length) err(ls.stderr.toString().trim());
    return 2;
  }
  const files = ls.stdout.toString('utf8').split('\0').filter(Boolean);
  if (files.length === 0) {
    err(`token-literals: INCONCLUSIVE: git ls-files returned no tracked files in ${root}`);
    return 2;
  }

  let entries = [];
  if (fs.existsSync(allowlistPath)) {
    const parsed = parseAllowlist(fs.readFileSync(allowlistPath, 'utf8'));
    if (parsed.errors.length) {
      for (const e of parsed.errors) err(`token-literals: ${e}`);
      return 1;
    }
    entries = parsed.entries;
  }

  const unreadable = [];
  const findings = [];
  let read = 0;
  let bytes = 0;
  for (const rel of files) {
    let text;
    try {
      text = readTracked(root, rel);
    } catch (e) {
      unreadable.push(`${rel}: ${e.code || e.message}`);
      continue;
    }
    read += 1;
    bytes += text.length;
    for (const f of scanText(rel, text, rules)) {
      if (!allowed(entries, f)) findings.push(f);
    }
  }

  const control = runPlantedControl(rules, entries);

  for (const f of findings) {
    err(`${f.path}:${f.line}: ${f.rule} (fingerprint ${f.fingerprint})`);
  }
  const stale = entries.filter((e) => !e.used);
  for (const e of stale) {
    err(`token-literals: allowlist line ${e.line} matches nothing (${e.rule} ${e.fingerprint} ${e.path}); remove it`);
  }

  if (unreadable.length) {
    for (const u of unreadable) err(`token-literals: cannot read tracked path ${u}`);
    err(`token-literals: INCONCLUSIVE: ${unreadable.length} of ${files.length} tracked paths could not be read`);
    return 2;
  }
  if (control.missed.length) {
    for (const m of control.missed) err(`token-literals: planted ${m.rule} sample was not caught`);
    err(`token-literals: INCONCLUSIVE: planted control caught ${control.planted - control.missed.length} of ${control.planted} samples, so a clean result would prove nothing`);
    return 2;
  }
  if (findings.length || stale.length) {
    if (findings.length) {
      err(
        `token-literals: ${findings.length} token-shaped literal(s) in tracked files. ` +
          'Build test tokens at run time, read real ones from the environment, and rotate any that were real.',
      );
    }
    return 1;
  }
  out(
    `token-literals: scanned ${read} tracked files (${bytes} bytes); planted control caught ${control.planted} of ${control.planted} samples; ` +
      `0 findings (${entries.length} allowlisted)`,
  );
  return 0;
}

function main(argv) {
  let root = null;
  let allowlistPath = DEFAULT_ALLOWLIST;
  for (let i = 0; i < argv.length; i += 1) {
    if (argv[i] === '--root' && argv[i + 1]) root = argv[++i];
    else if (argv[i] === '--allowlist' && argv[i + 1]) allowlistPath = argv[++i];
    else {
      console.error(`usage: node scripts/lint-token-literals.mjs [--root <dir>] [--allowlist <file>]`);
      return 2;
    }
  }
  if (root === null) {
    const top = spawnSync('git', ['-C', HERE, 'rev-parse', '--show-toplevel'], { encoding: 'utf8' });
    if (top.status !== 0) {
      console.error('token-literals: INCONCLUSIVE: not inside a git work tree');
      return 2;
    }
    root = top.stdout.trim();
  }
  return run({ root: path.resolve(root), allowlistPath });
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
