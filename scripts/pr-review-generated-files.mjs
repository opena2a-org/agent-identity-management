#!/usr/bin/env node
// Composes the automated reviewer's input for .github/workflows/pr-review.yml.
//
// The gate hands the pull request diff to a model with a fixed context window.
// A generated dependency lockfile is the one kind of change that is routinely
// larger than that window and carries nothing a reader can verify by eye, so a
// lockfile pull request was INCONCLUSIVE by construction: the required check
// could not pass, and a reviewer scrolling 380,000 bytes of resolved/integrity
// pairs would not have caught a swapped host either.
//
// This script is the gate's input, split out of the workflow's inline shell so
// that its properties are testable without executing a YAML `run:` block. It
// does three things, in one pass, and writes nothing unless all three succeed:
//
//   1. Recognises generated lockfile paths by a path-shaped predicate (final
//      path segment, at any depth, or the `.lock` suffix the workflow already
//      excluded) and drops them from the full-file context list. The previous
//      `case` pattern was basename-shaped and matched only a root lockfile.
//   2. Replaces the diff section of every npm-family lockfile (package-lock.json,
//      npm-shrinkwrap.json: the only kind an instrument exists for) with a
//      placeholder carrying the file's digest at the pull request head, the
//      elided counts, the entry delta against the base tree, and the verdict
//      plus check lines of a deterministic predicate run over the checked-out
//      tree. Every other section is preserved byte for byte: a generated section
//      of another kind is kept whole and named in the report's `retained` array,
//      and a section the script cannot classify (quoted path, no content lines,
//      an unmeasured header shape) is kept whole and named under `unclassified`,
//      so nothing is ever folded into a neighbour's placeholder and nothing is
//      silently shortened.
//   3. Writes a JSON report of what it did, and, invoked with --enforce-report,
//      reads that report back and fails on any red predicate. The job's
//      conclusion on a red predicate comes from this record, never from the
//      reviewer's reading of a text payload.
//
// Node built-ins only: the workflow runs this with the runner's node and no
// install step precedes it (an install would execute pull-request-controlled
// scripts before the gate has looked at anything).
//
// Usage:
//   node scripts/pr-review-generated-files.mjs --tree <dir> [--base-tree <dir>]
//     --changed-files <in> --diff <in>
//     --out-diff <out> --out-context-files <out> --out-report <out>
//   node scripts/pr-review-generated-files.mjs --enforce-report <file>
//
// Exit status: 0 on success (a FAIL verdict is still a successful run of the
// composer; the enforce mode turns it into exit 1), 2 on a missing
// precondition or a usage error, with one line on standard error beginning
// `pr-review-generated-files: ` and no output file created or modified.

import { createHash } from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';

const PROG = 'pr-review-generated-files';
const INSTRUMENT_PATH = 'scripts/pr-review-generated-files.mjs';

// The eleven basenames, compared byte for byte against the final path segment.
const GENERATED_BASENAMES = [
  'package-lock.json',
  'npm-shrinkwrap.json',
  'yarn.lock',
  'pnpm-lock.yaml',
  'go.sum',
  'Cargo.lock',
  'poetry.lock',
  'Pipfile.lock',
  'uv.lock',
  'composer.lock',
  'Gemfile.lock',
];
// The glob the workflow excluded before this script existed. Kept so that no
// path excluded at base (mix.lock, flake.lock, Podfile.lock, ...) re-enters
// the full-file context.
const LOCK_SUFFIX = '.lock';
// The kinds an instrument exists for. Every other generated kind is retained
// whole and recorded, never substituted: a placeholder that only says
// "unevaluated" is a truncation notice, and that is what this gate refuses.
const NPM_BASENAMES = new Set(['package-lock.json', 'npm-shrinkwrap.json']);

