/**
 * ESLint config for @opena2a/aim-sdk (TypeScript).
 *
 * Uses the eslintrc format because the pinned toolchain is ESLint 8 +
 * @typescript-eslint 6 (flat config is the ESLint 9 default). A future bump to
 * ESLint 9 / typescript-eslint 8 should migrate this to `eslint.config.mjs` and
 * also clears the ESLint-8-chain npm audit advisories.
 *
 * This is a lint-only dev tool; it is not part of the published artifact and is
 * not run in CI today. Keep it green so `npm run lint` stays usable.
 */
module.exports = {
  root: true,
  parser: '@typescript-eslint/parser',
  parserOptions: {
    ecmaVersion: 2022,
    sourceType: 'module',
  },
  env: {
    node: true,
    es2022: true,
  },
  plugins: ['@typescript-eslint'],
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/recommended',
  ],
  ignorePatterns: [
    'dist/**',
    'node_modules/**',
    'coverage/**',
    '*.config.ts',
    '*.config.mjs',
    '*.config.cjs',
  ],
  rules: {
    // Type-safety escape hatches are used deliberately at adapter/JS boundaries
    // (untyped daemon bags, dynamic requires); keep them visible but non-fatal.
    '@typescript-eslint/no-explicit-any': 'warn',
    // Adopting lint on a previously-unlinted tree: surface existing unused
    // symbols as warnings (green baseline) so `npm run lint` is usable, while
    // NEW unused symbols still stand out for cleanup. `_`-prefixed are ignored.
    '@typescript-eslint/no-unused-vars': [
      'warn',
      { argsIgnorePattern: '^_', varsIgnorePattern: '^_', caughtErrors: 'none' },
    ],
    // Empty catch blocks are the intentional "errors here must not block the
    // pipeline" pattern throughout the engine; other empty blocks still error.
    'no-empty': ['error', { allowEmptyCatch: true }],
    // `const self = this` appears in a few closures; low-value to forbid.
    '@typescript-eslint/no-this-alias': 'off',
    // Pre-existing regex escapes flagged as redundant — surface, don't block;
    // "fixing" a regex escape blindly can change semantics.
    'no-useless-escape': 'warn',
    // require(...) is used intentionally for lazy/optional loads in dual-format
    // code paths; the surrounding code documents each one.
    '@typescript-eslint/no-var-requires': 'off',
    // Keep the default type bans ({}, Object, String, ...) but allow `Function`:
    // it is used deliberately for `Record<string, Function>` fs-module patching
    // and `as Function` casts around Node crypto overloads.
    '@typescript-eslint/ban-types': [
      'error',
      { types: { Function: false }, extendDefaults: true },
    ],
  },
  overrides: [
    {
      files: ['**/*.test.ts', 'tests/**/*.ts'],
      rules: {
        '@typescript-eslint/no-explicit-any': 'off',
        '@typescript-eslint/no-non-null-assertion': 'off',
      },
    },
  ],
};
