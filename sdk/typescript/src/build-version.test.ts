import { describe, it, expect } from 'vitest';
import { cliVersion, RELEASE_TAG_PREFIX } from './build-version';
import { SDK_VERSION } from './version';

/**
 * A build of this repository printed `aim-arp v1.3.1`, the same line as the
 * published 1.3.1, while carrying commands 1.3.1 does not have (`init`). The
 * CLI's version line now names a build the release job did not make on the
 * package's own tag. The build reference is injected here; under vitest the
 * tsup constant is not defined at all, which is the case of the last test.
 */
describe('the CLI version names a build the release job did not make', () => {
  const qualified = `${SDK_VERSION}+source (built from source; not the published ${SDK_VERSION})`;

  it('is the bare version on the release tag for this version', () => {
    expect(cliVersion(`${RELEASE_TAG_PREFIX}${SDK_VERSION}`)).toBe(SDK_VERSION);
  });

  it('is qualified when the build had no reference', () => {
    expect(cliVersion('')).toBe(qualified);
  });

  it('is qualified on a branch build', () => {
    expect(cliVersion('main')).toBe(qualified);
  });

  it('is qualified on the release tag of another version', () => {
    expect(cliVersion(`${RELEASE_TAG_PREFIX}0.0.0`)).toBe(qualified);
  });

  it('is qualified on a tag of another package that carries the same version', () => {
    expect(cliVersion(`platform-v${SDK_VERSION}`)).toBe(qualified);
    expect(cliVersion(`sdk-java-v${SDK_VERSION}`)).toBe(qualified);
  });

  it('is qualified on a tag that only starts with the release tag', () => {
    expect(cliVersion(`${RELEASE_TAG_PREFIX}${SDK_VERSION}-rc1`)).toBe(qualified);
  });

  it('is qualified when the build constant is not defined, as in this test run', () => {
    expect(cliVersion()).toBe(qualified);
  });

  it('carries no em dash', () => {
    expect(cliVersion('')).not.toContain('—');
  });
});
