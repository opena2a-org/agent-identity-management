#!/usr/bin/env node
/**
 * Zero-skip gate for the vitest JSON report, shared by
 * .github/workflows/release.yml (publish-npm) and ci.yml (sdk-tests).
 *
 * Usage:
 *   node scripts/check-vitest-report.mjs <report.json> [--root=<dir>] [--allow-skips=<file>:<bound>]...
 *
 * The vitest JSON reporter's aggregate counters cannot see skipIf-skipped
 * cases: numPendingTests/numTodoTests stay 0 and numPassedTests absorbs
 * them (measured on the 1.3.1 release commit: default reporter 1158
 * passed / 36 skipped, aggregate 1194 total / 1194 passed / 0 pending).
 * So this script trusts ONLY the per-assertion statuses in
 * testResults[].assertionResults[].status and refuses — non-zero exit —
 * when anything failed, was skipped, pending or todo, or when nothing
 * ran at all.
 *
 * `--allow-skips=<SDK-root-relative test file>:<bound>` names an explicit,
 * bounded exclusion: a file whose "skipped" assertions are tolerated
 * because the gate's runner cannot provide their environment (see
 * docs/testing/sdk-publish-gate.md for the exclusion statement), and the
 * number of skips tolerated there. <bound> is a decimal integer of one or
 * more digits with no sign; an entry without a bound, or with anything
 * else as its bound, is refused (exit 2) before the report is judged, so
 * no unbounded form of an entry exists. The allowlist excuses ONLY status
 * "skipped", only inside the named files, and only while the file's
 * skipped count is at or below its bound: a count above the bound refuses
 * naming the file, the count, the bound and the delta. A count below the
 * bound passes (the environment-gated cases execute instead of skipping
 * when their backend and credentials are reachable). A skip in any other
 * file, a pending or todo anywhere, a failure anywhere, or a run where
 * nothing passed at all still refuses.
 *
 * An entry matches a report file only when the file's path, made
 * relative to the SDK root, EQUALS the entry's file: there is no suffix
 * match, so a nested copy such as
 * src/evil/src/a2a/A2AClient.integration.test.ts is outside the
 * allowlist. The SDK root is `--root=<dir>` when given and process.cwd()
 * otherwise (both workflow gate steps run this script from
 * working-directory sdk/typescript and pass no --root). Both sides are
 * normalised by turning backslashes into slashes and stripping a leading
 * "./". A report name outside the root is never allowlisted.
 */

import { readFileSync } from 'fs';
import { isAbsolute, relative, resolve } from 'path';

function refuse(message, code = 1) {
  console.error(message);
  process.exit(code);
}

const USAGE =
  'usage: check-vitest-report.mjs <report.json> [--root=<dir>] [--allow-skips=<file>:<bound>]...';

/**
 * Parse one --allow-skips entry into { file, bound }. The bound is the
 * text after the last ':' and must be one or more decimal digits with no
 * sign; everything else (no ':', an empty file, a non-integer or signed
 * bound) is refused with exit 2, quoting the entry.
 */
function parseAllowSkips(entry) {
  const sep = entry.lastIndexOf(':');
  const file = sep === -1 ? entry : entry.slice(0, sep);
  const boundText = sep === -1 ? '' : entry.slice(sep + 1);
  if (sep === -1 || file === '' || !/^[0-9]+$/.test(boundText)) {
    refuse(
      `refusing --allow-skips entry "${entry}": every entry must have the form ` +
        '--allow-skips=<file>:<bound> with <bound> an unsigned decimal integer ' +
        '(the number of skipped assertions tolerated in that file)',
      2,
    );
  }
  return { file, bound: Number.parseInt(boundText, 10), skipped: 0 };
}

const allowSkips = [];
let reportPath;
let rootArg;
for (const arg of process.argv.slice(2)) {
  if (arg.startsWith('--allow-skips=')) {
    allowSkips.push(parseAllowSkips(arg.slice('--allow-skips='.length)));
  } else if (arg.startsWith('--root=')) {
    rootArg = arg.slice('--root='.length);
  } else if (reportPath === undefined) {
    reportPath = arg;
  } else {
    refuse(`unexpected argument: ${arg}`, 2);
  }
}
if (reportPath === undefined) {
  refuse(USAGE, 2);
}
const sdkRoot = resolve(rootArg ?? process.cwd());

