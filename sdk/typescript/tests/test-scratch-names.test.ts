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
 *
 * The scanner reads every source file under tests/ (fixtures aside), helpers
 * as well as test files. It reads a prefix written in any quote style, held in
 * a name bound once with `const`, or passed as the last segment of any `join`
 * or `resolve` call. A mkdtemp call whose prefix it cannot read fails the
 * guard, so a form it does not know is reported rather than passed unchecked.
 */

const TESTS_DIR = __dirname;
const REPO_ROOT = join(__dirname, '..', '..', '..');
const SOURCE_FILE = /\.[cm]?[jt]s$/;

function sourceFiles(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      if (entry.name !== 'fixtures') out.push(...sourceFiles(path));
    } else if (SOURCE_FILE.test(entry.name) && path !== __filename) {
      // This file is left out: its scanner cases are sample mkdtemp calls.
      out.push(path);
    }
  }
  return out;
}

/** The index just past the string literal that opens at `start`. */
function endOfString(source: string, start: number): number {
  const quote = source[start];
  for (let i = start + 1; i < source.length; i++) {
    const c = source[i];
    if (c === '\\') i++;
    else if (c === quote) return i + 1;
    else if (quote === '`' && c === '$' && source[i + 1] === '{') i = endOfGroup(source, i + 1) - 1;
  }
  return source.length;
}

/** The index just past the bracket that closes the one at `start`. */
function endOfGroup(source: string, start: number): number {
  let depth = 0;
  for (let i = start; i < source.length; ) {
    const c = source[i];
    if (c === "'" || c === '"' || c === '`') {
      i = endOfString(source, i);
      continue;
    }
    if ('([{'.includes(c)) depth++;
    else if (')]}'.includes(c) && --depth === 0) return i + 1;
    i++;
  }
  return source.length;
}

/** The top-level comma-separated parts of an argument list. */
function splitArguments(text: string): string[] {
  const parts: string[] = [];
  let from = 0;
  for (let i = 0; i < text.length; ) {
    const c = text[i];
    if (c === "'" || c === '"' || c === '`') {
      i = endOfString(text, i);
      continue;
    }
    if ('([{'.includes(c)) {
      i = endOfGroup(text, i);
      continue;
    }
    if (c === ',') {
      parts.push(text.slice(from, i).trim());
      from = i + 1;
    }
    i++;
  }
  const last = text.slice(from).trim();
  if (last !== '') parts.push(last);
  return parts;
}

