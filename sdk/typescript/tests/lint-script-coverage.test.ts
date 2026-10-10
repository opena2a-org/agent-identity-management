/**
 * `npm run lint` lints every TypeScript file in the package that ESLint does
 * not ignore, not only src/.
 *
 * The script was `eslint src --ext .ts`, so scripts/ and tests/ were never
 * linted by it: `npm run lint` reported 23 warnings while `npx eslint .`
 * reported 31, the other 8 in files under scripts/ and tests/.
 *
 * This reads the lint script's targets and `--ext` list, lists every `.ts`
 * file ESLint does not ignore (ESLint's own `isPathIgnored`, so the
 * .eslintrc.cjs ignore patterns apply), and requires each one to be under a
 * target.
 */
import { describe, it, expect } from 'vitest';
import { ESLint } from 'eslint';
import * as fs from 'fs';
import * as path from 'path';

const PKG_ROOT = path.join(__dirname, '..');

// ESLint 8 options that take a value; the value is not a lint target.
const VALUE_OPTIONS = new Set([
  '-c', '--config', '-f', '--format', '-o', '--output-file', '--max-warnings',
  '--ignore-path', '--ignore-pattern', '--parser', '--parser-options', '--plugin',
  '--rule', '--rulesdir', '--resolve-plugins-relative-to', '--cache-location',
  '--cache-strategy', '--env', '--global',
]);

function parseLintScript(script: string): { targets: string[]; extensions: string[] } {
  const tokens = script.trim().split(/\s+/);
  expect(tokens[0], `lint script does not run eslint: ${script}`).toBe('eslint');
  const targets: string[] = [];
  const extensions: string[] = [];
  for (let i = 1; i < tokens.length; i++) {
    const token = tokens[i];
    if (token === '--ext') {
      extensions.push(...tokens[++i].split(','));
    } else if (token.startsWith('--ext=')) {
      extensions.push(...token.slice('--ext='.length).split(','));
    } else if (VALUE_OPTIONS.has(token)) {
      i++;
    } else if (!token.startsWith('-')) {
      targets.push(token);
    }
  }
  return { targets, extensions };
}

function listTsFiles(dir: string, out: string[] = []): string[] {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (entry.name === 'node_modules' || entry.name === '.git') continue;
    const p = path.join(dir, entry.name);
    if (entry.isDirectory()) listTsFiles(p, out);
    else if (entry.name.endsWith('.ts')) out.push(p);
  }
  return out;
}

function isUnderTarget(relPath: string, target: string): boolean {
  const t = path.normalize(target).replace(/\/+$/, '');
  return t === '.' || relPath === t || relPath.startsWith(`${t}/`);
}

describe('npm run lint coverage', () => {
  it('lints .ts files', () => {
    const pkg = JSON.parse(fs.readFileSync(path.join(PKG_ROOT, 'package.json'), 'utf8'));
    const { extensions } = parseLintScript(pkg.scripts.lint);
    expect(extensions.map((e) => (e.startsWith('.') ? e : `.${e}`))).toContain('.ts');
  });

  it('targets every .ts file ESLint does not ignore, including scripts/ and tests/', async () => {
    const pkg = JSON.parse(fs.readFileSync(path.join(PKG_ROOT, 'package.json'), 'utf8'));
    const { targets } = parseLintScript(pkg.scripts.lint);
    expect(targets.length, `lint script names no target: ${pkg.scripts.lint}`).toBeGreaterThan(0);

    const eslint = new ESLint({ cwd: PKG_ROOT });
    const linted: string[] = [];
    for (const file of listTsFiles(PKG_ROOT)) {
      if (!(await eslint.isPathIgnored(file))) linted.push(path.relative(PKG_ROOT, file));
    }
    // The tree has .ts files outside src/; without them this check proves nothing.
    expect(linted.some((f) => f.startsWith('scripts/'))).toBe(true);
    expect(linted.some((f) => f.startsWith('tests/'))).toBe(true);

    const uncovered = linted.filter((f) => !targets.some((t) => isUnderTarget(f, t)));
    expect(uncovered, `\`${pkg.scripts.lint}\` does not lint these files`).toEqual([]);
  }, 30_000);
});
