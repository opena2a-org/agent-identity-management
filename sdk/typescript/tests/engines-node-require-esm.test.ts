/**
 * Issue #836: package.json declared `engines.node` `>=18.0.0`, and the
 * CommonJS build loads `@noble/ed25519`, which ships as an ES module only, with
 * `require()`. npm installed 1.3.1 on Node.js 18.20.8 and 22.11.0 with no
 * `EBADENGINE` warning, and `require('@opena2a/aim-sdk')` then failed there with
 * `ERR_REQUIRE_ESM`.
 *
 * This builds the real tsup.config.ts, in CommonJS only, into its own directory
 * inside the package (so externals resolve from this package's node_modules, as
 * in tests/entry-point-class-identity.test.ts), lists the packages its files
 * load with `require()` or the arp module's `nodeRequire()`, and reads whether
 * each resolves to an ES module. A package loaded with `import()` is not listed.
 *
 *   - The build loads @noble/ed25519, an ES module, with `require()`.
 *   - engines.node admits none of the releases on which that `require()`
 *     failed, and each release on which the package loaded.
 *   - The range check reads `>=`, `^` and `||` as npm does.
 */
import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { existsSync, mkdtempSync, readdirSync, readFileSync, rmSync } from 'node:fs';
import { createRequire, isBuiltin } from 'node:module';
import path from 'node:path';
import { build, type Options } from 'tsup';

import config from '../tsup.config';

const pkgDir = path.resolve(__dirname, '..');
const enginesNode = (
  JSON.parse(readFileSync(path.join(pkgDir, 'package.json'), 'utf-8')) as { engines: { node: string } }
).engines.node;

// Each Node.js release below ran `npm install` of the package packed from the
// source before the fix, then `require('@opena2a/aim-sdk')` and
// `require('@opena2a/aim-sdk/arp')`. On these both failed with ERR_REQUIRE_ESM
// in @noble/ed25519:
const REQUIRE_FAILED = ['18.20.8', '20.18.3', '21.7.3', '22.11.0'];
// and on these both loaded:
const REQUIRE_LOADED = ['20.19.0', '22.12.0', '23.0.0'];

type Version = [number, number, number];

function compare(a: Version, b: Version): number {
  return a[0] - b[0] || a[1] - b[1] || a[2] - b[2];
}

const COMPARATOR = /^(>=|>|<=|<|\^)(\d+)\.(\d+)\.(\d+)$/;

/**
 * Whether `version` satisfies an npm range written as `||` alternatives of
 * space-separated `>=`, `>`, `<=`, `<` and `^` comparators on full versions,
 * `^` on a major above 0. Any other form throws before a version is compared.
 */
function satisfies(version: string, range: string): boolean {
  const alternatives = range.split('||').map((alternative) =>
    alternative
      .trim()
      .split(/\s+/)
      .map((comparator) => {
        const m = COMPARATOR.exec(comparator);
        if (m === null || (m[1] === '^' && m[2] === '0')) {
          throw new Error(`engines range not read by this test: "${range}"`);
        }
        return { op: m[1], base: [Number(m[2]), Number(m[3]), Number(m[4])] as Version };
      }),
  );
  const [major = 0, minor = 0, patch = 0] = version.split('.').map(Number);
  const v: Version = [major, minor, patch];
  return alternatives.some((comparators) =>
    comparators.every(({ op, base }) => {
      const order = compare(v, base);
      switch (op) {
        case '>=':
          return order >= 0;
        case '>':
          return order > 0;
        case '<=':
          return order <= 0;
        case '<':
          return order < 0;
        default:
          // `^X.Y.Z`, X above 0: at least X.Y.Z and below (X+1).0.0.
          return order >= 0 && v[0] === base[0];
      }
    }),
  );
}

/** Every file under `dir`, at any depth. */
function filesUnder(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const p = path.join(dir, entry.name);
    return entry.isDirectory() ? filesUnder(p) : [p];
  });
}