const DELTA_FIELDS = [
  'version',
  'resolved',
  'integrity',
  'link',
  'dependencies',
  'devDependencies',
  'optionalDependencies',
  'peerDependencies',
];
const REGISTRY_PREFIX = 'https://registry.npmjs.org/';
const INTEGRITY_SHAPE = /^sha512-[A-Za-z0-9+/]+={0,2}$/;
const EXACT_PIN = /^[0-9]+\.[0-9]+\.[0-9]+$/;

class InstrumentError extends Error {
  constructor(message, exitCode = 2) {
    super(message);
    this.exitCode = exitCode;
  }
}

// ---------------------------------------------------------------------------
// Bytes. The diff and the changed-files list are handled as latin1 strings: one
// character per byte, so slicing and searching never alter a section and the
// preserved sections are written back byte-identical. Strings that come from
// parsed JSON are converted to that form before they are placed in the diff.
// ---------------------------------------------------------------------------

const toLatin1 = (s) => Buffer.from(s, 'utf8').toString('latin1');
const fromLatin1 = (s) => Buffer.from(s, 'latin1').toString('utf8');
const isObject = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);
const has = (o, k) => Object.prototype.hasOwnProperty.call(o, k);
const sha256Hex = (buf) => createHash('sha256').update(buf).digest('hex');
const oneLine = (s) => String(s).replace(/[\r\n]+/g, ' ');

function byteCompare(a, b) {
  return Buffer.compare(Buffer.from(a, 'utf8'), Buffer.from(b, 'utf8'));
}

// JSON's escape for every byte below 0x20 and for `\`, so a rendered token is
// one line and no lockfile string can open or close a placeholder block.
function escapeToken(s) {
  return String(s).replace(/[\u0000-\u001f\\]/g, (c) => {
    switch (c) {
      case '\n': return '\\n';
      case '\r': return '\\r';
      case '\t': return '\\t';
      case '\b': return '\\b';
      case '\f': return '\\f';
      case '\\': return '\\\\';
      default: return `\\u${c.charCodeAt(0).toString(16).padStart(4, '0')}`;
    }
  });
}

function deepEqual(a, b) {
  if (a === b) return true;
  if (Array.isArray(a) || Array.isArray(b)) {
    if (!Array.isArray(a) || !Array.isArray(b) || a.length !== b.length) return false;
    return a.every((v, i) => deepEqual(v, b[i]));
  }
  if (isObject(a) && isObject(b)) {
    const ka = Object.keys(a);
    const kb = Object.keys(b);
    if (ka.length !== kb.length) return false;
    return ka.every((k) => has(b, k) && deepEqual(a[k], b[k]));
  }
  return false;
}

// A tree path is joined as bytes: fs accepts Buffer paths, and a diff path is
// never decoded on the way to the filesystem.
function treeFile(treeDir, relLatin1) {
  return Buffer.concat([
    Buffer.from(path.resolve(treeDir)),
    Buffer.from(path.sep),
    Buffer.from(relLatin1, 'latin1'),
  ]);
}

function fileExists(bufPath) {
  try {
    return fs.statSync(bufPath).isFile();
  } catch {
    return false;
  }
}

function readJson(bufPath, describe) {
  let text;
  try {
    text = fs.readFileSync(bufPath).toString('utf8');
  } catch (e) {
    throw new InstrumentError(`${describe} cannot be read: ${e.code ?? oneLine(e.message)}`);
  }
  let value;
  try {
    value = JSON.parse(text);
  } catch (e) {
    throw new InstrumentError(`${describe} is not parseable JSON: ${oneLine(e.message)}`);
  }
  if (!isObject(value)) throw new InstrumentError(`${describe} is not a JSON object`);
  return value;
}

// Lines with their terminators kept, so that a count and a re-join are exact.
function splitLines(text) {
  const out = [];
  let start = 0;
  while (start < text.length) {
    const nl = text.indexOf('\n', start);
    if (nl < 0) {
      out.push(text.slice(start));
      break;
    }
    out.push(text.slice(start, nl + 1));
    start = nl + 1;
  }
  return out;
}

const countLines = (text) => splitLines(text).length;

// ---------------------------------------------------------------------------
// Path predicate.
// ---------------------------------------------------------------------------

