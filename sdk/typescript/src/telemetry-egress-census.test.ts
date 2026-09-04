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

  // --- Caller-initiated functional clients -------------------------------------
  //
  // These are not observational channels. Each transmits only when the caller
  // invokes it, to an endpoint the caller supplies, carrying the payload the
  // caller passed in. The master opt-out governs data WE collect and send about
  // the user; switching it on must not silently break the API calls the user
  // asked the SDK to make. That is a different consent question, and answering it
  // with isOptedOut() would be the wrong control.
  //
  // They are listed rather than filtered out so the census still SEES them: if one
  // of these ever grows an observational side-channel, it stays in scope and the
  // reason below stops being true.
  'client/AIMClient.ts':
    'The AIM API client. Caller-supplied base URL, caller-initiated requests. This is ' +
    'the SDK performing the operation it was asked to perform.',
  'a2a/A2AClient.ts':
    'A2A protocol client. Caller-initiated messages to a peer agent the caller names.',
  'auth/oauth.ts':
    'OAuth token exchange against the issuer the caller configured. Required for the ' +
    'authenticated calls the caller is making.',
  'secrets/SecretsClient.ts':
    'Secrets backend client. Caller-supplied endpoint, caller-initiated reads.',
  'local/CrlCache.ts':
    'Revocation list fetch. Security-critical input to a verification decision: ' +
    'suppressing it would fail OPEN on revoked credentials, which is the opposite ' +
    'of what an opt-out should do.',
  'arp/proxy/forward.ts':
    'ARP CLI proxy forwarder. Forwards the caller traffic it was explicitly started ' +
    'to proxy, to the upstream the operator configured.',
};

/**
 * Channels whose gate lives at the construction site rather than inside the
 * module. Listing one here is not an exemption: the guard named below is
 * asserted to still exist, so deleting it fails this test.
 */
const GATED_AT_CALLSITE: Record<string, { site: string; guard: RegExp }> = {
  'arp/intelligence/adapters.ts': {
    site: 'arp/intelligence/coordinator.ts',
    // L2 constructs an adapter only when the operator switched it on AND named a
    // destination. Both halves matter: `enabled === true` (not `!== false`) so an
    // absent value from a shallow-merged partial config means off, and an explicit
    // `adapter` so no destination is ever chosen from ambient environment state.
    // Deleting either half re-opens the default-on vendor egress this guard exists
    // to prevent, and fails this test.
    guard: /this\.config\.enabled\s*===\s*true\s*&&\s*this\.config\.adapter/,
  },
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
/**
 * A module transmits if it calls fetch/axios, or calls `.request(` on anything.
 *
 * The trailing alternative is deliberately broad. The previous pattern required the
 * literal `https.request(` or `http.request(`, and the L2 vendor adapter calls
 * `mod.request(...)` where `const mod = useHttp ? nodeRequire('http') : https` — so
 * aliasing the module through a variable made the biggest egress in the package
 * invisible to its own census. Matching any `.request(` costs a few false positives,
 * which are cheap to triage into EXCLUDED, and closes an evasion that was not
 * deliberate but worked perfectly.
 */
const SENDER = /(^|[^A-Za-z0-9_$.])fetch[A-Za-z0-9_$]*\s*\(|\.\s*fetch[A-Za-z0-9_$]*\s*\(|\baxios\.|\.\s*request\s*\(/;

/**
 * Modules that must appear in the census, whatever the matcher is doing.
 *
 * These are positive controls. A census that stops seeing one of these has broken,
 * and "no offenders" from a broken census reads exactly like a clean result.
 * `arp/intelligence/adapters.ts` is here because it was invisible for two months.
 */
const MUST_BE_COUNTED = [
  'arp/intelligence/adapters.ts',
  'arp/telemetry/signature/emitter.ts',
  'arp/telemetry/gtin.ts',
];

/**
 * Modules that transmit anything off this process.
 *
 * The question this census asks used to be "what leaves for the OpenA2A registry".
 * That framing is why the L2 vendor channel went uncounted: it addresses
 * `api.anthropic.com`, so a registry-scoped filter could never see it, and widening
 * the host list alone would not have helped because the send was also aliased. The
 * question is now "what leaves this process", and the destination is a property to
 * disclose rather than a condition for being counted.
 */
function egressModules(): string[] {
  const hits: string[] = [];
  for (const file of walk(SRC)) {
    const code = stripComments(readFileSync(file, 'utf-8'));
    if (SENDER.test(code)) hits.push(relative(SRC, file));
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

  it('sees every module it is known to have missed before', () => {
    // Named positive controls. The census was believed to pin the whole channel
    // set while silently omitting the L2 vendor adapter, and nothing failed.
    const mods = egressModules();
    const missing = MUST_BE_COUNTED.filter(m => !mods.includes(m));
    expect(
      missing,
      'The census no longer sees these modules, so it is not measuring what it claims:\n' +
        missing.map(m => `  - ${m}`).join('\n') +
        '\n\nFix the matcher rather than removing the entry.',
    ).toEqual([]);
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
