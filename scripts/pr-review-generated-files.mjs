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
// Invoked with --plan-over-budget, and only after the review action refused
// the full-file request as over its token budget, it plans the review sent
// instead: the same request cut into batches on file boundaries, or one
// diff-only request when the full-file context cannot be batched. The rules
// are stated at that mode's section below.
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
//   node scripts/pr-review-generated-files.mjs --plan-over-budget <new dir>
//     --measured-tokens <n> --system-prompt <in> --user-message <in>
//     --header <in> --full-files <in> --diff <in>
//     --wide-diff <in> --wide-context <n>
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
// Over-budget plan mode: what the workflow sends when the full-file request
// does not fit.
// ---------------------------------------------------------------------------
//
// The shared review action measures the request before it sends it and
// refuses one over its token budget, so the required check is INCONCLUSIVE
// with no finding and the pull request cannot merge. Measured on 2026-10-06:
// #573 (34 deleted backup files) at 228,383 input tokens, and #575 (85 changed
// lines) at 227,980, of which 390,051 bytes are one 194,903-byte line of a
// tracked tsconfig.tsbuildinfo, shown removed and added again.
//
// This mode runs only after that refusal and plans the review sent instead,
// by the first of two paths that fits:
//
//   batch      The measured request, cut on file boundaries. A changed file's
//              diff section travels with its full-file block, every block and
//              section lands in exactly one batch, and each batch repeats the
//              request's header (title, description, the whole changed-file
//              list). Nothing reviewed is added and nothing is dropped: before
//              anything is written, the parts are composed again and compared
//              with the measured user message byte for byte.
//   diff-only  One request carrying the diff with wider context and no
//              full-file blocks, when the full-file context cannot be batched:
//              one file's block and section together estimate over the budget,
//              or the batches would exceed the action's ceiling.
//
// Every size here is an ESTIMATE, and nothing is decided by it except which
// request to build. The action measures each batch, and the diff-only request,
// against the same budget immediately before sending it; a batch that errors,
// is unmeasured, is over budget or returns no verdict bound to its own nonce
// makes the run INCONCLUSIVE there. The estimate is the measured request's own
// density, tokens per byte, applied to each planned request. Content denser
// than the request's average estimates low: on #575 the tsbuildinfo batch
// estimates near 162,000 tokens and measures near 176,000. So several files
// share a batch only up to two thirds of the budget, a single file that cannot
// be split is planned up to the budget itself, and batches are numbered
// largest first, because the action reviews them in that order and an estimate
// that proves low is then refused before any review has been paid for.

// The action's defaults. pr-review.yml passes neither input to it, and the
// cells assert so, so these are the numbers the action applies.
const REVIEW_TOKEN_BUDGET = 180000;
const REVIEW_MAX_BATCHES = 8;
// Several files share a batch up to this estimate: a batch half again as
// dense as the request it was cut from still measures under the budget.
const PACK_TARGET_TOKENS = 120000;
// The context of the diff the gather step fetched from the pull request.
const GATHERED_CONTEXT_LINES = 3;

// The literal parts of the user message, as the Build review prompt step
// writes them.
const FULL_HEADING = toLatin1('\n\nFULL SOURCE FILES (line-numbered — use for verifying mitigations):\n');
const DIFF_HEADING = '\n\nDIFF (changes introduced in this PR):\n';
const BLOCK_HEADER = /\n=== (.*) ===\n/g;
const TRUNCATION_NOTICE = new RegExp(
  `\\n\\[FULL FILE CONTEXT TRUNCATED at [0-9]+ bytes ${toLatin1('—')} remaining files omitted\\]\\n$`,
);
const PLACEHOLDER_START = '=== GENERATED FILE PLACEHOLDER: ';
// The mode sections are inserted ahead of the system prompt's verdict
// instruction, so the reply format stays the last thing the prompt states.
const VERDICT_MARKER = '\n=== VERDICT LINE (required) ===\n';

const BATCH_SECTION = `${[
  '=== BATCHED REVIEW ===',
  'The full request for this pull request measured over the review budget, so the gate split it on file boundaries and this request is one batch of it. The user message begins with a line the gate writes, "REVIEW BATCH k OF N", and the lines after it, up to the first blank line, list the changed files whose diff and full source this batch carries. No file\'s diff or full source is split across batches, and the changed-files list after the description still names every file in the pull request.',
  'Review the changes this batch carries. When a finding depends on code in a file another batch carries, say so in its Mitigation check.',
  'REQUEST_CHANGES in any batch blocks the whole pull request.',
].join('\n')}\n`;