function finalSegment(p) {
  const i = p.lastIndexOf('/');
  return i < 0 ? p : p.slice(i + 1);
}

// Returns the kind label of a generated path, or null. A path is generated
// exactly when its final segment is one of the eleven names, at any depth, or
// it ends with the five bytes `.lock`.
function generatedKind(p) {
  const seg = finalSegment(p);
  if (GENERATED_BASENAMES.includes(seg)) return seg;
  if (p.endsWith(LOCK_SUFFIX)) return '*.lock';
  return null;
}

const isNpmFamily = (p) => NPM_BASENAMES.has(finalSegment(p));

// ---------------------------------------------------------------------------
// Diff sections.
// ---------------------------------------------------------------------------

// A section is the text from a line beginning `diff --git ` up to but excluding
// the next such line or the end of the input. Text before the first section is
// a preamble and is preserved as it is.
function splitSections(diff) {
  const starts = [];
  if (diff.startsWith('diff --git ')) starts.push(0);
  let i = 0;
  for (;;) {
    const j = diff.indexOf('\ndiff --git ', i);
    if (j < 0) break;
    starts.push(j + 1);
    i = j + 1;
  }
  const preamble = diff.slice(0, starts.length ? starts[0] : diff.length);
  const sections = starts.map((s, k) =>
    diff.slice(s, k + 1 < starts.length ? starts[k + 1] : diff.length),
  );
  return { preamble, sections };
}

const stripOneTab = (s) => (s.endsWith('\t') ? s.slice(0, -1) : s);

// Reads a section's path and status from the header lines that precede its
// first `@@` line, never from the `diff --git` operands: git leaves a path
// containing a space unquoted there and appends a tab to the `+++ ` line, so an
// operand split on spaces would misname the file.
function classifySection(text) {
  const header = [];
  for (const line of splitLines(text)) {
    if (line.startsWith('@@')) break;
    header.push(line.endsWith('\n') ? line.slice(0, -1) : line);
  }
  const headerLine = header[0] ?? '';
  const minus = header.find((l) => l.startsWith('--- '));
  const plus = header.find((l) => l.startsWith('+++ '));
  const renameFrom = header.find((l) => l.startsWith('rename from '));
  const lines = countLines(text);
  const bytes = text.length;
  const unclassified = (reason) => ({ kind: 'unclassified', headerLine, reason, lines, bytes });

  const quoted = (l) => l !== undefined && l.slice(4).startsWith('"');
  if (quoted(plus) || quoted(minus)) return unclassified('quoted path');
  if (renameFrom !== undefined && renameFrom.slice('rename from '.length).startsWith('"')) {
    return unclassified('quoted path');
  }
  if (plus === undefined) return unclassified('no content lines');

  const newFile = header.some((l) => l.startsWith('new file mode'));
  const deletedFile = header.some((l) => l.startsWith('deleted file mode'));
  let filePath;
  if (plus.startsWith('+++ b/')) {
    filePath = stripOneTab(plus.slice('+++ b/'.length));
  } else if (plus === '+++ /dev/null') {
    if (minus === undefined || !minus.startsWith('--- a/') || !deletedFile) {
      return unclassified('unrecognised header');
    }
    filePath = stripOneTab(minus.slice('--- a/'.length));
  } else {
    return unclassified('unrecognised header');
  }
  if (minus === undefined) return unclassified('unrecognised header');
  if (minus === '--- /dev/null') {
    if (!newFile) return unclassified('unrecognised header');
  } else if (!minus.startsWith('--- a/')) {
    return unclassified('unrecognised header');
  }
  if (filePath === '' || filePath.startsWith('/') || filePath.split('/').includes('..')) {
    return unclassified('unrecognised header');
  }

  let status = 'modified';
  let oldPath = null;
  if (deletedFile) status = 'deleted';
  else if (newFile) status = 'added';
  else if (renameFrom !== undefined) {
    status = 'renamed';
    oldPath = renameFrom.slice('rename from '.length);
  }
  return { kind: 'section', headerLine, path: filePath, status, oldPath, lines, bytes };
}