/** The text of a string literal with no substitutions, or null for any other expression. */
function literalText(expr: string): string | null {
  if (!/^['"`]/.test(expr) || endOfString(expr, 0) !== expr.length) return null;
  if (expr[0] === '`' && expr.includes('${')) return null;
  return expr.slice(1, -1);
}

/** The prefix an expression passed to mkdtemp spells, or null when it cannot be read. */
function resolvePrefix(expr: string, source: string): string | null {
  const literal = literalText(expr);
  if (literal !== null) return literal;

  const call = /^(?:[A-Za-z_$][\w$]*\.)*(?:join|resolve)\s*\(/.exec(expr);
  if (call && endOfGroup(expr, call[0].length - 1) === expr.length) {
    const segments = splitArguments(expr.slice(call[0].length, -1));
    return segments.length > 0 ? resolvePrefix(segments[segments.length - 1], source) : null;
  }

  if (/^[A-Za-z_$][\w$]*$/.test(expr)) {
    const name = expr.replace(/\$/g, '\\$');
    const bindings = [...source.matchAll(new RegExp(`\\bconst\\s+${name}\\s*(?::\\s*string\\s*)?=\\s*`, 'g'))];
    if (bindings.length !== 1) return null;
    const start = (bindings[0].index ?? 0) + bindings[0][0].length;
    if (!/['"`]/.test(source[start] ?? '')) return null;
    return literalText(source.slice(start, endOfString(source, start)));
  }

  return null;
}

interface ScratchCall {
  line: number;
  /** The last path segment of the prefix, or null when the scanner cannot read it. */
  prefix: string | null;
}

/** Every mkdtemp or mkdtempSync call in a source text, with the prefix it passes. */
function scratchCalls(source: string): ScratchCall[] {
  const calls: ScratchCall[] = [];
  for (const m of source.matchAll(/\bmkdtemp(?:Sync)?\s*\(/g)) {
    const at = m.index ?? 0;
    const open = at + m[0].length - 1;
    const args = splitArguments(source.slice(open + 1, endOfGroup(source, open) - 1));
    const prefix = args.length > 0 ? resolvePrefix(args[0], source) : null;
    calls.push({
      line: source.slice(0, at).split('\n').length,
      prefix: prefix === null ? null : prefix.slice(prefix.lastIndexOf('/') + 1),
    });
  }
  return calls;
}

function scratchPrefixes(source: string): Array<string | null> {
  return scratchCalls(source).map((call) => call.prefix);
}

/** Every shell function a source text defines, as `name() {` at a line start. */
function shellFunctions(source: string): string[] {
  return [...source.matchAll(/^\s*([A-Za-z_][A-Za-z0-9_]*)\(\) *\{/gm)].map((m) => m[1]);
}

function read(name: string): string {
  return readFileSync(join(TESTS_DIR, name), 'utf-8');
}

describe('the mkdtemp prefix scanner', () => {
  it.each([
    { form: 'a single-quoted literal', source: "mkdtempSync(join(tmpdir(), 'single-'))", prefix: 'single-' },
    { form: 'a double-quoted literal', source: 'mkdtempSync(join(tmpdir(), "double-"))', prefix: 'double-' },
    { form: 'a template literal', source: 'mkdtempSync(join(tmpdir(), `template-`))', prefix: 'template-' },
    {
      form: 'a nodePath.join call',
      source: "fs.mkdtempSync(nodePath.join(os.tmpdir(), 'node-path-'))",
      prefix: 'node-path-',
    },
    { form: 'a resolve call', source: "await mkdtemp(path.resolve(base, 'resolved-'))", prefix: 'resolved-' },
    { form: 'a bare path', source: "mkdtempSync('/tmp/bare-')", prefix: 'bare-' },
    {
      form: 'a const name',
      source: "const prefix = 'held-';\nconst dir = fs.mkdtempSync(path.join(base, prefix));",
      prefix: 'held-',
    },
    {
      form: 'a call with options',
      source: "mkdtempSync(join(tmpdir(), 'with-options-'), { encoding: 'utf8' })",
      prefix: 'with-options-',
    },
  ])('reads a prefix written as $form', ({ source, prefix }) => {
    expect(scratchPrefixes(source)).toEqual([prefix]);
  });

  it.each([
    {
      form: 'a function parameter',
      source: 'function scratch(prefix) {\n  return mkdtempSync(join(tmpdir(), prefix));\n}',
    },
    { form: 'a template with a substitution', source: 'mkdtempSync(join(tmpdir(), `${name}-`))' },
    { form: 'a concatenation', source: "mkdtempSync(join(tmpdir(), 'run-' + id))" },
    { form: 'a let name', source: "let prefix = 'held-';\nmkdtempSync(join(tmpdir(), prefix))" },
  ])('reports a prefix written as $form as unreadable', ({ source }) => {
    expect(scratchPrefixes(source)).toEqual([null]);
  });

  it('names the line of each call', () => {
    const source = "import { mkdtempSync } from 'fs';\n\nmkdtempSync(join(tmpdir(), 'first-'));\nmkdtempSync('second-');";
    expect(scratchCalls(source)).toEqual([
      { line: 3, prefix: 'first-' },
      { line: 4, prefix: 'second-' },
    ]);
  });
});

describe('scratch directories and shell helpers in the tests are named after what they serve', () => {
  it('every mkdtemp call under tests/ passes a prefix the scanner reads', () => {
    const unreadable: string[] = [];
    for (const file of sourceFiles(TESTS_DIR)) {
      for (const call of scratchCalls(readFileSync(file, 'utf-8'))) {
        if (call.prefix === null) unreadable.push(`${relative(TESTS_DIR, file)}:${call.line}`);
      }
    }
    expect(unreadable).toEqual([]);
  });

  it('no mkdtemp prefix under tests/ opens with a short identifier and a number', () => {
    const offenders: string[] = [];
    for (const file of sourceFiles(TESTS_DIR)) {
      for (const call of scratchCalls(readFileSync(file, 'utf-8'))) {
        if (call.prefix !== null && /^[A-Za-z]{1,4}-\d+-/.test(call.prefix)) {
          offenders.push(`${relative(TESTS_DIR, file)}:${call.line}: '${call.prefix}'`);
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

  it('the generated-files review script test names its scratch directory after the test', () => {
    const source = readFileSync(join(REPO_ROOT, 'scripts', 'test-pr-review-generated-files.mjs'), 'utf-8');
    expect(scratchPrefixes(source)).toEqual(['pr-review-generated-files-']);
  });
});
