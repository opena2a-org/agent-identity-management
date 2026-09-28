/**
 * Issue #334 item 1: every entry point (`.`, `/arp`, `/express`, `/fastify`)
 * bundled its own copy of AIMClient and the error classes, so an
 * `ActionDeniedError` thrown through `/express` was not `instanceof` the class
 * a consumer imported from the root. tsup `splitting: true` puts the shared
 * code in chunks every entry of one format imports.
 *
 * This builds the real tsup.config.ts into its own directory inside the
 * package (so externals resolve from this package's node_modules, and so it
 * never races tests/packaging-sourcemaps.test.ts, which rebuilds dist/), then
 * loads the built entries the way a consumer does.
 */
import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { createRequire } from 'node:module';
import { mkdtempSync, rmSync } from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { build, type Options } from 'tsup';

import config from '../tsup.config';

const pkgDir = path.resolve(__dirname, '..');
const require = createRequire(import.meta.url);

let outDir: string;

type Mod = Record<string, any>;

beforeAll(async () => {
  outDir = mkdtempSync(path.join(pkgDir, '.identity-build-'));
  await build({
    ...(config as Options),
    outDir,
    dts: false,
    sourcemap: false,
    silent: true,
  });
}, 120_000);

afterAll(() => {
  if (outDir) rmSync(outDir, { recursive: true, force: true });
});

const ENTRIES = ['integrations/express', 'integrations/fastify'];
const SHARED = ['AIMClient', 'AIMError', 'ActionDeniedError', 'AuthenticationError', 'RateLimitError'];

function assertShared(root: Mod, other: Mod, entry: string): void {
  for (const name of SHARED) {
    expect(other[name], `${entry} exports ${name}`).toBeTypeOf('function');
    expect(other[name] === root[name], `${entry}.${name} is the root's ${name}`).toBe(true);
  }
  const thrown = new other.ActionDeniedError('denied');
  expect(thrown instanceof root.ActionDeniedError).toBe(true);
  expect(thrown instanceof root.AIMError).toBe(true);
}

describe('entry points share one class identity (#334)', () => {
  it('tsup.config.ts builds with splitting enabled', () => {
    expect((config as Options).splitting).toBe(true);
  });

  it('CommonJS: /express and /fastify export the root\'s classes', () => {
    const root: Mod = require(path.join(outDir, 'index.js'));
    for (const entry of ENTRIES) {
      assertShared(root, require(path.join(outDir, `${entry}.js`)), entry);
    }
    // /arp loads alongside the root without a second copy of anything it shares.
    expect(() => require(path.join(outDir, 'arp/index.js'))).not.toThrow();
  });

  it('ESM: /express and /fastify export the root\'s classes', async () => {
    const root: Mod = await import(pathToFileURL(path.join(outDir, 'index.mjs')).href);
    for (const entry of ENTRIES) {
      const mod: Mod = await import(pathToFileURL(path.join(outDir, `${entry}.mjs`)).href);
      assertShared(root, mod, entry);
    }
    await expect(import(pathToFileURL(path.join(outDir, 'arp/index.mjs')).href)).resolves.toBeDefined();
  });
});