const diffOnlySection = (contextLines) => `${[
  '=== DIFF-ONLY REVIEW ===',
  `The full request for this pull request measured over the review budget and its full source could not be split into batches under it, so this request carries no FULL SOURCE FILES section. The user message begins with a line the gate writes, "DIFF-ONLY REVIEW". The DIFF shows up to ${contextLines} unchanged lines around each change: verify mitigations against those lines, and when a mitigation could sit outside them, say so in the finding's Mitigation check.`,
  'For line numbers, count from the new-file start of each hunk header ("+c" in "@@ -a,b +c,d @@") over its context and added lines. This replaces the LINE NUMBERS rule above for this request.',
].join('\n')}\n`;

const DIFF_ONLY_NOTE = 'DIFF-ONLY REVIEW\n\n';
const batchNote = (k, n, units) => `REVIEW BATCH ${k} OF ${n}\n${units.map((u) => `${u.display}\n`).join('')}\n`;

// The full-file context as the gather step's loop writes it: one line feed,
// then per file a line feed, `=== <path> ===` and the file through `nl -ba`,
// then, if the loop stopped early, its truncation notice. `nl -ba` starts
// every line it writes with a space or a digit, so a line beginning `=== ` is
// always a block header.
function splitFullFiles(full, file) {
  let body = full;
  let notice = '';
  const m = TRUNCATION_NOTICE.exec(full);
  if (m !== null) {
    notice = m[0];
    body = full.slice(0, m.index);
  }
  const heads = [...body.matchAll(BLOCK_HEADER)];
  const lead = heads.length > 0 ? body.slice(0, heads[0].index) : body;
  if (lead !== '\n' && lead !== '') {
    throw new InstrumentError(`${file}: --full-files does not begin as the gather step writes it`);
  }
  const blocks = heads.map((h, i) => ({
    path: h[1],
    text: body.slice(h.index, i + 1 < heads.length ? heads[i + 1].index : body.length),
  }));
  return { lead, blocks, notice };
}

// The composed diff in sections. A generated-file placeholder replaced its
// file's whole section, `diff --git` line included, so it starts a section of
// its own; inside a hunk every line starts with a space, `+`, `-` or `\`, so
// neither start can be written by the pull request at the start of a line.
function splitComposedDiff(diff) {
  const starts = [];
  let pos = 0;
  while (pos < diff.length) {
    if (diff.startsWith('diff --git ', pos) || diff.startsWith(PLACEHOLDER_START, pos)) starts.push(pos);
    const nl = diff.indexOf('\n', pos);
    if (nl < 0) break;
    pos = nl + 1;
  }
  const preamble = diff.slice(0, starts.length > 0 ? starts[0] : diff.length);
  const sections = starts.map((s, k) => diff.slice(s, k + 1 < starts.length ? starts[k + 1] : diff.length));
  return { preamble, sections };
}

// A section's path (null when it has none this script can read), the line
// naming it in a batch note, and a signature that is equal for one file's
// section in two diffs of the same change taken with different context.
function describeSection(text) {
  if (text.startsWith(PLACEHOLDER_START)) {
    const nl = text.indexOf('\n');
    const first = nl < 0 ? text : text.slice(0, nl);
    const name = first.slice(PLACEHOLDER_START.length).replace(/ ===$/, '');
    return { path: null, display: `${escapeToken(name)} (generated file placeholder)`, signature: first };
  }
  const c = classifySection(text);
  if (c.kind === 'section') {
    return { path: c.path, display: escapeToken(c.path), signature: `${c.status}\t${c.path}\t${c.oldPath ?? ''}` };
  }
  return { path: null, display: escapeToken(c.headerLine), signature: c.headerLine };
}

function sectionSignatures(sections) {
  return sections.map((s) => describeSection(s).signature).sort((a, b) => (a < b ? -1 : a > b ? 1 : 0));
}

// One unit per diff section, carrying the full-file block of the same path;
// a block whose path no section names is a unit of its own.
function planUnits(blocks, { preamble, sections }) {
  const blockByPath = new Map();
  blocks.forEach((b, i) => {
    if (!blockByPath.has(b.path)) blockByPath.set(b.path, i);
  });
  const used = new Set();
  const units = [];
  if (preamble !== '') units.push({ display: '(diff text before the first file section)', fullText: '', diffText: preamble });
  for (const text of sections) {
    const d = describeSection(text);
    let fullText = '';
    const bi = d.path === null ? undefined : blockByPath.get(d.path);
    if (bi !== undefined && !used.has(bi)) {
      used.add(bi);
      fullText = blocks[bi].text;
    }
    units.push({ display: d.display, fullText, diffText: text });
  }
  blocks.forEach((b, i) => {
    if (!used.has(i)) units.push({ display: escapeToken(b.path), fullText: b.text, diffText: '' });
  });
  return units;
}

function composeBatch(k, n, units, parts) {
  return (
    batchNote(k, n, units) +
    parts.header +
    FULL_HEADING +
    parts.lead +
    units.map((u) => u.fullText).join('') +
    parts.notice +
    DIFF_HEADING +
    units.map((u) => u.diffText).join('')
  );
}

function withSection(system, section) {
  const i = system.indexOf(VERDICT_MARKER);
  return system.slice(0, i + 1) + section + '\n' + system.slice(i + 1);
}

function planOverBudget(args) {
  const outDir = args['plan-over-budget'];
  if (/[\r\n]/.test(outDir)) throw new InstrumentError('usage: --plan-over-budget must be a path on one line');
  const measuredText = args['measured-tokens'];
  if (!/^[0-9]+$/.test(measuredText)) {
    throw new InstrumentError(`--measured-tokens ${JSON.stringify(measuredText)} is not a token count`);
  }
  const measured = Number(measuredText);
  if (measured <= REVIEW_TOKEN_BUDGET) {
    throw new InstrumentError(
      `--measured-tokens ${measured} is not over the ${REVIEW_TOKEN_BUDGET}-token review budget; this mode plans only a request the action refused as over it`,
    );
  }
  const wideContextText = args['wide-context'];
  if (!/^[1-9][0-9]*$/.test(wideContextText)) {
    throw new InstrumentError(`--wide-context ${JSON.stringify(wideContextText)} is not a positive line count`);
  }
  const wideContext = Number(wideContextText);
  const readInput = (flag) => {
    try {
      return fs.readFileSync(args[flag]).toString('latin1');
    } catch (e) {
      throw new InstrumentError(`${args[flag]}: --${flag} cannot be read: ${e.code ?? oneLine(e.message)}`);
    }
  };
  const system = readInput('system-prompt');
  const user = readInput('user-message');
  const header = readInput('header');
  const full = readInput('full-files');
  const diff = readInput('diff');
  const wideDiff = readInput('wide-diff');

  const at = system.indexOf(VERDICT_MARKER);
  if (at < 0 || system.indexOf(VERDICT_MARKER, at + 1) >= 0) {
    throw new InstrumentError(`${args['system-prompt']}: --system-prompt does not carry its verdict instruction heading exactly once`);
  }
  // The batches are cut from these parts, so the parts must BE the request
  // that was measured: any other text would be planned against a measurement
  // that is not its own, and reviewed as though it were the pull request.
  if (header + FULL_HEADING + full + DIFF_HEADING + diff !== user) {
    throw new InstrumentError(
      `${args['user-message']}: --user-message is not --header, --full-files and --diff composed as the workflow composes them`,
    );
  }

  const tokensPerByte = measured / (system.length + user.length);
  const estimate = (bytes) => Math.ceil(bytes * tokensPerByte);

  const { lead, blocks, notice } = splitFullFiles(full, args['full-files']);
  const diffParts = splitComposedDiff(diff);
  const units = planUnits(blocks, diffParts);
  const parts = { header, lead, notice };

  // 1. Batches. Units are packed in diff order, so neighbouring files share a
  //    batch, and a batch closes before a unit would carry it over the
  //    packing target; a unit over the target on its own is a batch of one.
  const systemBatch = withSection(system, BATCH_SECTION);
  const fixedBytes = systemBatch.length + composeBatch(1, 1, [], parts).length;
  const unitBytes = (u) => u.display.length + 1 + u.fullText.length + u.diffText.length;
  const groups = [];
  let current = [];
  let currentBytes = fixedBytes;
  for (const u of units) {
    const next = currentBytes + unitBytes(u);
    if (current.length > 0 && estimate(next) > PACK_TARGET_TOKENS) {
      groups.push(current);
      current = [u];
      currentBytes = fixedBytes + unitBytes(u);
    } else {
      current.push(u);
      currentBytes = next;
    }
  }
  if (current.length > 0) groups.push(current);

  const planned = groups.map((g, i) => ({
    units: g,
    order: i,
    estimatedTokens: estimate(fixedBytes + g.reduce((s, u) => s + unitBytes(u), 0)),
  }));
  let batchRefusal = null;
  const lone = planned.find((p) => p.estimatedTokens > REVIEW_TOKEN_BUDGET);
  if (planned.length < 2) {
    batchRefusal = 'the full-file context does not divide into two or more batches';
  } else if (lone !== undefined) {
    batchRefusal = `${fromLatin1(lone.units[0].display)} estimates ${lone.estimatedTokens} input tokens on its own with its full source, over the ${REVIEW_TOKEN_BUDGET}-token budget, and one file is never split across batches`;
  } else if (planned.length > REVIEW_MAX_BATCHES) {
    batchRefusal = `the full-file context needs ${planned.length} batches, over the action's ceiling of ${REVIEW_MAX_BATCHES}`;
  }

  // 2. Diff-only. The wide diff is used only when it carries the same file
  //    sections as the gathered diff and estimates under the budget; the
  //    gathered diff is otherwise the one sent, and the action measures it.
  const diffOnlyText = (d) => DIFF_ONLY_NOTE + header + DIFF_HEADING + d;
  const wideParts = splitComposedDiff(wideDiff);
  const sameSections =
    wideParts.preamble === diffParts.preamble &&
    JSON.stringify(sectionSignatures(wideParts.sections)) === JSON.stringify(sectionSignatures(diffParts.sections));
  const wideSystem = withSection(system, diffOnlySection(wideContext));
  const wideEstimate = estimate(wideSystem.length + diffOnlyText(wideDiff).length);
  let diffOnly;
  if (sameSections && wideEstimate <= REVIEW_TOKEN_BUDGET) {
    diffOnly = { contextLines: wideContext, system: wideSystem, text: diffOnlyText(wideDiff), wideDiffUsed: true, reason: null };
  } else {
    diffOnly = {
      contextLines: GATHERED_CONTEXT_LINES,
      system: withSection(system, diffOnlySection(GATHERED_CONTEXT_LINES)),
      text: diffOnlyText(diff),
      wideDiffUsed: false,
      reason: sameSections
        ? `the ${wideContext}-line diff estimates ${wideEstimate} input tokens, over the budget`
        : `the ${wideContext}-line diff does not carry the same file sections as the gathered diff`,
    };
  }
  diffOnly.estimatedTokens = estimate(diffOnly.system.length + diffOnly.text.length);

  const mode = batchRefusal === null ? 'batch' : 'diff-only';
  // Largest first; equal estimates keep diff order.
  const ordered =
    mode === 'batch' ? [...planned].sort((a, b) => b.estimatedTokens - a.estimatedTokens || a.order - b.order) : [];
  const batchFiles = ordered.map((p, i) => {
    const text = composeBatch(i + 1, ordered.length, p.units, parts);
    return {
      name: `batch-${String(i + 1).padStart(2, '0')}.txt`,
      text,
      files: p.units.map((u) => fromLatin1(u.display)),
      bytes: systemBatch.length + text.length,
      estimatedTokens: estimate(systemBatch.length + text.length),
    };
  });

  const batchDir = path.join(outDir, 'batches');
  const systemFile = path.join(outDir, 'system_prompt.txt');
  const diffOnlyFile = path.join(outDir, 'user_msg_diff_only.txt');
  // In batch mode the action reads the batch directory and not this input.
  // It names a file that does not exist, so a run that lost its batch
  // directory would be refused as missing a request, never sent one batch as
  // though it were the pull request.
  const noSingleRequest = path.join(outDir, 'no-single-request-in-batch-mode');
  const outputs =
    mode === 'batch'
      ? {
          mode,
          batches: String(batchFiles.length),
          'batch-dir': batchDir,
          'system-prompt-file': systemFile,
          'user-message-file': noSingleRequest,
          'context-lines': '',
        }
      : {
          mode,
          batches: '',
          'batch-dir': '',
          'system-prompt-file': systemFile,
          'user-message-file': diffOnlyFile,
          'context-lines': String(diffOnly.contextLines),
        };
  const record = {
    instrument: INSTRUMENT_PATH,
    mode,
    measuredTokens: measured,
    measuredBytes: system.length + user.length,
    tokensPerByte,
    budget: REVIEW_TOKEN_BUDGET,
    packTarget: PACK_TARGET_TOKENS,
    maxBatches: REVIEW_MAX_BATCHES,
    batchRefusal,
    batches: batchFiles.map(({ name, files, bytes, estimatedTokens }) => ({ name, files, bytes, estimatedTokens })),
    diffOnly: {
      contextLines: diffOnly.contextLines,
      wideDiffUsed: diffOnly.wideDiffUsed,
      reason: diffOnly.reason,
      bytes: diffOnly.system.length + diffOnly.text.length,
      estimatedTokens: diffOnly.estimatedTokens,
    },
    outputs,
  };

  // Only now, with every request composed, is anything written.
  try {
    fs.mkdirSync(outDir);
  } catch (e) {
    throw new InstrumentError(`${outDir}: --plan-over-budget directory cannot be created: ${e.code ?? oneLine(e.message)}`);
  }
  if (mode === 'batch') {
    fs.mkdirSync(batchDir);
    for (const b of batchFiles) fs.writeFileSync(path.join(batchDir, b.name), Buffer.from(b.text, 'latin1'));
    fs.writeFileSync(systemFile, Buffer.from(systemBatch, 'latin1'));
  } else {
    fs.writeFileSync(diffOnlyFile, Buffer.from(diffOnly.text, 'latin1'));
    fs.writeFileSync(systemFile, Buffer.from(diffOnly.system, 'latin1'));
  }
  fs.writeFileSync(path.join(outDir, 'plan.json'), `${JSON.stringify(record, null, 2)}\n`);
  fs.writeFileSync(
    path.join(outDir, 'outputs.txt'),
    Object.entries(outputs)
      .map(([k, v]) => `${k}=${v}\n`)
      .join(''),
  );
  const summary =
    mode === 'batch'
      ? `${batchFiles.length} batches estimated at ${batchFiles.map((b) => b.estimatedTokens).join(', ')} input tokens`
      : `one diff-only request with ${diffOnly.contextLines} context lines estimated at ${diffOnly.estimatedTokens} input tokens, because ${batchRefusal}`;
  process.stdout.write(
    `${PROG}: the full-file request measured ${measured} input tokens, over the ${REVIEW_TOKEN_BUDGET}-token budget; planned ${summary} (${tokensPerByte.toFixed(4)} tokens per byte, from that measurement)\n`,
  );
  return 0;
}

