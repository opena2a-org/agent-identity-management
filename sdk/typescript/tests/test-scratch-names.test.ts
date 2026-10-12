import { describe, it, expect } from 'vitest';
import { mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'fs';
import { tmpdir } from 'os';
import { dirname, join, relative } from 'path';
import * as ts from 'typescript';

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
 * The scanner parses every source file under tests/, helpers as well as test
 * files, and every test file under src/. It leaves out fixtures directories,
 * and this file, whose scanner cases are sample mkdtemp calls. Comments and the
 * text of strings are never read as calls. It reads a call to mkdtemp or
 * mkdtempSync made by name or as a property (`fs.mkdtempSync`), and a prefix
 * written as a literal in any quote style, held in a name the file declares
 * once, with `const` and an initializer that is one string literal, or passed
 * as the last argument of a `join` or `resolve` call (`path.join`, for one).
 * Each of these is reported as unreadable and fails the guard: a call whose
 * prefix it cannot read, an import or destructuring that renames either
 * function, and any other mention of either name in code (`const mk =
 * fs.mkdtempSync`, for one). A call that reaches either function without
 * writing its name as code (`fs['mkdtempSync'](...)`, for one) is not seen.
 */

const TESTS_DIR = __dirname;
const SDK_ROOT = join(__dirname, '..');
const REPO_ROOT = join(__dirname, '..', '..', '..');
const SOURCE_FILE = /\.[cm]?[jt]s$/;
const TEST_FILE = /\.test\.ts$/;
const MKDTEMP = new Set(['mkdtemp', 'mkdtempSync']);

/** Every file under `dir` whose name matches `pattern`, at any depth, fixtures directories and this file aside. */
function sourceFiles(dir: string, pattern: RegExp): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      if (entry.name !== 'fixtures') out.push(...sourceFiles(path, pattern));
    } else if (pattern.test(entry.name) && path !== __filename) {
      // This file is left out: its scanner cases are sample mkdtemp calls.
      out.push(path);
    }
  }
  return out;
}

/** The files the guard reads: every source file under tests/ and every test file under src/. */
function scannedFiles(): string[] {
  return [...sourceFiles(TESTS_DIR, SOURCE_FILE), ...sourceFiles(join(SDK_ROOT, 'src'), TEST_FILE)];
}

/** Whether `name` is the name a declaration binds in the scope around it. */
function declaresName(name: ts.Identifier): boolean {
  const parent = name.parent;
  return (
    (ts.isVariableDeclaration(parent) ||
      ts.isParameter(parent) ||
      ts.isBindingElement(parent) ||
      ts.isFunctionDeclaration(parent) ||
      ts.isImportClause(parent) ||
      ts.isImportSpecifier(parent) ||
      ts.isNamespaceImport(parent)) &&
    parent.name === name
  );
}

/**
 * The string each name a source file declares holds: the literal, for a name
 * declared once, with `const` and an initializer that is one string literal,
 * and null for a name declared any other way or more than once.
 */
function heldStrings(file: ts.SourceFile): Map<string, string | null> {
  const held = new Map<string, string | null>();
  const visit = (node: ts.Node): void => {
    if (ts.isIdentifier(node) && declaresName(node)) {
      const decl = node.parent;
      const literal =
        ts.isVariableDeclaration(decl) &&
        (ts.getCombinedNodeFlags(decl) & ts.NodeFlags.BlockScoped) === ts.NodeFlags.Const &&
        decl.initializer !== undefined &&
        ts.isStringLiteralLike(decl.initializer)
          ? decl.initializer.text
          : null;
      held.set(node.text, held.has(node.text) ? null : literal);
    }
    ts.forEachChild(node, visit);
  };
  visit(file);
  return held;
}

/** Whether `expr` is a name, or names joined by dots. */
function isNameChain(expr: ts.Expression): expr is ts.Identifier | ts.PropertyAccessExpression {
  return ts.isIdentifier(expr) || (ts.isPropertyAccessExpression(expr) && isNameChain(expr.expression));
}

/** The prefix an expression passed to mkdtemp spells, or null when it cannot be read. */
function resolvePrefix(expr: ts.Expression, held: Map<string, string | null>): string | null {
  if (ts.isStringLiteralLike(expr)) return expr.text;
  if (ts.isIdentifier(expr)) return held.get(expr.text) ?? null;
  if (ts.isCallExpression(expr) && isNameChain(expr.expression) && expr.arguments.length > 0) {
    const callee = expr.expression;
    const name = ts.isIdentifier(callee) ? callee.text : callee.name.text;
    if (name === 'join' || name === 'resolve') {
      return resolvePrefix(expr.arguments[expr.arguments.length - 1], held);
    }
  }
  return null;
}

/** The call a mention of mkdtemp or mkdtempSync is the callee of, by name or as a property, if any. */
function callOf(mention: ts.Identifier): ts.CallExpression | null {
  const parent = mention.parent;
  const callee = ts.isPropertyAccessExpression(parent) && parent.name === mention ? parent : mention;
  return ts.isCallExpression(callee.parent) && callee.parent.expression === callee ? callee.parent : null;
}

