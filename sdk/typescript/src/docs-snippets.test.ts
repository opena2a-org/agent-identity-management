/**
 * The TypeScript examples in the repository's docs/ pages compile against the
 * SDK they import (#680). Every fenced typescript block under docs/ that
 * imports from '@opena2a/aim-sdk' is type-checked against this package's
 * source: an import name the entry point does not export, a type used as a
 * value, or a call that does not match the exported signature fails here.
 */

import { describe, it, expect, beforeAll } from 'vitest';
import { readdirSync, readFileSync, statSync } from 'fs';
import { join, relative, sep } from 'path';
import * as ts from 'typescript';

const SDK_ROOT = join(__dirname, '..');
const REPO_ROOT = join(SDK_ROOT, '..', '..');
const DOCS_ROOT = join(REPO_ROOT, 'docs');
const SNIPPET_DIR = join(SDK_ROOT, '.doc-snippets');

// The package's entry points, as package.json "exports" maps them.
const ENTRY_POINTS: Record<string, string> = {
  '@opena2a/aim-sdk': 'src/index.ts',
  '@opena2a/aim-sdk/arp': 'src/arp/index.ts',
  '@opena2a/aim-sdk/express': 'src/integrations/express.ts',
  '@opena2a/aim-sdk/fastify': 'src/integrations/fastify.ts',
};

const SDK_IMPORT = /from\s+['"]@opena2a\/aim-sdk(?:\/[a-z]+)?['"]/;

interface Snippet {
  doc: string; // repository-relative markdown path
  line: number; // 1-based line of the opening fence
  code: string;
  fileName: string; // virtual .ts path the compiler sees
}

function markdownFiles(dir: string): string[] {
  const out: string[] = [];
  for (const name of readdirSync(dir)) {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) out.push(...markdownFiles(path));
    else if (name.endsWith('.md')) out.push(path);
  }
  return out;
}

function sdkSnippets(): Snippet[] {
  const snippets: Snippet[] = [];
  for (const path of markdownFiles(DOCS_ROOT).sort()) {
    const doc = relative(REPO_ROOT, path).split(sep).join('/');
    const lines = readFileSync(path, 'utf-8').split('\n');
    let start = -1;
    for (let i = 0; i < lines.length; i++) {
      if (start < 0 && /^```(?:typescript|ts)\s*$/.test(lines[i])) {
        start = i;
      } else if (start >= 0 && /^```\s*$/.test(lines[i])) {
        const code = lines.slice(start + 1, i).join('\n');
        if (SDK_IMPORT.test(code)) {
          const fileName = join(SNIPPET_DIR, `${doc.replace(/[/.]/g, '_')}_L${start + 1}.ts`);
          snippets.push({ doc, line: start + 1, code, fileName });
        }
        start = -1;
      }
    }
  }
  return snippets;
}

const snippets = sdkSnippets();
let program: ts.Program;

beforeAll(() => {
  const options: ts.CompilerOptions = {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ESNext,
    moduleResolution: ts.ModuleResolutionKind.Bundler,
    lib: ['lib.es2022.d.ts'],
    types: ['node'],
    typeRoots: [join(SDK_ROOT, 'node_modules', '@types')],
    strict: true,
    esModuleInterop: true,
    resolveJsonModule: true,
    skipLibCheck: true,
    noEmit: true,
    baseUrl: SDK_ROOT,
    paths: Object.fromEntries(
      Object.entries(ENTRY_POINTS).map(([specifier, file]) => [specifier, [file]])
    ),
  };
  const virtual = new Map(snippets.map((s) => [s.fileName, s.code]));
  const host = ts.createCompilerHost(options, true);
  const { getSourceFile, fileExists, readFile } = host;
  host.fileExists = (f) => virtual.has(f) || fileExists.call(host, f);
  host.readFile = (f) => virtual.get(f) ?? readFile.call(host, f);
  host.getSourceFile = (f, languageVersion, onError, shouldCreate) => {
    const code = virtual.get(f);
    return code !== undefined
      ? ts.createSourceFile(f, code, languageVersion, true, ts.ScriptKind.TS)
      : getSourceFile.call(host, f, languageVersion, onError, shouldCreate);
  };
  program = ts.createProgram(
    [...snippets.map((s) => s.fileName), ...Object.values(ENTRY_POINTS).map((f) => join(SDK_ROOT, f))],
    options,
    host
  );
}, 120_000);

describe('TypeScript examples in docs/ compile against the SDK (#680)', () => {
  it('finds the SDK examples in the PQC, A2A and direct API pages', () => {
    const docs = new Set(snippets.map((s) => s.doc));
    expect([...docs]).toEqual(
      expect.arrayContaining(['docs/api/direct-api-usage.md', 'docs/guides/A2A.md', 'docs/guides/PQC.md'])
    );
  });

  it('imports only names the SDK entry point exports', () => {
    const checker = program.getTypeChecker();
    const missing: string[] = [];
    for (const s of snippets) {
      const sf = program.getSourceFile(s.fileName)!;
      for (const stmt of sf.statements) {
        if (!ts.isImportDeclaration(stmt) || !ts.isStringLiteral(stmt.moduleSpecifier)) continue;
        const entry = ENTRY_POINTS[stmt.moduleSpecifier.text];
        const bindings = stmt.importClause?.namedBindings;
        if (!entry || !bindings || !ts.isNamedImports(bindings)) continue;
        const entryFile = program.getSourceFile(join(SDK_ROOT, entry))!;
        const exported = new Set(
          checker.getExportsOfModule(checker.getSymbolAtLocation(entryFile)!).map((sym) => sym.name)
        );
        for (const el of bindings.elements) {
          const name = (el.propertyName ?? el.name).text;
          if (!exported.has(name)) missing.push(`${s.doc}:${s.line} ${name} from ${stmt.moduleSpecifier.text}`);
        }
      }
    }
    expect(missing).toEqual([]);
  });

  it('type-checks every example against the exported signatures', () => {
    const errors: string[] = [];
    for (const s of snippets) {
      const sf = program.getSourceFile(s.fileName)!;
      const diagnostics = [...program.getSyntacticDiagnostics(sf), ...program.getSemanticDiagnostics(sf)];
      for (const d of diagnostics) {
        const { line } = sf.getLineAndCharacterOfPosition(d.start ?? 0);
        // Report the markdown line: the code starts one line after the fence.
        const message = ts.flattenDiagnosticMessageText(d.messageText, ' ');
        errors.push(`${s.doc}:${s.line + 1 + line} TS${d.code} ${message}`);
      }
    }
    expect(errors).toEqual([]);
  });
});
