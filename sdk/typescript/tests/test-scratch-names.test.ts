import { describe, it, expect } from 'vitest';
import { readdirSync, readFileSync } from 'fs';
import { join, relative } from 'path';

/**
 * Scratch directories and shell helpers in the SDK's repository tests are
 * named after the stub or test they serve.
 *
 * A leftover scratch directory or a line of stub trace is read by someone who
 * has only that name to go on. The docker publish release-only test and the
 * license compliance workflow test named theirs with a short identifier and a
 * work-item number, and the license test's stub helpers with the same
 * identifier, none of which says what made them.
 */

const TESTS_DIR = __dirname;

function testFiles(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      if (entry.name !== 'fixtures') out.push(...testFiles(path));
    } else if (entry.name.endsWith('.test.ts')) {
      out.push(path);
    }
  }
  return out;
}

/** Every string-literal prefix passed to mkdtemp in a source text. */
function scratchPrefixes(source: string): string[] {
  const prefixes: string[] = [];
  for (const m of source.matchAll(/mkdtemp(?:Sync)?\(\s*(?:path\.)?join\([^;]*?,\s*'([^']+)'\s*\)/g)) {
    prefixes.push(m[1]);
  }
  return prefixes;
}

/** Every shell function a source text defines, as `name() {` at a line start. */
function shellFunctions(source: string): string[] {
  return [...source.matchAll(/^\s*([A-Za-z_][A-Za-z0-9_]*)\(\) *\{/gm)].map((m) => m[1]);
}

function read(name: string): string {
  return readFileSync(join(TESTS_DIR, name), 'utf-8');
}

describe('scratch directories and shell helpers in the SDK tests are named after what they serve', () => {
  it('no mkdtemp prefix under tests/ opens with a short identifier and a number', () => {
    const offenders: string[] = [];
    for (const file of testFiles(TESTS_DIR)) {
      for (const prefix of scratchPrefixes(readFileSync(file, 'utf-8'))) {
        if (/^[A-Za-z]{1,4}-\d+-/.test(prefix)) {
          offenders.push(`${relative(TESTS_DIR, file)}: '${prefix}'`);
        }
      }
    }
    expect(offenders).toEqual([]);
  });

  it('the docker publish release-only test names its scratch directory after the release guard stubs', () => {
    const prefixes = scratchPrefixes(read('docker-publish-release-only.test.ts'));
    expect(prefixes).toEqual(['docker-publish-release-guard-stubs-']);
  });

  it('the license compliance workflow test names its scratch directories after the test', () => {
    const prefixes = scratchPrefixes(read('license-compliance-workflow.test.ts'));
    expect(prefixes).toEqual([
      'license-compliance-exec-probe-',
      'license-compliance-sandbox-',
    ]);
  });

  it('the license compliance workflow stubs define their helpers as stub_*, beside $STUB_LOG and $STUB_DIR', () => {
    const source = read('license-compliance-workflow.test.ts');
    expect(shellFunctions(source)).toEqual(['stub_log', 'stub_replay']);
    expect(source).toContain('>> "$STUB_LOG"');
    expect(source).toContain('counter="$STUB_DIR/count.$action"');
  });
});
