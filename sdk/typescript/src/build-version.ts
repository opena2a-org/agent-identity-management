/**
 * The version the `aim-arp` CLI prints, which says whether this build is the
 * published release.
 *
 * A build of this repository printed `aim-arp v1.3.1`, the same line as the
 * published 1.3.1, while it carried commands 1.3.1 does not have. The npm
 * release is built by the release workflow on the tag `sdk-ts-v<version>`;
 * tsup.config.ts records that build's `GITHUB_REF_NAME` as
 * `__AIM_SDK_BUILD_REF__`. Any other build (a local `npm run build`, a branch
 * build, a tag of another version or package) prints the version with a
 * source-build qualifier.
 *
 * Only the CLI's own version lines use this. `SDK_VERSION`, the exported
 * `VERSION`, the package `version` and the User-Agent stay the bare version.
 */

import { SDK_VERSION } from './version';

/** Defined by tsup at build time; not defined when the source runs directly (tests). */
declare const __AIM_SDK_BUILD_REF__: string | undefined;

/** The tag prefix the release workflow publishes the npm package from. */
export const RELEASE_TAG_PREFIX = 'sdk-ts-v';

const BUILD_REF: string | undefined =
  typeof __AIM_SDK_BUILD_REF__ === 'string' ? __AIM_SDK_BUILD_REF__ : undefined;

/**
 * `SDK_VERSION` when `buildRef` is the release tag for it, otherwise
 * `<version>+source (built from source; not the published <version>)`.
 */
export function cliVersion(buildRef: string | undefined = BUILD_REF): string {
  if (buildRef === `${RELEASE_TAG_PREFIX}${SDK_VERSION}`) return SDK_VERSION;
  return `${SDK_VERSION}+source (built from source; not the published ${SDK_VERSION})`;
}
