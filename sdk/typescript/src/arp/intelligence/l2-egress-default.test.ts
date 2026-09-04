import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mkdtempSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';

/**
 * L2 must not send anything to a third party unless the operator switched it on AND
 * chose the destination.
 *
 * Every published `@opena2a/aim-sdk` from 1.0.0 to 1.3.1 shipped
 * `intelligence: { enabled: true, adapter: 'agent-proxy' }`, and `agent-proxy`
 * resolved a destination from whichever model key happened to be exported. A
 * default-configured ARP therefore sent event content — including truncated command
 * lines — to a vendor, billed to the operator's own account, with nothing in the
 * README saying so.
 *
 * These tests are written against the two gates that let that happen, not against
 * the symptom:
 *
 *   - the constructor built an adapter on `enabled !== false`, so an ABSENT value
 *     (which is what a shallow-merged partial config produces) meant ON;
 *   - an absent `adapter` fell back to auto-detection from the environment.
 *
 * Every assertion below has a positive control in the SAME run. Without one, a
 * broken harness — a mock that stops intercepting, a constructor that throws early —
 * produces zero outbound requests and reads exactly like a pass.
 *
 * Mutation results, measured when this suite was written, stated because "which
 * mutant survives" is the only honest description of what a suite covers:
 *
 *   killed  restoring `enabled: true, adapter: 'agent-proxy'` to defaultConfig()   -> T0
 *   killed  restoring the env-scanning body of createAdapter('agent-proxy')        -> T3
 *   killed  restoring the raw `Matched text: "..."` interpolation                  -> T5
 *   killed  reverting BOTH L2 gates in coordinator.ts to `enabled !== false`       -> T2c
 *   SURVIVES  reverting EITHER gate alone
 *
 * That last line is not a gap being papered over. The constructor gate and
 * `shouldEscalateToL2` are deliberately redundant, so either one alone still
 * prevents the egress — which means no test can attribute the property to one of
 * them. The pair is what is proven, and it is proven by reverting the pair. Do not
 * "simplify" by deleting one on the grounds that tests still pass.
 */

const requests: Array<{ hostname?: string; path?: string; body: string }> = [];

function fakeRequest(options: Record<string, unknown>, cb?: (res: unknown) => void) {
  const record = { hostname: options.hostname as string, path: options.path as string, body: '' };
  requests.push(record);
  const res = {
    statusCode: 200,
    on(event: string, handler: (chunk?: unknown) => void) {
      if (event === 'data') handler(Buffer.from(JSON.stringify({ content: [{ text: 'CONSISTENT: YES' }] })));
      if (event === 'end') handler();
      return res;
    },
  };
  if (cb) setImmediate(() => cb(res));
  return {
    on() { return this; },
    write(chunk: string) { record.body += chunk; },
    end() { /* no socket is ever opened */ },
    destroy() { /* no-op */ },
  };
}

vi.mock('https', () => ({ request: fakeRequest, default: { request: fakeRequest } }));
vi.mock('http', () => ({ request: fakeRequest, default: { request: fakeRequest } }));

import { IntelligenceCoordinator } from './coordinator';
import { createAdapter } from './adapters';
import { defaultConfig } from '../config/loader';
import type { ARPConfig, ARPEvent, IntelligenceConfig } from '../types';

const SYNTHETIC_KEY = 'sk-ant-' + 'test' + '-' + '0'.repeat(24);
const CANARY_COMMAND = 'CANARY' + 'COMMANDLINE' + '7f3a91';
const CANARY_MATCHED = 'CANARY' + 'MATCHEDSECRET' + 'b42e08';

function dataDir(): string {
  return mkdtempSync(join(tmpdir(), 'arp-l2-egress-'));
}

function criticalEvent(overrides: Partial<ARPEvent> = {}): ARPEvent {
  return {
    id: 'evt-1',
    timestamp: new Date().toISOString(),
    source: 'network',
    category: 'suspicious',
    severity: 'critical',
    description: 'Outbound connection to an unexpected host',
    data: { command: CANARY_COMMAND, remoteAddr: '203.0.113.9:443' },
    ...overrides,
  } as ARPEvent;
}

function cfg(intelligence: IntelligenceConfig | undefined): ARPConfig {
  return { ...defaultConfig(), agentName: 'test-agent', intelligence } as ARPConfig;
}

async function runAnalyze(intelligence: IntelligenceConfig | undefined, event = criticalEvent()) {
  const c = new IntelligenceCoordinator(cfg(intelligence), dataDir());
  await c.analyze(event);
  return c;
}

beforeEach(() => {
  requests.length = 0;
  process.env.ANTHROPIC_API_KEY = SYNTHETIC_KEY;
  process.env.OPENAI_API_KEY = SYNTHETIC_KEY;
});

afterEach(() => {
  delete process.env.ANTHROPIC_API_KEY;
  delete process.env.OPENAI_API_KEY;
});