let report;
try {
  report = JSON.parse(readFileSync(reportPath, 'utf-8'));
} catch (err) {
  refuse(`unreadable vitest report at ${reportPath}: ${err.message}`, 2);
}
if (report === null || typeof report !== 'object' || !Array.isArray(report.testResults)) {
  refuse(`unreadable vitest report at ${reportPath}: no testResults array`, 2);
}

// Backslashes become slashes and a leading "./" is dropped, on both the
// report side and the entry side.
const normalise = (p) => String(p).replace(/\\/g, '/').replace(/^(\.\/)+/, '');

// The report's testResults[].name made relative to the SDK root. An
// absolute name outside the root comes back starting with ".." (or still
// absolute, on a different Windows drive) and can never equal an entry;
// a name that is not absolute is compared as given.
const toRootRelative = (fileName) => {
  const name = normalise(fileName);
  if (!isAbsolute(name)) return name;
  const rel = normalise(relative(sdkRoot, name));
  if (rel === '' || rel === '..' || rel.startsWith('../') || isAbsolute(rel)) return null;
  return rel;
};

// Each entry keeps its file as written (for messages) and its normalised
// form (for the exact comparison).
for (const entry of allowSkips) {
  entry.key = normalise(entry.file);
}
const allowEntryFor = (fileName) => {
  const rel = toRootRelative(fileName);
  return rel === null ? undefined : allowSkips.find((entry) => entry.key === rel);
};

const counts = { passed: 0, failed: 0, skipped: 0, pending: 0, todo: 0 };
let unknown = 0;
const unlistedSkips = [];
for (const fileResult of report.testResults) {
  const fileName = fileResult?.name ?? '(unnamed file)';
  for (const assertion of fileResult?.assertionResults ?? []) {
    const status = assertion?.status;
    if (status in counts) {
      counts[status] += 1;
    } else {
      unknown += 1;
    }
    if (status === 'skipped') {
      const entry = allowEntryFor(fileName);
      if (entry === undefined) {
        unlistedSkips.push(`${fileName} > ${assertion.fullName ?? assertion.title ?? '(untitled)'}`);
      } else {
        entry.skipped += 1;
      }
    }
  }
}
const total = counts.passed + counts.failed + counts.skipped + counts.pending + counts.todo + unknown;

// The per-status counts, printed before any verdict so every gate log
// carries them (including the size of the allowlisted skip population).
console.log(
  `tests passed=${counts.passed} failed=${counts.failed} skipped=${counts.skipped} ` +
    `pending=${counts.pending} todo=${counts.todo}` +
    (unknown > 0 ? ` unknown=${unknown}` : ''),
);

if (report.numTotalTests === 0 || report.testResults.length === 0 || total === 0) {
  refuse('refusing: no test ran at all');
}
if (counts.failed > 0) {
  refuse(`refusing: ${counts.failed} test(s) failed`);
}
if (counts.pending > 0 || counts.todo > 0 || unknown > 0) {
  refuse(
    `refusing: ${counts.pending} pending, ${counts.todo} todo and ${unknown} ` +
      'unknown-status assertion(s) — every test must actually run and pass',
  );
}
if (unlistedSkips.length > 0) {
  refuse(
    `refusing: ${unlistedSkips.length} skipped test(s) outside the allowlist:\n` +
      unlistedSkips.map((s) => `  - ${s}`).join('\n'),
  );
}
const overBound = allowSkips.filter((entry) => entry.skipped > entry.bound);
if (overBound.length > 0) {
  refuse(
    `refusing: ${overBound.length} allowlisted file(s) skipped more than their bound:\n` +
      overBound
        .map(
          (entry) =>
            `  - ${entry.file}: ${entry.skipped} skipped, bound ${entry.bound} ` +
            `(+${entry.skipped - entry.bound}); growing the bound is a gate change, ` +
            'see docs/testing/sdk-publish-gate.md',
        )
        .join('\n'),
  );
}
if (counts.passed === 0) {
  // Only allowlisted skips remain: nothing actually ran green.
  refuse('refusing: no test ran green — the report holds allowlisted skips only');
}

if (counts.skipped > 0) {
  // The entries as configured, then the measured skipped/bound per file.
  console.log(
    `ok: ${counts.passed} passed; ${counts.skipped} skipped, all inside the ` +
      'allowlisted environment-gated files (' +
      allowSkips.map((entry) => `${entry.file}:${entry.bound}`).join(', ') +
      ') and within their bounds: ' +
      allowSkips.map((entry) => `${entry.file} ${entry.skipped}/${entry.bound}`).join(', '),
  );
} else {
  console.log(`ok: ${counts.passed} passed, nothing skipped`);
}