/** Whether a mention of mkdtemp or mkdtempSync is a plain import or destructured binding, which renames nothing. */
function bindsUnrenamed(mention: ts.Identifier): boolean {
  const parent = mention.parent;
  return (ts.isImportSpecifier(parent) || ts.isBindingElement(parent)) && parent.name.getText() === mention.text;
}

interface ScratchCall {
  line: number;
  /** The last path segment of the prefix, or null when the scanner cannot read it. */
  prefix: string | null;
}

/**
 * Every mkdtemp or mkdtempSync call in a source text, with the prefix it
 * passes. A mention of either name that is neither a call nor a plain import
 * or destructured binding counts as a call whose prefix cannot be read.
 */
function scratchCalls(source: string): ScratchCall[] {
  const file = ts.createSourceFile('scanned.ts', source, ts.ScriptTarget.Latest, true);
  const held = heldStrings(file);
  const calls: ScratchCall[] = [];
  const visit = (node: ts.Node): void => {
    if (ts.isIdentifier(node) && MKDTEMP.has(node.text)) {
      const call = callOf(node);
      if (call !== null || !bindsUnrenamed(node)) {
        const prefix = call !== null && call.arguments.length > 0 ? resolvePrefix(call.arguments[0], held) : null;
        calls.push({
          line: file.getLineAndCharacterOfPosition(node.getStart(file)).line + 1,
          prefix: prefix === null ? null : prefix.slice(prefix.lastIndexOf('/') + 1),
        });
      }
    }
    ts.forEachChild(node, visit);
  };
  visit(file);
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
    { form: 'a literal holding an escaped quote', source: "mkdtempSync(join(tmpdir(), 'it\\'s-'))", prefix: "it's-" },
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
      form: 'a typed const name',
      source: "const prefix: string = 'typed-';\nmkdtempSync(join(tmpdir(), prefix))",
      prefix: 'typed-',
    },
    {
      form: 'a call with options',
      source: "mkdtempSync(join(tmpdir(), 'with-options-'), { encoding: 'utf8' })",
      prefix: 'with-options-',
    },
    { form: 'a call with a space before its parenthesis', source: "mkdtempSync (join(tmpdir(), 'spaced-'))", prefix: 'spaced-' },
    {
      form: 'a call through a destructured binding',
      source: "const { mkdtempSync } = require('fs');\nmkdtempSync(join(tmpdir(), 'destructured-'))",
      prefix: 'destructured-',
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
    {
      form: 'a const whose initializer continues past its first literal',
      source: "const opaque = 'abc-' + 99 + '-';\nmkdtempSync(join(tmpdir(), opaque))",
    },
    {
      form: 'a const name declared twice',
      source: "const prefix = 'first-';\nfunction scratch() {\n  const prefix = 'second-';\n  return mkdtempSync(join(tmpdir(), prefix));\n}",
    },
    {
      form: 'a const name the file also declares as a parameter',
      source: "const prefix = 'held-';\nfunction scratch(prefix: string) {\n  return mkdtempSync(join(tmpdir(), prefix));\n}",
    },
    {
      form: 'a const name the file also declares as a destructured parameter',
      source: "const prefix = 'held-';\nfunction scratch({ prefix }) {\n  return mkdtempSync(join(tmpdir(), prefix));\n}",
    },
    {
      form: 'a const name the file also declares with let',
      source: "const prefix = 'held-';\nfunction scratch() {\n  let prefix = 'other-';\n  return mkdtempSync(join(tmpdir(), prefix));\n}",
    },
    {
      form: 'a const name the file also declares with var',
      source: "const prefix = 'held-';\nfunction scratch() {\n  var prefix = 'other-';\n  return mkdtempSync(join(tmpdir(), prefix));\n}",
    },
    {
      form: 'a const name the file also declares as a function',
      source: "function scratch() {\n  const prefix = 'held-';\n  return mkdtempSync(join(tmpdir(), prefix));\n}\nfunction prefix() {}",
    },
    {
      form: 'a const name the file also imports by default',
      source: "import prefix from './names';\nfunction scratch() {\n  const prefix = 'held-';\n  return mkdtempSync(join(tmpdir(), prefix));\n}",
    },
    {
      form: 'a const name the file also imports by name',
      source: "import { prefix } from './names';\nfunction scratch() {\n  const prefix = 'held-';\n  return mkdtempSync(join(tmpdir(), prefix));\n}",
    },
    {
      form: 'a const name the file also imports as a namespace',
      source: "import * as prefix from './names';\nfunction scratch() {\n  const prefix = 'held-';\n  return mkdtempSync(join(tmpdir(), prefix));\n}",
    },
  ])('reports a prefix written as $form as unreadable', ({ source }) => {
    expect(scratchPrefixes(source)).toEqual([null]);
  });

  it.each([
    {
      form: 'an import that renames mkdtempSync',
      source: "import { mkdtempSync as mk } from 'fs';\nmk(join(tmpdir(), 'renamed-'))",
    },
    {
      form: 'a destructuring that renames mkdtempSync',
      source: "const { mkdtempSync: mk } = require('fs');\nmk(join(tmpdir(), 'renamed-'))",
    },
    { form: 'mkdtemp taken as a value', source: "const mk = fs.mkdtemp;\nawait mk(join(tmpdir(), 'taken-'))" },
    { form: 'mkdtempSync passed to another call', source: "wrap('passed-', mkdtempSync)" },
  ])('reports $form as a call whose prefix it cannot read', ({ source }) => {
    expect(scratchCalls(source)).toEqual([{ line: 1, prefix: null }]);
  });

  it('reads no call in a comment or a string', () => {
    const source = "// wraps mkdtempSync(dir) per case\n/* mkdtemp(dir) */\nconst text = 'mkdtempSync(dir)';";
    expect(scratchCalls(source)).toEqual([]);
  });

  it('names the line of each call', () => {
    const source = "import { mkdtempSync } from 'fs';\n\nmkdtempSync(join(tmpdir(), 'first-'));\nmkdtempSync('second-');";
    expect(scratchCalls(source)).toEqual([
      { line: 3, prefix: 'first-' },
      { line: 4, prefix: 'second-' },
    ]);
  });
});