// ---------------------------------------------------------------------------
// The npm-lockfile predicate: six checks over a lockfile at path P in directory
// D of the tree, rendered as `check <name>: PASS`, `check <name>: FAIL <key>` or
// `check <name>: not applicable: <reason>`. The verdict is PASS exactly when no
// check line reads FAIL.
// ---------------------------------------------------------------------------

const mapOf = (obj, key) => (isObject(obj) && isObject(obj[key]) ? obj[key] : {});
const scalarKey = (v) => (typeof v === 'string' ? v : JSON.stringify(v));

// First differing name between two maps, over the union of their keys in byte
// order; null when they are equal key for key and value for value.
function firstMapDifference(a, b) {
  const keys = [...new Set([...Object.keys(a), ...Object.keys(b)])].sort(byteCompare);
  for (const k of keys) {
    if (!has(a, k) || !has(b, k) || !deepEqual(a[k], b[k])) return k;
  }
  return null;
}

function globToRegExp(glob) {
  let g = String(glob);
  if (g.startsWith('./')) g = g.slice(2);
  if (g.endsWith('/')) g = g.slice(0, -1);
  const parts = g.split('*').map((s) => s.replace(/[.+?^${}()|[\]\\]/g, '\\$&'));
  return new RegExp(`^${parts.join('[^/]*')}$`);
}

const entriesOf = (packages) =>
  Object.entries(packages).filter(([p]) => p !== '').map(([p, e]) => [p, isObject(e) ? e : {}]);
const isLink = (e) => e.link === true;
const packageName = (entryPath) => {
  const i = entryPath.lastIndexOf('node_modules/');
  return i < 0 ? entryPath : entryPath.slice(i + 'node_modules/'.length);
};

