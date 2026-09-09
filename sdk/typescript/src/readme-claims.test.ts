/**
 * Pins README/CHANGELOG claims to delivered behaviour (AIM-11 items 3, 7, and
 * the release invariant).
 *
 * Item 3 resolution: the SDK implements no retry logic (measured: no retry
 * loop, no `retries` config key), so the README must not claim automatic
 * retries. If retries are ever implemented, rewrite the README and this pin
 * together, in the same release — a public claim and the code must never
 * disagree.
 */

import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';

const ROOT = join(__dirname, '..');
const readme = readFileSync(join(ROOT, 'README.md'), 'utf-8');
const changelog = readFileSync(join(ROOT, 'CHANGELOG.md'), 'utf-8');
const clientSrc = readFileSync(join(ROOT, 'src', 'client', 'AIMClient.ts'), 'utf-8');
const typesSrc = readFileSync(join(ROOT, 'src', 'types', 'index.ts'), 'utf-8');

describe('README claims match the code (item 3)', () => {
  it('AIM-11.AC3 README does not claim automatic retries the client does not implement', () => {
    // The delivered client has no retry loop and no retry configuration...
    expect(clientSrc).not.toMatch(/exponential backoff/i);
    expect(typesSrc).not.toMatch(/\bretries\??:/);
    // ...so the README must not promise one.
    expect(readme).not.toMatch(/Automatic Retries/);
    expect(readme).not.toMatch(/Built-in retry logic/);
  });
});

describe('README delegation window claim (item 7)', () => {
  it('AIM-11.AC7 README documents the enforced not-yet-valid lower bound', () => {
    expect(readme).toMatch(/not yet valid|before its signed .?createdAt|before `createdAt`/i);
  });
});

describe('CHANGELOG release invariant (AIM-11)', () => {
  it('AIM-11.AC11 the Unreleased section carries an entry per shipped P2 item', () => {
    const unreleased = changelog.split(/^## \[Unreleased\]/m)[1]?.split(/^## \[/m)[0] ?? '';
    // One recognizable marker per shipped item (1-9 of the release-test P2 set).
    expect(unreleased).toMatch(/RFC 6749/); // item 1: token shape
    expect(unreleased).toMatch(/\/oauth\/token/); // item 2: typed token errors
    expect(unreleased).toMatch(/Automatic Retries|retry/i); // item 3: claim corrected
    expect(unreleased).toMatch(/Retry-After/); // item 4
    expect(unreleased).toMatch(/403/); // item 5: telemetry on wire 403
    expect(unreleased).toMatch(/ECONNREFUSED|errno/); // item 6: network context
    expect(unreleased).toMatch(/createdAt/); // item 7: delegation lower bound
    expect(unreleased).toMatch(/OPENA2A_REGISTRY_URL/); // item 8: GTIN destination
    expect(unreleased).toMatch(/AIM_AGENT_ID|missing variable/); // item 9: loader naming
  });
});
