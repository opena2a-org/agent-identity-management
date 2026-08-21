import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';
import { SDK_VERSION } from './version';

/**
 * `SDK_VERSION` is what `require('@opena2a/aim-sdk').VERSION` returns, and
 * `package.json` is what npm publishes under. When they disagree, the installed
 * package reports a version that was never released -- provenance a caller reads
 * as fact about the artifact in their hand.
 *
 * `version.ts` is a hand-maintained literal whose own docstring says to keep it
 * in sync on every release. That instruction was followed for 1.1.0 and missed
 * for 1.2.0: package.json said 1.2.0 while the export still said 1.1.0, and the
 * 1.2.0 release test caught it in the pre-flight rather than after publish.
 *
 * A comment asking a human to remember is not a mechanism. This is.
 */
describe('the version the SDK reports is the version npm published', () => {
  const pkg = JSON.parse(
    readFileSync(join(__dirname, '..', 'package.json'), 'utf-8'),
  ) as { version: string };

  it('SDK_VERSION equals package.json version', () => {
    expect(SDK_VERSION).toBe(pkg.version);
  });

  it('and package.json actually carries a version, so this is not vacuous', () => {
    // Guards the oracle: if the read silently produced undefined, the assertion
    // above would compare undefined to undefined on a broken build.
    expect(pkg.version).toMatch(/^\d+\.\d+\.\d+/);
  });
});
