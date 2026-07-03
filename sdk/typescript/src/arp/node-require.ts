/**
 * A `require` that works in both build formats of this package.
 *
 * The interceptors patch the CJS module registry in place (`require('fs')`
 * returns the mutable exports object; ESM namespaces are frozen), so they
 * need a real `require` even when this package is consumed as ESM. A bare
 * `require` reference cannot be used here: esbuild rewrites it to a throwing
 * `__require` shim in the ESM output. `createRequire` resolves against this
 * module's own location (tsup `shims: true` provides `import.meta.url` in the
 * CJS build), so package specifiers like 'js-yaml' resolve from this
 * package's dependency tree, not the consumer's cwd.
 *
 * The fallback covers consumers that re-bundle the ESM output to CJS
 * (import.meta.url becomes undefined there). The base path only affects
 * package-specifier resolution — builtins ('fs', 'child_process', 'http')
 * resolve identically from any base — so falling back to the consumer's cwd
 * keeps interception working and at worst moves 'js-yaml' resolution into
 * the consumer's tree, where config/loader.ts already reports a clear error
 * if it is absent.
 */
import { createRequire } from 'node:module';

function makeRequire(): NodeRequire {
  try {
    return createRequire(import.meta.url);
  } catch {
    return createRequire(process.cwd() + '/');
  }
}

export const nodeRequire: NodeRequire = makeRequire();