/** Whether Node.js loads `file` as an ES module: `.mjs`, or `.js` whose nearest package.json says `"type": "module"`. */
function isEsModule(file: string): boolean {
  if (file.endsWith('.mjs')) return true;
  if (!file.endsWith('.js')) return false;
  for (let dir = path.dirname(file); ; dir = path.dirname(dir)) {
    const manifest = path.join(dir, 'package.json');
    if (existsSync(manifest)) {
      return (JSON.parse(readFileSync(manifest, 'utf-8')) as { type?: string }).type === 'module';
    }
    if (path.dirname(dir) === dir) return false;
  }
}

const LOAD_CALL = /\b(?:require|nodeRequire)\((['"])([^'"]+)\1\)/g;

let outDir: string;
// The packages the CommonJS build loads with require() or nodeRequire(), and those of them that are ES modules.
let required: string[];
let esModules: string[];

beforeAll(async () => {
  outDir = mkdtempSync(path.join(pkgDir, '.engines-build-'));
  await build({
    ...(config as Options),
    format: ['cjs'],
    outDir,
    dts: false,
    sourcemap: false,
    silent: true,
  });
  const specifiers = new Set<string>();
  for (const file of filesUnder(outDir).filter((f) => f.endsWith('.js'))) {
    for (const m of readFileSync(file, 'utf-8').matchAll(LOAD_CALL)) {
      const specifier = m[2];
      if (!specifier.startsWith('.') && !specifier.startsWith('/') && !isBuiltin(specifier)) {
        specifiers.add(specifier);
      }
    }
  }
  required = [...specifiers].sort();
  const builtRequire = createRequire(path.join(outDir, 'index.js'));
  esModules = required.filter((specifier) => isEsModule(builtRequire.resolve(specifier)));
}, 120_000);

afterAll(() => {
  if (outDir) rmSync(outDir, { recursive: true, force: true });
});

describe('engines.node admits only the Node.js releases that load the CommonJS build (#836)', () => {
  it('the CommonJS build loads @noble/ed25519, an ES module, with require()', () => {
    expect(required).toContain('@noble/ed25519');
    expect(esModules, `ES modules among ${required.join(', ')}`).toContain('@noble/ed25519');
  });

  it('engines.node admits none of the releases on which require() of the package failed', () => {
    const admitted = REQUIRE_FAILED.filter((version) => satisfies(version, enginesNode));
    expect(admitted, `admitted by engines.node "${enginesNode}"`).toEqual([]);
  });

  it('engines.node admits each release on which the package loaded', () => {
    const refused = REQUIRE_LOADED.filter((version) => !satisfies(version, enginesNode));
    expect(refused, `refused by engines.node "${enginesNode}"`).toEqual([]);
  });

  it('the range check reads >=, ^ and || as npm does, and throws on a form it does not read', () => {
    expect(satisfies('18.20.8', '>=18.0.0')).toBe(true);
    expect(satisfies('22.11.0', '>=18.0.0')).toBe(true);
    expect(satisfies('17.9.1', '>=18.0.0')).toBe(false);
    const range = '^20.19.0 || >=22.12.0';
    expect(satisfies('20.18.3', range)).toBe(false);
    expect(satisfies('20.19.0', range)).toBe(true);
    expect(satisfies('20.20.1', range)).toBe(true);
    expect(satisfies('21.7.3', range)).toBe(false);
    expect(satisfies('22.11.0', range)).toBe(false);
    expect(satisfies('22.12.0', range)).toBe(true);
    expect(satisfies('26.12.0', range)).toBe(true);
    expect(satisfies('20.5.0', '>=19.0.0 <20.0.0')).toBe(false);
    expect(satisfies('19.9.0', '>=19.0.0 <20.0.0')).toBe(true);
    expect(() => satisfies('20.19.0', '^20.19.0 || 22.x')).toThrow('engines range not read by this test');
    expect(() => satisfies('20.19.0', '>=20')).toThrow('engines range not read by this test');
  });
});