describe('the files the scratch-name guard reads', () => {
  it('walks every depth, leaves out fixtures and keeps to the name pattern it is given', () => {
    const root = mkdtempSync(join(tmpdir(), 'test-scratch-names-walk-'));
    try {
      const names = ['a.test.ts', 'helper.ts', 'helper.mjs', 'helper.cjs', 'notes.md', 'deep/er/b.test.ts', 'fixtures/c.test.ts'];
      for (const name of names) {
        mkdirSync(dirname(join(root, name)), { recursive: true });
        writeFileSync(join(root, name), '');
      }
      const walked = (pattern: RegExp): string[] => sourceFiles(root, pattern).map((f) => relative(root, f)).sort();
      expect(walked(SOURCE_FILE)).toEqual(['a.test.ts', 'deep/er/b.test.ts', 'helper.cjs', 'helper.mjs', 'helper.ts']);
      expect(walked(TEST_FILE)).toEqual(['a.test.ts', 'deep/er/b.test.ts']);
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  });

  it('are the source files under tests/ and the test files under src/, this file aside', () => {
    const files = scannedFiles().map((f) => relative(SDK_ROOT, f));
    expect(files).toEqual(
      expect.arrayContaining([
        'tests/arp/intelligence/behavioral-risk.test.ts',
        'tests/arp/harness/benchmark.ts',
        'tests/local-verify-consumer.mjs',
        'tests/local-verify-consumer.cjs',
        'src/telemetry/relay.test.ts',
        'src/arp/telemetry/signature/wire.test.ts',
      ]),
    );
    expect(files).not.toContain('tests/test-scratch-names.test.ts');
    expect(files).not.toContain('src/telemetry/relay.ts');
  });
});

describe('scratch directories and shell helpers in the tests are named after what they serve', () => {
  it("every mkdtemp call in the SDK's tests passes a prefix the scanner reads", () => {
    const unreadable: string[] = [];
    for (const file of scannedFiles()) {
      for (const call of scratchCalls(readFileSync(file, 'utf-8'))) {
        if (call.prefix === null) unreadable.push(`${relative(SDK_ROOT, file)}:${call.line}`);
      }
    }
    expect(unreadable).toEqual([]);
  });

  it("no mkdtemp prefix in the SDK's tests opens with a short identifier and a number", () => {
    const offenders: string[] = [];
    for (const file of scannedFiles()) {
      for (const call of scratchCalls(readFileSync(file, 'utf-8'))) {
        if (call.prefix !== null && /^[A-Za-z]{1,4}-\d+-/.test(call.prefix)) {
          offenders.push(`${relative(SDK_ROOT, file)}:${call.line}: '${call.prefix}'`);
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

  it.each([
    { file: 'src/telemetry/local-writer.test.ts', prefixes: ['telemetry-local-writer-'] },
    { file: 'src/telemetry/local-writer.mode.test.ts', prefixes: ['telemetry-local-writer-mode-'] },
    { file: 'src/telemetry/relay.test.ts', prefixes: ['telemetry-relay-'] },
    { file: 'src/telemetry/relay-user-agent.test.ts', prefixes: ['telemetry-relay-user-agent-'] },
    {
      file: 'src/client/AIMClient.telemetry.test.ts',
      prefixes: ['aimclient-telemetry-relay-wiring-', 'aimclient-telemetry-auto-joiner-'],
    },
  ])('$file names its scratch directories after the test', ({ file, prefixes }) => {
    expect(scratchPrefixes(readFileSync(join(SDK_ROOT, file), 'utf-8'))).toEqual(prefixes);
  });
});
