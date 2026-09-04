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
import { describeL2Status } from './coordinator';
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
    // The property this whole suite exists to prove: on a machine exporting
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
    expect(body).toMatch(/withheld: (empty|up to 8 chars|\d+-\d+ chars|over 128 chars)/);
  });

  it('T6: event.description is NOT redacted — this is the known residual, pinned', async () => {
    // Honest boundary of the event.data redaction, asserted rather than described.
    //
    // The allowlist covers `event.data`. It does NOT cover `event.description`,
    // which every prompt builder sends verbatim, and the process monitor composes
    // descriptions like `New child process: PID 123 — <first 100 chars of command>`
    // (monitors/process.ts). So on a deliberately configured remote adapter, up to
    // 100 characters of a command line still reach the vendor through the
    // description, even though `data.command` is withheld.
    //
    // This test exists so that stays a KNOWN, tested property instead of a claim
    // someone later discovers is false. If description redaction lands, this test
    // should flip to asserting absence — do not simply delete it.
    const event = criticalEvent({
      source: 'process',
      description: `New child process: PID 4242 — ${CANARY_COMMAND}`,
      data: { pid: 4242, command: CANARY_COMMAND },
    });

    await runAnalyze(
      { enabled: true, adapter: 'anthropic', adapterConfig: { apiKey: SYNTHETIC_KEY } },
      event,
    );

    expect(requests.length, 'no request captured — nothing was measured').toBeGreaterThan(0);
    const body = requests.map(r => r.body).join('');

    // data.command IS withheld by the allowlist.
    expect(body).toMatch(/withheld: (empty|up to 8 chars|\d+-\d+ chars|over 128 chars)/);

    // ...but the same value inside the description is not. Asserting the leak
    // rather than pretending it is closed.
    expect(
      body,
      'description redaction has landed — update this test and the CHANGELOG claim',
    ).toContain(CANARY_COMMAND);
  });
});

describe('what the status line reports is what actually runs', () => {
  // Regression tests for a line that was wrong twice. It first restated the gate as
  // `enabled !== false` and kept printing "3-Layer" after the default changed; it was
  // then rewritten as `enabled === true && adapter`, which is still wrong for
  // 'agent-proxy' — that passes both checks and throws on construction, so the line
  // announced a layer that was not running. Neither version had a test.

  it('T7: agent-proxy is reported as NOT running, because constructing it throws', () => {
    const s = describeL2Status({ enabled: true, adapter: 'agent-proxy' } as IntelligenceConfig);
    expect(s.running, 'agent-proxy cannot run, so it must not be reported as running').toBe(false);
    expect(s.reason).toMatch(/no longer selects a provider from environment credentials/);
  });

  it('T8: the states that should run, do; the states that should not, do not', () => {
    expect(describeL2Status(defaultConfig().intelligence).running).toBe(false);
    expect(describeL2Status({ enabled: true } as IntelligenceConfig).running).toBe(false);
    expect(describeL2Status({ adapter: 'ollama' } as IntelligenceConfig).running).toBe(false);
    expect(describeL2Status({ enabled: false, adapter: 'ollama' } as IntelligenceConfig).running).toBe(false);

    // POSITIVE CONTROL: a genuinely runnable config reports running, so the four
    // falses above are a verdict and not a function that always says no.
    const ok = describeL2Status({ enabled: true, adapter: 'ollama' } as IntelligenceConfig);
    expect(ok.running, 'a valid local config must report running').toBe(true);
    expect(ok.adapter).toBe('ollama');
  });
});

describe('withheld descriptors do not reconstruct the value', () => {
  // These go through a real analyze() call and read the REQUEST BODY. They do not
  // re-implement the descriptor: a test that restates the rule it is checking passes
  // whenever the copy and the original agree, including when both are wrong.

  const remote = { enabled: true, adapter: 'anthropic', adapterConfig: { apiKey: SYNTHETIC_KEY } } as IntelligenceConfig;

  async function bodyFor(data: Record<string, unknown>): Promise<string> {
    requests.length = 0;
    await runAnalyze(remote, criticalEvent({ description: 'fixed text', data }));
    expect(requests.length, 'no request captured — nothing measured').toBeGreaterThan(0);
    return requests.map(r => r.body).join('');
  }

  it('T9: a boolean is not recoverable from its descriptor', async () => {
    // An exact length plus a class set IS the value for a small value: `true` renders
    // 4 chars/lower and `false` 5 chars/lower, so the pair reconstructs the boolean.
    const t = await bodyFor({ secretFlag: true });
    const f = await bodyFor({ secretFlag: false });

    expect(t, 'the raw value must not appear').not.toContain('true');
    const tDesc = t.match(/<withheld:[^>]*>/)?.[0];
    const fDesc = f.match(/<withheld:[^>]*>/)?.[0];
    expect(tDesc, 'no withheld descriptor found — the allowlist did not fire').toBeTruthy();
    expect(tDesc, 'true and false must not be distinguishable by descriptor').toBe(fDesc);
  });

  it('T10: a long value keeps enough shape to assess, and never the value', async () => {
    const secret = 'A'.repeat(20) + 'b'.repeat(20) + '9';   // 41 chars
    const body = await bodyFor({ secretBlob: secret });

    expect(body, 'the value itself must not travel').not.toContain(secret);
    expect(body).toContain('33-64 chars');
    expect(body).toMatch(/lower/);

    // CONTROL: the bucket is genuinely coarser than the exact length, so the
    // descriptor cannot be inverted to the value's size.
    expect(body, 'the exact length leaked').not.toContain('41 chars');
  });
});
