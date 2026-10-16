/**
 * `npm run typecheck` type-checks the test files as well as src/.
 *
 * tsconfig.json includes only src/ and excludes every *.test.ts file, and
 * vitest strips types without checking them. The typecheck script, which the
 * CI and release Typecheck steps run, was `tsc --noEmit`, whose file list held
 * none of the files under tests/ and none of the test files under src/.
 * tsconfig.test.json adds them, and the script runs it after the src check.
 */
import { describe, it, expect } from 'vitest';
import * as fs from 'fs';
import * as path from 'path';
import ts from 'typescript';

const PKG_ROOT = path.join(__dirname, '..');

function toPosix(rel: string): string {
  return rel.split(path.sep).join('/');
}

function parseConfig(name: string): ts.ParsedCommandLine {
  const configPath = path.join(PKG_ROOT, name);
  const read = ts.readConfigFile(configPath, ts.sys.readFile);
  expect(read.error, `${name} could not be read`).toBeUndefined();
  return ts.parseJsonConfigFileContent(read.config, ts.sys, PKG_ROOT, undefined, configPath);
}

function listFiles(dir: string, keep: (rel: string) => boolean): string[] {
  const out: string[] = [];
  const walk = (current: string) => {
    for (const entry of fs.readdirSync(current, { withFileTypes: true })) {
      const full = path.join(current, entry.name);
      if (entry.isDirectory()) {
        walk(full);
      } else {
        const rel = toPosix(path.relative(PKG_ROOT, full));
        if (keep(rel)) out.push(rel);
      }
    }
  };
  walk(path.join(PKG_ROOT, dir));
  return out;
}

describe('npm run typecheck covers the tests', () => {
  it('runs the src check and then tsconfig.test.json', () => {
    const pkg = JSON.parse(fs.readFileSync(path.join(PKG_ROOT, 'package.json'), 'utf8'));
    const script = String(pkg.scripts.typecheck);
    const steps = script.split('&&').map((step) => step.trim().split(/\s+/).join(' '));
    expect(steps, `typecheck script: ${script}`).toContain('tsc --noEmit');
    expect(
      steps.some((step) => /^tsc (-p|--project) tsconfig\.test\.json$/.test(step)),
      `typecheck script does not run tsconfig.test.json: ${script}`,
    ).toBe(true);
  });

  it('reads every .ts file under tests/ and every test file under src/, the integration tests included', () => {
    const read = new Set(
      parseConfig('tsconfig.test.json').fileNames.map((f) => toPosix(path.relative(PKG_ROOT, f))),
    );
    const testFiles = [
      ...listFiles('tests', (rel) => rel.endsWith('.ts')),
      ...listFiles('src', (rel) => rel.endsWith('.test.ts')),
    ];
    // Without files in both places, and integration files among them, this
    // check proves nothing.
    expect(testFiles.some((f) => f.startsWith('tests/'))).toBe(true);
    expect(testFiles.some((f) => f.startsWith('src/'))).toBe(true);
    expect(testFiles.some((f) => f.endsWith('.integration.test.ts'))).toBe(true);

    const unread = testFiles.filter((f) => !read.has(f)).sort();
    expect(unread, 'tsconfig.test.json does not read these test files').toEqual([]);
  });

  it('keeps every compiler option of tsconfig.json except rootDir, and emits nothing', () => {
    const main = parseConfig('tsconfig.json').options;
    const test = parseConfig('tsconfig.test.json').options;
    const declared = Object.keys(
      JSON.parse(fs.readFileSync(path.join(PKG_ROOT, 'tsconfig.json'), 'utf8')).compilerOptions,
    );
    expect(declared.length).toBeGreaterThan(0);

    for (const key of declared.filter((k) => k !== 'rootDir')) {
      expect(test[key], `tsconfig.test.json changes ${key}`).toEqual(main[key]);
    }
    expect(test.noEmit).toBe(true);
  });
});