function evaluateNpmLockfile({ treeDir, lockPath, lock }) {
  const dir = lockPath.includes('/') ? lockPath.slice(0, lockPath.lastIndexOf('/')) : '';
  const manifestPath = dir === '' ? 'package.json' : `${dir}/package.json`;
  const manifestFile = treeFile(treeDir, manifestPath);
  if (!fileExists(manifestFile)) {
    throw new InstrumentError(
      `${fromLatin1(lockPath)}: manifest ${fromLatin1(manifestPath)} is absent under --tree`,
    );
  }
  const manifest = readJson(manifestFile, `${fromLatin1(lockPath)}: manifest ${fromLatin1(manifestPath)}`);
  const packages = mapOf(lock, 'packages');
  const rootEntry = mapOf(packages, '');
  const manifestDeps = mapOf(manifest, 'dependencies');
  const manifestDev = mapOf(manifest, 'devDependencies');
  const checks = [];
  const pass = (name, detail = '') => checks.push(`check ${name}: PASS${detail}`);
  const fail = (name, key) => checks.push(`check ${name}: FAIL ${escapeToken(key)}`);
  const na = (name, reason) => checks.push(`check ${name}: not applicable: ${reason}`);

  // (1) lockfileVersion
  if (lock.lockfileVersion === 3) pass('lockfileVersion');
  else fail('lockfileVersion', has(lock, 'lockfileVersion') ? scalarKey(lock.lockfileVersion) : 'absent');

  // (2) manifest-root-entry
  {
    const d =
      firstMapDifference(manifestDeps, mapOf(rootEntry, 'dependencies')) ??
      firstMapDifference(manifestDev, mapOf(rootEntry, 'devDependencies'));
    if (d === null) pass('manifest-root-entry');
    else fail('manifest-root-entry', d);
  }

  // (3) exact-pins
  {
    let pins = 0;
    let failure = null;
    for (const name of Object.keys(manifestDeps).sort(byteCompare)) {
      const want = manifestDeps[name];
      if (typeof want !== 'string' || !EXACT_PIN.test(want)) continue;
      pins += 1;
      const entry = packages[`node_modules/${name}`];
      const got = isObject(entry) && has(entry, 'version') ? scalarKey(entry.version) : 'absent';
      if (!isObject(entry) || entry.version !== want) {
        failure = `${name} ${want} != ${got}`;
        break;
      }
    }
    if (failure === null) pass('exact-pins', ` ${pins} pins`);
    else fail('exact-pins', failure);
  }

  // (4) resolved-hosts and (5) integrity-shape, over every entry other than ""
  // and other than link entries.
  const registryEntries = entriesOf(packages).filter(([, e]) => !isLink(e));
  {
    const bad = registryEntries.find(
      ([, e]) => has(e, 'resolved') && !(typeof e.resolved === 'string' && e.resolved.startsWith(REGISTRY_PREFIX)),
    );
    if (bad === undefined) pass('resolved-hosts');
    else fail('resolved-hosts', `${bad[0]} ${scalarKey(bad[1].resolved)}`);
  }
  {
    const bad = registryEntries.find(
      ([, e]) => has(e, 'integrity') && !(typeof e.integrity === 'string' && INTEGRITY_SHAPE.test(e.integrity)),
    );
    if (bad === undefined) pass('integrity-shape');
    else fail('integrity-shape', bad[0]);
  }

  // (6) root-parity
  if (dir === '') {
    na('root-parity', 'root lockfile');
  } else {
    const rootFile = treeFile(treeDir, 'package-lock.json');
    let root = null;
    if (fileExists(rootFile)) {
      root = readJson(rootFile, `${fromLatin1(lockPath)}: root package-lock.json (root-parity applies)`);
    }
    const workspaces = root === null ? null : mapOf(root, 'packages')['']?.workspaces;
    const member =
      Array.isArray(workspaces) &&
      workspaces.some((g) => typeof g === 'string' && globToRegExp(g).test(fromLatin1(dir)));
    if (!member) {
      na('root-parity', 'standalone lockfile');
    } else {
      const rootPackages = mapOf(root, 'packages');
      const rootWorkspaceEntry = mapOf(rootPackages, fromLatin1(dir));
      let failure =
        firstMapDifference(manifestDeps, mapOf(rootWorkspaceEntry, 'dependencies')) ??
        firstMapDifference(manifestDev, mapOf(rootWorkspaceEntry, 'devDependencies'));
      if (failure === null) {
        const rootEntries = entriesOf(rootPackages);
        for (const [entryPath, e] of registryEntries) {
          const name = packageName(entryPath);
          const match = rootEntries.some(
            ([rp, r]) =>
              packageName(rp) === name && r.version === e.version && r.integrity === e.integrity,
          );
          if (!match) {
            failure = entryPath;
            break;
          }
        }
      }
      if (failure === null) pass('root-parity');
      else fail('root-parity', failure);
    }
  }

  const verdict = checks.some((c) => /^check [^:]+: FAIL /.test(c)) ? 'FAIL' : 'PASS';
  return { checks, verdict };
}

// ---------------------------------------------------------------------------
// Entry delta between the base and the head copy of a lockfile.
// ---------------------------------------------------------------------------

function versionToken(entry) {
  if (isLink(entry)) return `link ${escapeToken(has(entry, 'resolved') ? scalarKey(entry.resolved) : '-')}`;
  if (!has(entry, 'version')) return '-';
  return escapeToken(scalarKey(entry.version));
}

