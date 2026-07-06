/**
 * Single source of truth for the @opena2a/aim-sdk package version.
 *
 * Keep this in sync with `version` in package.json on every release
 * (`npm version <patch|minor|major>` updates package.json; update this file in
 * the same commit). It is a standalone leaf module so any part of the SDK can
 * import it without a circular dependency on the barrel `index.ts`.
 */
export const SDK_VERSION = '1.0.0';
