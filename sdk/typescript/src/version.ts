/**
 * Single source of truth for the @opena2a/aim-sdk package version.
 *
 * Keep this in sync with `version` in package.json on every release
 * (`npm version <patch|minor|major>` updates package.json; update this file in
 * the same commit). It is a standalone leaf module so any part of the SDK can
 * import it without a circular dependency on the barrel `index.ts`.
 *
 * `version.test.ts` asserts that equality, because this comment asking a human
 * to remember is not a mechanism and 1.2.0 is where it stopped being remembered:
 * package.json read 1.2.0 while this file still read 1.1.0, so the package would
 * have published reporting a version it was not. The test fails the build
 * instead.
 */
export const SDK_VERSION = '1.3.1';
