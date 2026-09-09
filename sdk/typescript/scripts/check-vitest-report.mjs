#!/usr/bin/env node
/**
 * Zero-skip gate for the vitest JSON report, shared by
 * .github/workflows/release.yml (publish-npm) and ci.yml (sdk-tests).
 *
 * Usage:
 *   node scripts/check-vitest-report.mjs <report.json> [--root=<dir>] [--allow-skips=<file>]...
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
 * `--allow-skips=<SDK-root-relative test file>` names an explicit
 * exclusion: a file whose "skipped" assertions are tolerated because the
 * gate's runner cannot provide their environment (see
 * docs/testing/sdk-publish-gate.md for the exclusion statement). The
 * allowlist excuses ONLY status "skipped" and only inside the named
 * files: a skip in any other file, a pending or todo anywhere, a failure
 * anywhere, or a run where nothing passed at all still refuses.
 *
 * An entry matches a report file only when the file's path, made
 * relative to the SDK root, EQUALS the entry: there is no suffix match,
 * so a nested copy such as src/evil/src/a2a/A2AClient.integration.test.ts
 * is outside the allowlist. The SDK root is `--root=<dir>` when given and
 * process.cwd() otherwise (both workflow gate steps run this script from
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

const allowSkips = [];
let reportPath;
let rootArg;
for (const arg of process.argv.slice(2)) {
  if (arg.startsWith('--allow-skips=')) {
    allowSkips.push(arg.slice('--allow-skips='.length));
  } else if (arg.startsWith('--root=')) {
    rootArg = arg.slice('--root='.length);
  } else if (reportPath === undefined) {
    reportPath = arg;
  } else {
    refuse(`unexpected argument: ${arg}`, 2);
  }
}
if (reportPath === undefined) {
  refuse(
    'usage: check-vitest-report.mjs <report.json> [--root=<dir>] [--allow-skips=<file>]...',
    2,
  );
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

const allowSkipsNormalised = allowSkips.map(normalise);
const isAllowlisted = (fileName) => {
  const rel = toRootRelative(fileName);
  return rel !== null && allowSkipsNormalised.includes(rel);
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
    if (status === 'skipped' && !isAllowlisted(fileName)) {
      unlistedSkips.push(`${fileName} > ${assertion.fullName ?? assertion.title ?? '(untitled)'}`);
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
if (counts.passed === 0) {
  // Only allowlisted skips remain: nothing actually ran green.
  refuse('refusing: no test ran green — the report holds allowlisted skips only');
}

if (counts.skipped > 0) {
  console.log(
    `ok: ${counts.passed} passed; ${counts.skipped} skipped, all inside the ` +
      `allowlisted environment-gated files (${allowSkips.join(', ')})`,
  );
} else {
  console.log(`ok: ${counts.passed} passed, nothing skipped`);
}
