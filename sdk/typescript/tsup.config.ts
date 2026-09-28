import { defineConfig } from 'tsup';

export default defineConfig({
  entry: {
    index: 'src/index.ts',
    'arp/index': 'src/arp/index.ts',
    'arp/cli/aim-arp': 'src/arp/cli/aim-arp.ts',
    'integrations/express': 'src/integrations/express.ts',
    'integrations/fastify': 'src/integrations/fastify.ts',
  },
  format: ['cjs', 'esm'],
  // The arp module resolves its dual-format `require` via
  // createRequire(import.meta.url); shims provide import.meta.url in the CJS
  // build (and are inert for source that never references them).
  shims: true,
  dts: true,
  // Shared chunks, in both formats: `.`, `/arp`, `/express` and `/fastify`
  // each bundled their own copy of AIMClient and the error classes, so an
  // `ActionDeniedError` thrown through `/express` was not `instanceof` the
  // root export's class. With splitting, every entry of one format imports
  // the same chunk. The ESM/CJS boundary still yields two copies (the dual
  // package hazard); README documents `error.code` for that case.
  // tests/entry-point-class-identity.test.ts pins it.
  splitting: true,
  sourcemap: true,
  // Maps ship for readable stack traces, but WITHOUT embedded sources:
  // README.md promises "source is not shipped in the npm package", and
  // esbuild's default sourcesContent put ~2.6 MB of the full TypeScript
  // source (doc comments included) into every published tarball.
  // tests/packaging-sourcemaps.test.ts pins the packed artifact.
  esbuildOptions(options) {
    options.sourcesContent = false;
  },
  clean: true,
  treeshake: true,
  minify: false,
  // Preserve class/function names in the CJS output so error-class
  // `constructor.name` matches the ESM build (esbuild otherwise drops the
  // inferred name of a class assigned into the CJS exports object).
  keepNames: true,
  // @opena2a/atx-verify is ESM-only; keep it external and load it via a dynamic
  // import() at runtime so both the CJS and ESM builds resolve it natively on
  // every supported Node (a static require of an ESM package would throw).
  external: ['express', 'fastify', '@opena2a/atx-verify'],
});
