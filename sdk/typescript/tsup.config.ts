import { defineConfig } from 'tsup';

export default defineConfig({
  entry: {
    index: 'src/index.ts',
    'arp/index': 'src/arp/index.ts',
    'integrations/express': 'src/integrations/express.ts',
    'integrations/fastify': 'src/integrations/fastify.ts',
  },
  format: ['cjs', 'esm'],
  // The arp module resolves its dual-format `require` via
  // createRequire(import.meta.url); shims provide import.meta.url in the CJS
  // build (and are inert for source that never references them).
  shims: true,
  dts: true,
  splitting: false,
  sourcemap: true,
  clean: true,
  treeshake: true,
  minify: false,
  // @opena2a/atx-verify is ESM-only; keep it external and load it via a dynamic
  // import() at runtime so both the CJS and ESM builds resolve it natively on
  // every supported Node (a static require of an ESM package would throw).
  external: ['express', 'fastify', '@opena2a/atx-verify'],
});