// ---------------------------------------------------------------------------
// CLI.
// ---------------------------------------------------------------------------

const COMPOSE_FLAGS = ['tree', 'base-tree', 'changed-files', 'diff', 'out-diff', 'out-context-files', 'out-report'];
const REQUIRED_COMPOSE_FLAGS = COMPOSE_FLAGS.filter((f) => f !== 'base-tree');
const PLAN_FLAGS = [
  'plan-over-budget',
  'measured-tokens',
  'system-prompt',
  'user-message',
  'header',
  'full-files',
  'diff',
  'wide-diff',
  'wide-context',
];

function parseArgs(argv) {
  const args = {};
  for (let i = 0; i < argv.length; i += 2) {
    const flag = argv[i];
    const value = argv[i + 1];
    if (!flag.startsWith('--') || value === undefined) throw new InstrumentError(`usage: ${flag}: expected --flag <value>`);
    const name = flag.slice(2);
    if (name !== 'enforce-report' && !COMPOSE_FLAGS.includes(name) && !PLAN_FLAGS.includes(name)) {
      throw new InstrumentError(`usage: unknown flag ${flag}`);
    }
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
  if (has(args, 'plan-over-budget')) {
    for (const f of Object.keys(args)) {
      if (!PLAN_FLAGS.includes(f)) throw new InstrumentError(`usage: --${f} is not a --plan-over-budget flag`);
    }
    for (const f of PLAN_FLAGS) {
      if (!has(args, f)) throw new InstrumentError(`usage: --${f} is required`);
    }
    return planOverBudget(args);
  }
  for (const f of Object.keys(args)) {
    if (!COMPOSE_FLAGS.includes(f)) throw new InstrumentError(`usage: unknown flag --${f}`);
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