function computeDelta(basePackages, headPackages) {
  const base = Object.fromEntries(entriesOf(basePackages));
  const head = Object.fromEntries(entriesOf(headPackages));
  const added = Object.keys(head).filter((k) => !has(base, k)).sort(byteCompare);
  const removed = Object.keys(base).filter((k) => !has(head, k)).sort(byteCompare);
  const changed = Object.keys(head)
    .filter((k) => has(base, k))
    .map((k) => ({ k, fields: DELTA_FIELDS.filter((f) => !deepEqual(base[k][f], head[k][f])) }))
    .filter(({ fields }) => fields.length > 0)
    .sort((a, b) => byteCompare(a.k, b.k));
  const lines = [
    ...added.map((k) => `+ ${escapeToken(k)} ${versionToken(head[k])}`),
    ...removed.map((k) => `- ${escapeToken(k)} ${versionToken(base[k])}`),
    ...changed.map(({ k, fields }) => {
      const before = versionToken(base[k]);
      const after = versionToken(head[k]);
      return before !== after
        ? `~ ${escapeToken(k)} ${before} -> ${after}`
        : `~ ${escapeToken(k)} ${before} (${fields.join(', ')})`;
    }),
  ];
  return {
    added: added.length,
    removed: removed.length,
    changed: changed.length,
    header: `delta: ${added.length} added, ${removed.length} removed, ${changed.length} changed`,
    lines,
  };
}

// ---------------------------------------------------------------------------
// Placeholder.
// ---------------------------------------------------------------------------

function renderPlaceholder(sub) {
  const out = [];
  out.push(`=== GENERATED FILE PLACEHOLDER: ${sub.pathLatin1} ===`);
  out.push(`status: ${sub.statusLatin1}`);
  out.push(`sha256: ${sub.sha256}`);
  out.push(`elided: ${sub.elided.lines} diff lines, ${sub.elided.bytes} bytes`);
  out.push(toLatin1(sub.delta.header));
  for (const line of sub.delta.lines) out.push(toLatin1(line));
  out.push(
    `instrument: ${INSTRUMENT_PATH} npm-lockfile predicate over ${sub.pathLatin1} in the checked-out tree`,
  );
  out.push(`verdict: ${sub.verdict}`);
  for (const line of sub.checks) out.push(toLatin1(line));
  out.push('=== END GENERATED FILE PLACEHOLDER ===');
  return `${out.join('\n')}\n`;
}

// Substitutes one npm-family section. Throws InstrumentError on a missing
// precondition; the caller writes nothing in that case.
function substituteNpmSection(section, treeDir, baseTreeDir) {
  const p = section.path;
  const display = fromLatin1(p);
  const basePathLatin1 = section.status === 'renamed' ? section.oldPath : p;
  const baseDisplay = fromLatin1(basePathLatin1);

  let headBytes = null;
  let head = null;
  if (section.status !== 'deleted') {
    const headFile = treeFile(treeDir, p);
    if (!fileExists(headFile)) {
      throw new InstrumentError(`${display}: status ${section.status} but the path does not exist under --tree`);
    }
    headBytes = fs.readFileSync(headFile);
    head = readJson(headFile, `${display}: head copy under --tree`);
  }

  let baseBytes = null;
  let base = null;
  const baseFile = baseTreeDir === null ? null : treeFile(baseTreeDir, basePathLatin1);
  const baseExists = baseFile !== null && fileExists(baseFile);
  if (section.status === 'added') {
    if (baseExists) {
      throw new InstrumentError(`${display}: status added but a file exists at that path under --base-tree`);
    }
  } else if (!baseExists) {
    throw new InstrumentError(
      `${display}: status ${section.status} but no file exists under --base-tree at ${baseDisplay}`,
    );
  } else {
    baseBytes = fs.readFileSync(baseFile);
    base = readJson(baseFile, `${display}: base copy at ${baseDisplay} under --base-tree`);
  }

  const delta = computeDelta(base === null ? {} : mapOf(base, 'packages'), head === null ? {} : mapOf(head, 'packages'));
  let verdict = 'DELETED';
  let checks = [];
  if (head !== null) ({ verdict, checks } = evaluateNpmLockfile({ treeDir, lockPath: p, lock: head }));

  const status = section.status === 'renamed' ? `renamed from ${fromLatin1(section.oldPath)}` : section.status;
  const sub = {
    path: display,
    kind: 'npm',
    status,
    sha256: headBytes === null ? 'absent' : sha256Hex(headBytes),
    baseSha256: baseBytes === null ? 'absent' : sha256Hex(baseBytes),
    verdict,
    elided: { lines: section.lines, bytes: section.bytes },
    delta: { added: delta.added, removed: delta.removed, changed: delta.changed, lines: delta.lines },
    checks,
  };
  const text = renderPlaceholder({
    pathLatin1: p,
    statusLatin1: section.status === 'renamed' ? `renamed from ${section.oldPath}` : section.status,
    sha256: sub.sha256,
    elided: sub.elided,
    delta,
    verdict,
    checks,
  });
  return { record: sub, text };
}

