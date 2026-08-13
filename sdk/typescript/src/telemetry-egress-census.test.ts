import { describe, it, expect } from 'vitest';
import { readFileSync, readdirSync, statSync } from 'fs';
import { join, relative } from 'path';

/**
 * Every module that sends data to the OpenA2A registry, and whether the master
 * opt-out reaches it.
 *
 * This pins the CLASS, not an instance. Two consent defects in this SDK were the
 * same shape: a channel was added, it POSTed to the registry, and nobody wired it
 * to the opt-out — while the user-facing text kept promising the opt-out covered
 * everything. Per-channel tests cannot catch that, because the failure is a
 * channel nobody wrote a test for. The only thing that catches it is enumerating
 * the channels from the source and asserting the set.
 *
 * The sweep that found the third channel missed the fourth by scoping its grep to
 * `src/arp/`. This test reads the whole of `src/` for that reason.
 *
 * If this test fails because you added a channel: wire it to `isOptedOut()`, or
 * add it to EXCLUDED with the reason and the user-facing text that discloses it.
 * Do not widen the matcher.
 */

const SRC = join(__dirname);
const REGISTRY_HOST = 'api.oa2a.org';

/**
 * Channels deliberately outside the ARP master opt-out.
 *
 * Listing one here is a claim that its exclusion is disclosed to the user. Both
 * README.md and the first-run disclosure state that the causal-denial relay has
 * its own switch and is not governed by this opt-out.
 */
const EXCLUDED: Record<string, string> = {
  'telemetry/relay.ts':
    'AIM client causal-denial relay. Own switch (telemetry.enabled + relay.enabled), ' +
    'default off. Exclusion is stated in README.md and in the first-run disclosure. ' +
    'Tracked for review as a joint DPO/CA question.',
};

/**
 * Channels whose gate lives at the construction site rather than inside the
 * module. Listing one here is not an exemption: the guard named below is
 * asserted to still exist, so deleting it fails this test.
 */
const GATED_AT_CALLSITE: Record<string, { site: string; guard: RegExp }> = {
  'arp/telemetry/gtin.ts': {
    site: 'arp/index.ts',
    // The forwarder is never constructed while opted out.
    guard: /this\.config\.gtin\?\.enabled\s*&&\s*!optedOut/,
  },
};

function walk(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    const p = join(dir, entry);
    if (statSync(p).isDirectory()) {
      if (entry === 'node_modules' || entry === 'dist') continue;
      walk(p, out);
    } else if (entry.endsWith('.ts') && !entry.endsWith('.test.ts')) {
      out.push(p);
    }
  }
  return out;
}

/**
 * Strip comments before looking for senders. `src/index.ts` documents Express and
 * Fastify integration with `app.post(...)` examples in JSDoc; counting those as
 * telemetry egress made the census report a module that transmits nothing.
 */
function stripComments(src: string): string {
  return src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, '');
}

/**
 * Any call that puts bytes on the wire, including an injected or aliased sender.
 *
 * `telemetry/relay.ts` transmits via `this.fetchImpl(...)`, so a matcher looking
 * only for a literal `fetch(` does not see it — and a channel is invisible to
 * this census exactly when it is hardest to notice by hand. Match any
 * fetch-shaped identifier, bare or as a member call.
 */
const SENDER = /(^|[^A-Za-z0-9_$.])fetch[A-Za-z0-9_$]*\s*\(|\.\s*fetch[A-Za-z0-9_$]*\s*\(|\baxios\.|\bhttps?\.request\s*\(/;

/** Modules that actually transmit to the registry, as opposed to naming its URL. */
function egressModules(): string[] {
  const hits: string[] = [];
  for (const file of walk(SRC)) {
    const code = stripComments(readFileSync(file, 'utf-8'));
    const sends = SENDER.test(code);
    if (!sends) continue;
    const reachesRegistry =
      code.includes(REGISTRY_HOST) ||
      /registryUrl|REGISTRY_URL|this\.endpoint/.test(code);
    if (reachesRegistry) hits.push(relative(SRC, file));
  }
  return hits.sort();
}

describe('telemetry egress census', () => {
  it('finds the channels at all — the census is not silently empty', () => {
    // Without this, every assertion below passes vacuously if walk() or the
    // matcher breaks and the census returns nothing.
    const mods = egressModules();
    expect(mods.length).toBeGreaterThanOrEqual(3);
  });

  it('every registry egress module consults the master opt-out, or is a disclosed exclusion', () => {
    const offenders: string[] = [];

    for (const mod of egressModules()) {
      if (mod in EXCLUDED) continue;
      if (mod in GATED_AT_CALLSITE) continue; // its guard is asserted separately below
      const src = readFileSync(join(SRC, mod), 'utf-8');
      if (!src.includes('isOptedOut')) offenders.push(mod);
    }

    expect(
      offenders,
      `These modules send to ${REGISTRY_HOST} without consulting isOptedOut():\n` +
        offenders.map(o => `  - ${o}`).join('\n') +
        '\n\nWire each to isOptedOut(), or add it to EXCLUDED with the reason and ' +
        'the user-facing text that discloses it.',
    ).toEqual([]);
  });

  it('call-site gates still exist — a channel listed as gated elsewhere really is', () => {
    for (const [mod, { site, guard }] of Object.entries(GATED_AT_CALLSITE)) {
      const siteSrc = readFileSync(join(SRC, site), 'utf-8');
      expect(
        guard.test(siteSrc),
        `${mod} is recorded as gated at ${site}, but that guard is gone. ` +
          `Either restore it, or wire ${mod} to isOptedOut() directly.`,
      ).toBe(true);
    }
  });

  it('the known ARP channels are present and gated — guards against a matcher that stops matching', () => {
    const mods = egressModules();
    // Positive control: if these three ever drop out of the census, the matcher
    // has broken and the assertion above is no longer measuring anything.
    for (const expected of [
      'arp/intelligence/runtime-twin.ts',
      'arp/telemetry/gtin.ts',
      'arp/telemetry/signature/emitter.ts',
    ]) {
      expect(mods, `${expected} fell out of the census`).toContain(expected);
    }
  });

  it('every EXCLUDED entry still exists and still sends — stale exemptions do not linger', () => {
    const mods = egressModules();
    for (const mod of Object.keys(EXCLUDED)) {
      expect(
        mods,
        `${mod} is exempted but no longer sends to the registry. Remove the exemption.`,
      ).toContain(mod);
    }
  });
});
