import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';
import * as sdk from '../src';

/**
 * The root README says the one-line `secure("name")` shape works only in the
 * SDKs that have a `secure()` entry point.
 *
 * Below the Python quick start, the README said "The same one-line shape works
 * in Java and TypeScript". Java's `AIMClient.secure(name)` exists; neither
 * TypeScript package has a `secure()`: `@opena2a/aim-core` builds an `AIMCore`
 * and checks capabilities against a local policy, and this package's
 * `AIMClient` registers an agent and verifies each action with separate calls.
 *
 *   C1  every SDK the README links from a "one-line shape" sentence has a
 *       `secure()` entry point, and the README still names at least one;
 *   C2  the pre-fix sentence, and the same claim with the SDKs in the other
 *       order, are refused for TypeScript.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');
const readme = readFileSync(join(REPO_ROOT, 'README.md'), 'utf-8');
const JAVA_CLIENT = join(
  REPO_ROOT,
  'sdk',
  'java',
  'src',
  'main',
  'java',
  'org',
  'opena2a',
  'aim',
  'client',
  'AIMClient.java'
);

/** Whether each SDK section the README links to has a `secure()` entry point. */
const HAS_SECURE: Record<string, () => boolean> = {
  java: () => /public static AIMClient secure\(/.test(readFileSync(JAVA_CLIENT, 'utf-8')),
  typescript: () =>
    'secure' in sdk || typeof (sdk.AIMClient as unknown as { secure?: unknown }).secure === 'function',
};

/** Section anchors linked from each sentence that says the one-line shape works there. */
function oneLineShapeClaims(text: string): string[] {
  const anchors: string[] = [];
  for (const sentence of text.split(/(?<=[.?!])\s+/)) {
    if (!/one-line shape/i.test(sentence)) continue;
    for (const m of sentence.matchAll(/\]\(#([\w-]+)\)/g)) anchors.push(m[1]);
  }
  return anchors;
}

/** Anchors a "one-line shape" sentence links whose SDK has no `secure()`. */
function unsupportedClaims(text: string): string[] {
  return oneLineShapeClaims(text).filter((anchor) => !(HAS_SECURE[anchor]?.() ?? false));
}

describe('root README one-line shape claim', () => {
  it('C1 names only SDKs that have a secure() entry point', () => {
    expect(oneLineShapeClaims(readme), 'the README still says where the one-line shape works').not.toEqual([]);
    expect(unsupportedClaims(readme)).toEqual([]);
  });

  it('C2 the pre-fix sentence and a reordered near miss are refused for TypeScript', () => {
    expect(HAS_SECURE.java(), 'the Java client has AIMClient.secure(name)').toBe(true);
    expect(HAS_SECURE.typescript(), 'this package exports no secure()').toBe(false);

    const preFix =
      'New to AIM? The [SDK quickstart tutorial](https://opena2a.org/docs/tutorials/sdk-quickstart) ' +
      'walks through this end to end. The same one-line shape works in [Java](#java) and [TypeScript](#typescript).';
    expect(unsupportedClaims(preFix)).toEqual(['typescript']);

    const reordered = 'The one-line shape also works in [TypeScript](#typescript) and [Java](#java).';
    expect(unsupportedClaims(reordered)).toEqual(['typescript']);
  });
});