// ---------------------------------------------------------------------------
// Compose mode.
// ---------------------------------------------------------------------------

function compose(args) {
  const treeDir = args.tree;
  const baseTreeDir = args['base-tree'] ?? null;
  for (const [flag, dir] of [['--tree', treeDir], ['--base-tree', baseTreeDir]]) {
    if (dir !== null && !fs.existsSync(dir)) throw new InstrumentError(`${dir}: ${flag} directory does not exist`);
  }
  const readInput = (flag) => {
    try {
      return fs.readFileSync(args[flag]).toString('latin1');
    } catch (e) {
      throw new InstrumentError(`${args[flag]}: --${flag} cannot be read: ${e.code ?? oneLine(e.message)}`);
    }
  };
  const changed = readInput('changed-files');
  const diff = readInput('diff');

  // 1. The full-file context list: every non-generated line, in input order,
  //    byte for byte.
  const contextLines = [];
  const generated = [];
  for (const line of splitLines(changed)) {
    const p = line.endsWith('\n') ? line.slice(0, -1) : line;
    if (generatedKind(p) === null) contextLines.push(line);
    else generated.push(fromLatin1(p));
  }

  // 2. The diff. Every section is preserved byte-identical and in its original
  //    order unless it is an npm-family lockfile section, which is replaced.
  const { preamble, sections } = splitSections(diff);
  const outParts = [preamble];
  const substituted = [];
  const retained = [];
  const unclassified = [];
  for (const text of sections) {
    const section = classifySection(text);
    if (section.kind === 'unclassified') {
      unclassified.push({
        header: fromLatin1(section.headerLine),
        lines: section.lines,
        bytes: section.bytes,
        reason: section.reason,
      });
      outParts.push(text);
      continue;
    }
    const kind = generatedKind(section.path);
    if (kind !== null && isNpmFamily(section.path)) {
      const { record, text: placeholder } = substituteNpmSection(section, treeDir, baseTreeDir);
      substituted.push(record);
      outParts.push(placeholder);
    } else if (kind !== null) {
      retained.push({
        path: fromLatin1(section.path),
        kind,
        lines: section.lines,
        bytes: section.bytes,
        reason: 'no instrument for this kind',
      });
      outParts.push(text);
    } else {
      outParts.push(text);
    }
  }

  // 3. The record of what was done, naming the instrument by path and by the
  //    digest of its own bytes as read from the path it was invoked as.
  const self = process.argv[1] ?? fileURLToPath(import.meta.url);
  const report = {
    instrument: { path: INSTRUMENT_PATH, invokedAs: self, sha256: sha256Hex(fs.readFileSync(self)) },
    tree: treeDir,
    baseTree: baseTreeDir,
    generated,
    substituted,
    retained,
    unclassified,
  };

  // 4. Only now, with every section composed, are the outputs written.
  fs.writeFileSync(args['out-diff'], Buffer.from(outParts.join(''), 'latin1'));
  fs.writeFileSync(args['out-context-files'], Buffer.from(contextLines.join(''), 'latin1'));
  fs.writeFileSync(args['out-report'], `${JSON.stringify(report, null, 2)}\n`);
  process.stdout.write(
    `${PROG}: ${generated.length} generated path(s) excluded from the full-file context, ` +
      `${substituted.length} section(s) substituted, ${retained.length} retained, ${unclassified.length} unclassified\n`,
  );
  return 0;
}