describe('the shipped default itself', () => {
  it('T0: defaultConfig() has L2 off and names no destination', () => {
    // This asserts the DEFAULT, not its consequence, and it exists because the
    // behavioural tests below could not tell the two apart.
    //
    // Restoring `enabled: true, adapter: 'agent-proxy'` here left every other test
    // in this file green: createAdapter('agent-proxy') now throws, so the adapter
    // came back null and no request was made either way. That makes the two fixes a
    // layered defence whose inner layer no test could distinguish from its absence.
    // Both layers are wanted; this is the one that pins the outer one.
    const intel = defaultConfig().intelligence ?? {};

    // Not `toBe(false)`: `enabled` is deliberately ABSENT. Every gate that starts
    // L2 tests `=== true`, so absent is off just as firmly — while an explicit
    // false here would also trip the twin kill switch in buildRuntimeTwin and
    // disable L1's behavioral signal by default. What must hold is that the
    // default is not TRUE, whichever of the two spellings is in use.
    expect(intel.enabled, 'L2 must not default to on').not.toBe(true);
    expect(
      intel.adapter,
      'the default must name no destination — an adapter chosen by default is a destination the operator never picked',
    ).toBeUndefined();
  });
});

describe('L2 sends nothing unless it was configured to', () => {
  it('T1: the shipped default makes no outbound call, even with vendor keys exported', async () => {
    // The exact acceptance line from the roadmap unit: on a machine exporting
    // ANTHROPIC_API_KEY, a default-config ARP run makes no outbound vendor call.
    await runAnalyze(defaultConfig().intelligence);
    expect(requests, `default config sent: ${JSON.stringify(requests)}`).toEqual([]);

    // POSITIVE CONTROL, same run, same mock: an explicitly configured adapter DOES
    // send. If this is empty the harness is broken and the assertion above is
    // vacuous rather than true.
    await runAnalyze({ enabled: true, adapter: 'anthropic', adapterConfig: { apiKey: SYNTHETIC_KEY } });
    expect(requests.length, 'positive control produced no request — the mock is not intercepting').toBeGreaterThan(0);
    expect(requests[0].hostname).toBe('api.anthropic.com');
  });

  it('T2: a partial config that sets only budgetUsd does not arm the vendor channel', async () => {
    // loadConfig merges shallowly, so this replaces the whole intelligence object and
    // arrives at the coordinator with `enabled` undefined. Under the previous
    // `enabled !== false` test this path egressed: narrowing the budget, a hardening
    // action, switched L2 on.
    await runAnalyze({ budgetUsd: 1 } as IntelligenceConfig);
    expect(requests, `partial config sent: ${JSON.stringify(requests)}`).toEqual([]);
  });

  it('T2b: enabled:true with no adapter still sends nothing — absent means no destination', async () => {
    await runAnalyze({ enabled: true } as IntelligenceConfig);
    expect(requests, `adapterless config sent: ${JSON.stringify(requests)}`).toEqual([]);
  });

  it('T2c: an adapter named with `enabled` ABSENT sends nothing — absent is not on', async () => {
    // This is the fixture that actually exercises the `enabled` gate, and it exists
    // because the first version of this suite did not.
    //
    // T2 above sets only budgetUsd, so it has no adapter either — which means the
    // adapter half of the condition answers first and the `enabled` half is never
    // consulted. Reverting `enabled === true` back to `enabled !== false` left the
    // whole suite green. A test that cannot fail is not evidence.
    //
    // Here the destination IS named, so the ONLY thing standing between this config
    // and a vendor request is how an absent `enabled` is read. Under the shipped
    // `!== false` this egressed; under `=== true` it does not. This is also the
    // realistic shape: an operator who sets adapter and budget while relying on the
    // old default-true.
    await runAnalyze({
      adapter: 'anthropic',
      adapterConfig: { apiKey: SYNTHETIC_KEY },
      budgetUsd: 1,
    } as IntelligenceConfig);
    expect(
      requests,
      `a config naming a vendor with enabled absent sent: ${JSON.stringify(requests)}`,
    ).toEqual([]);
  });

  it('T3: agent-proxy refuses instead of resolving a vendor from the environment', () => {
    expect(() => createAdapter('agent-proxy')).toThrow(/no longer selects a provider from environment credentials/);

    // POSITIVE CONTROL: the ones that should still build, still build — so the throw
    // above is a deliberate refusal and not createAdapter being broken outright.
    expect(createAdapter('ollama').name).toBe('ollama');
    expect(createAdapter('anthropic', { apiKey: SYNTHETIC_KEY }).name).toBe('anthropic');
  });
});

describe('when a remote adapter IS configured, raw material still does not travel', () => {
  it('T5: the command line and matched secret are withheld, the pattern id is not', async () => {
    const event = criticalEvent({
      source: 'prompt',
      description: 'LLM response matched output leak pattern OL-001',
      data: {
        patternId: 'OL-001',
        patternCategory: 'output-leak',
        direction: 'output',
        matchedText: CANARY_MATCHED,
        command: CANARY_COMMAND,
      },
    });

    await runAnalyze(
      { enabled: true, adapter: 'anthropic', adapterConfig: { apiKey: SYNTHETIC_KEY } },
      event,
    );

    expect(requests.length, 'no request was captured — nothing was measured').toBeGreaterThan(0);
    const body = requests.map(r => r.body).join('');

    // The material must not be there.
    expect(body, 'the matched secret reached the vendor').not.toContain(CANARY_MATCHED);
    expect(body, 'the command line reached the vendor').not.toContain(CANARY_COMMAND);

    // CONTROL: a value the boundary is proven NOT to remove. Without this the test
    // would also pass if the redactor simply emptied the whole prompt, which would
    // make L2 useless rather than safe.
    expect(body, 'the pattern id was stripped too — the redaction is over-broad').toContain('OL-001');

    // The fact of the match still travels, so the model can still assess it.
    expect(body).toMatch(/withheld: \d+ chars/);
  });
});