// ---------------------------------------------------------------------------
// Enforce mode: the job's conclusion on a red predicate, read from the record.
// ---------------------------------------------------------------------------

function enforceReport(file) {
  let text;
  try {
    text = fs.readFileSync(file, 'utf8');
  } catch (e) {
    throw new InstrumentError(`${file}: report cannot be read: ${e.code ?? oneLine(e.message)}`);
  }
  let report;
  try {
    report = JSON.parse(text);
  } catch (e) {
    throw new InstrumentError(`${file}: report is not parseable JSON: ${oneLine(e.message)}`);
  }
  if (!isObject(report) || !Array.isArray(report.substituted)) {
    throw new InstrumentError(`${file}: report is not a JSON object carrying a substituted array`);
  }
  report.substituted.forEach((el, i) => {
    if (!isObject(el) || typeof el.path !== 'string' || typeof el.verdict !== 'string') {
      throw new InstrumentError(`${file}: substituted[${i}] is not an object carrying a path and a verdict`);
    }
  });
  const failing = report.substituted.filter((el) => el.verdict === 'FAIL');
  if (failing.length > 0) {
    for (const el of failing) {
      const checks = Array.isArray(el.checks) ? el.checks.filter((c) => typeof c === 'string') : [];
      const red = checks.filter((c) => /^check [^:]+: FAIL /.test(c));
      if (red.length === 0) red.push('check (unrecorded): FAIL verdict FAIL with no failing check line recorded');
      for (const line of red) {
        process.stderr.write(`${PROG}: FAIL ${el.path}: ${line}\n`);
        process.stderr.write(`::error::${PROG}: generated-file predicate FAIL for ${el.path}: ${line}\n`);
      }
    }
    return 1;
  }
  const unknown = report.substituted.find((el) => el.verdict !== 'PASS' && el.verdict !== 'DELETED');
  if (unknown !== undefined) {
    throw new InstrumentError(`${file}: ${unknown.path}: unknown verdict ${JSON.stringify(unknown.verdict)}`);
  }
  process.stdout.write(`${PROG}: predicate PASS for ${report.substituted.length} substituted path(s)\n`);
  return 0;
}

// ---------------------------------------------------------------------------
// CLI.
// ---------------------------------------------------------------------------

const COMPOSE_FLAGS = ['tree', 'base-tree', 'changed-files', 'diff', 'out-diff', 'out-context-files', 'out-report'];
const REQUIRED_COMPOSE_FLAGS = COMPOSE_FLAGS.filter((f) => f !== 'base-tree');

function parseArgs(argv) {
  const args = {};
  for (let i = 0; i < argv.length; i += 2) {
    const flag = argv[i];
    const value = argv[i + 1];
    if (!flag.startsWith('--') || value === undefined) throw new InstrumentError(`usage: ${flag}: expected --flag <value>`);
    const name = flag.slice(2);
    if (name !== 'enforce-report' && !COMPOSE_FLAGS.includes(name)) throw new InstrumentError(`usage: unknown flag ${flag}`);
    if (has(args, name)) throw new InstrumentError(`usage: ${flag} given twice`);
    args[name] = value;
  }
  return args;
}

function main(argv) {
  const args = parseArgs(argv);
  if (has(args, 'enforce-report')) {
    if (Object.keys(args).length !== 1) throw new InstrumentError('usage: --enforce-report takes no other flag');
    return enforceReport(args['enforce-report']);
  }
  for (const f of REQUIRED_COMPOSE_FLAGS) {
    if (!has(args, f)) throw new InstrumentError(`usage: --${f} is required`);
  }
  return compose(args);
}

try {
  process.exitCode = main(process.argv.slice(2));
} catch (e) {
  if (e instanceof InstrumentError) {
    process.stderr.write(`${PROG}: ${oneLine(e.message)}\n`);
    process.exitCode = e.exitCode;
  } else {
    process.stderr.write(`${PROG}: ${oneLine(e && e.stack ? e.stack : e)}\n`);
    process.exitCode = 2;
  }
}
