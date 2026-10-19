import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import type { ARPConfig, ARPEvent, BudgetState, IntelligenceConfig } from '../../../src/arp/types';

/**
 * A batched L2 call's answer is read, and each event of the batch receives its own
 * assessment the way a single-event call's event does: `llmAssessment`, `classifiedBy`
 * of `L2-llm`, and the raised severity and category of a PAUSE or KILL.
 *
 * Measured with a stub adapter before this change, a batch of three `high` events flushed
 * by stop() was sent as one call and recorded in budget.json at its cost, and all three
 * events kept `classifiedBy: 'L0-rules'`, their severity and category, and no
 * `llmAssessment`. These tests queue events with batching on, flush them with stop(), and
 * read the events afterwards.
 */

const stub = vi.hoisted(() => {
  const state = {
    calls: 0,
    fail: false,
    content: '',
    prompts: [] as string[],
  };
  const adapter = {
    name: 'stub',
    async assess(prompt: string): Promise<{ content: string; inputTokens: number; outputTokens: number; model: string }> {
      state.calls += 1;
      state.prompts.push(prompt);
      await new Promise<void>((resolve) => setImmediate(resolve));
      if (state.fail) throw new Error('stub adapter: the call failed');
      return { content: state.content, inputTokens: 150, outputTokens: 50, model: 'stub' };
    },
    // One cent per thousand tokens: an answered call (150 in, 50 out) costs 0.002, and
    // a batch's estimate (200 in, 600 out) is 0.008.
    estimateCost(inputTokens: number, outputTokens: number): number {
      return (inputTokens + outputTokens) / 100000;
    },
    async healthCheck(): Promise<boolean> {
      return true;
    },
  };
  return { state, adapter };
});

vi.mock('../../../src/arp/intelligence/adapters', () => ({
  createAdapter: () => stub.adapter,
}));

import { IntelligenceCoordinator } from '../../../src/arp/intelligence/coordinator';

const ANSWERED_TOKENS = 200;
const ANSWERED_COST_USD = 0.002;

function config(intelligence: IntelligenceConfig = {}): ARPConfig {
  return {
    agentName: 'test-agent',
    intelligence: {
      enabled: true,
      adapter: 'anthropic',
      budgetUsd: 100,
      maxCallsPerHour: 20,
      enableBatching: true,
      batchWindowMs: 3600000,
      ...intelligence,
    },
  };
}

/** A flagged, non-critical event: L2 takes it, and batching queues it. */
function flagged(i: number): ARPEvent {
  return {
    id: `evt-${i}`,
    timestamp: new Date().toISOString(),
    source: 'network',
    category: 'anomaly',
    severity: 'high',
    description: `Outbound connection ${i} to an unexpected host`,
    data: { remoteAddr: '203.0.113.9:443' },
    classifiedBy: 'L0-rules',
  };
}

function readBudgetFile(dir: string): BudgetState {
  return JSON.parse(fs.readFileSync(path.join(dir, 'budget.json'), 'utf-8')) as BudgetState;
}

describe('IntelligenceCoordinator batched L2 assessment', () => {
  let dataDir: string;

  beforeEach(() => {
    dataDir = fs.mkdtempSync(path.join(os.tmpdir(), 'coord-batch-'));
    stub.state.calls = 0;
    stub.state.fail = false;
    stub.state.content = '';
    stub.state.prompts = [];
  });

  afterEach(() => {
    fs.rmSync(dataDir, { recursive: true, force: true });
  });

  /** Queue `count` flagged events, answer their batch with `answer`, and return them. */
  async function assessBatch(answer: string, count: number, intelligence?: IntelligenceConfig): Promise<ARPEvent[]> {
    stub.state.content = answer;
    const coord = new IntelligenceCoordinator(config(intelligence), dataDir);
    const events = Array.from({ length: count }, (_, i) => flagged(i + 1));
    for (const e of events) {
      // A queued event has no assessment yet; it receives one when its batch is sent.
      expect(await coord.analyze(e)).toBeNull();
    }
    expect(stub.state.calls).toBe(0);
    await coord.stop();
    return events;
  }

  it('applies each event its own verdict from the batch answer, as a single-event call applies one', async () => {
    const [kill, pause, allow] = await assessBatch(
      [
        '1: KILL — sends data to a host the agent never uses',
        '2: PAUSE — unusual port for this agent',
        '3: ALLOW — the agent calls this API',
      ].join('\n'),
      3,
    );

    expect(stub.state.calls).toBe(1);
    expect(readBudgetFile(dataDir).totalCalls).toBe(1);

    expect(kill.classifiedBy).toBe('L2-llm');
    expect(kill.severity).toBe('critical');
    expect(kill.category).toBe('threat');
    expect(kill.llmAssessment).toMatchObject({
      recommendation: 'kill',
      consistent: false,
      // A batch line carries no confidence: the single-event parser's default.
      confidence: 0.5,
      reasoning: 'sends data to a host the agent never uses',
    });

    expect(pause.classifiedBy).toBe('L2-llm');
    expect(pause.severity).toBe('high');
    expect(pause.category).toBe('violation');
    expect(pause.llmAssessment).toMatchObject({
      recommendation: 'pause',
      consistent: false,
      reasoning: 'unusual port for this agent',
    });

    expect(allow.classifiedBy).toBe('L2-llm');
    expect(allow.severity).toBe('high');
    expect(allow.category).toBe('anomaly');
    expect(allow.llmAssessment).toMatchObject({
      recommendation: 'allow',
      consistent: true,
      confidence: 0.5,
      reasoning: 'the agent calls this API',
    });
  });

  it('two flagged high events flushed by stop() both receive an assessment from the one call', async () => {
    const events = await assessBatch('1: ALERT — first\n2: ALERT — second', 2);

    expect(stub.state.calls).toBe(1);
    const state = readBudgetFile(dataDir);
    expect(state.totalCalls).toBe(1);
    expect(state.totalSpentUsd).toBe(ANSWERED_COST_USD);
    for (const e of events) {
      expect(e.classifiedBy).toBe('L2-llm');
      expect(e.llmAssessment).toMatchObject({ recommendation: 'alert', consistent: false });
      expect(e.severity).toBe('high');
      expect(e.category).toBe('anomaly');
    }
  });

  it('asks for a line for every event, in the format the answer is read in', async () => {
    await assessBatch('1: ALLOW — fine\n2: ALLOW — fine', 2);

    const [prompt] = stub.state.prompts;
    expect(prompt).toContain('1. [network] high: Outbound connection 1 to an unexpected host');
    expect(prompt).toContain('2. [network] high: Outbound connection 2 to an unexpected host');
    expect(prompt).toContain('Answer every event, one line each, in exactly this format:');
    expect(prompt).toContain('EVENT_NUM: ALLOW, ALERT, PAUSE, or KILL — one sentence reason');
  });

  it('gives an event the answer leaves out the deny-safe default and leaves its severity alone', async () => {
    const [answered, missing] = await assessBatch('1: ALLOW — expected traffic', 2);

    expect(answered.llmAssessment?.recommendation).toBe('allow');

    expect(missing.classifiedBy).toBe('L2-llm');
    expect(missing.llmAssessment).toMatchObject({
      recommendation: 'alert',
      consistent: false,
      reasoning: 'No action for this event in the batch answer — defaulting to deny-safe',
    });
    expect(missing.severity).toBe('high');
    expect(missing.category).toBe('anomaly');
  });

  it('does not read an action it does not recognize as ALLOW', async () => {
    const [unknown, hedged, allowed] = await assessBatch(
      '1: APPROVE — looks fine\n2: ALLOW/KILL — unsure\n3: ALLOW — looks fine',
      3,
    );

    for (const e of [unknown, hedged]) {
      expect(e.classifiedBy).toBe('L2-llm');
      expect(e.llmAssessment).toMatchObject({
        recommendation: 'alert',
        consistent: false,
        reasoning: 'No action for this event in the batch answer — defaulting to deny-safe',
      });
      expect(e.severity).toBe('high');
      expect(e.category).toBe('anomaly');
    }
    expect(allowed.llmAssessment).toMatchObject({ recommendation: 'allow', consistent: true });
  });

  it('keeps the stricter action when the answer gives an event two', async () => {
    const [killFirst, killSecond, pauseSecond] = await assessBatch(
      [
        '1: KILL — exfiltration',
        '2: ALLOW — benign',
        '3: ALLOW — benign',
        '1: ALLOW — benign on reflection',
        '2: KILL — exfiltration on reflection',
        '3: PAUSE — needs a look',
      ].join('\n'),
      3,
    );

    expect(killFirst.llmAssessment).toMatchObject({ recommendation: 'kill', reasoning: 'exfiltration' });
    expect(killFirst.severity).toBe('critical');
    expect(killSecond.llmAssessment).toMatchObject({ recommendation: 'kill', reasoning: 'exfiltration on reflection' });
    expect(killSecond.severity).toBe('critical');
    expect(pauseSecond.llmAssessment).toMatchObject({ recommendation: 'pause', reasoning: 'needs a look' });
    expect(pauseSecond.category).toBe('violation');
  });

  it('ignores numbers outside the batch and lines that are not verdicts', async () => {
    const [first, second] = await assessBatch(
      [
        'Assessment of 2 events:',
        '0: KILL — not an event of this batch',
        '3: KILL — not an event of this batch',
        '1: ALLOW — expected',
        '2: ALLOW — expected',
        'Overall: no event needs action.',
      ].join('\n'),
      2,
    );

    expect(first.llmAssessment?.recommendation).toBe('allow');
    expect(first.severity).toBe('high');
    expect(second.llmAssessment?.recommendation).toBe('allow');
    expect(second.severity).toBe('high');
  });

  it('reads a verdict line written with markdown emphasis, an EVENT prefix, a list bullet or no reason', async () => {
    const [bold, prefixed, bulleted, bare] = await assessBatch(
      ['**1**: **KILL** — exfiltration', 'Event 2: PAUSE - odd port', '- 3. ALLOW: expected', '4) ALERT'].join('\n'),
      4,
    );

    expect(bold.llmAssessment).toMatchObject({ recommendation: 'kill', reasoning: 'exfiltration' });
    expect(prefixed.llmAssessment).toMatchObject({ recommendation: 'pause', reasoning: 'odd port' });
    expect(bulleted.llmAssessment).toMatchObject({ recommendation: 'allow', reasoning: 'expected' });
    expect(bare.llmAssessment).toMatchObject({
      recommendation: 'alert',
      reasoning: 'No reason given in the batch answer',
    });
  });

  it("shares the call's tokens and cost equally among the events of the batch", async () => {
    const events = await assessBatch('1: ALLOW — a\n2: ALLOW — b\n3: ALLOW — c', 3);

    expect(events.map((e) => e.llmAssessment?.tokensUsed)).toEqual([67, 67, 66]);
    const tokens = events.reduce((sum, e) => sum + (e.llmAssessment?.tokensUsed ?? 0), 0);
    expect(tokens).toBe(ANSWERED_TOKENS);
    for (const e of events) {
      expect(e.llmAssessment?.estimatedCost).toBeCloseTo(ANSWERED_COST_USD / 3, 12);
    }
    expect(readBudgetFile(dataDir).totalSpentUsd).toBe(ANSWERED_COST_USD);
  });

  it('leaves the events of a batch whose call fails at their L0/L1 classification', async () => {
    stub.state.fail = true;
    const events = await assessBatch('1: KILL — exfiltration\n2: KILL — exfiltration', 2);

    expect(stub.state.calls).toBe(1);
    for (const e of events) {
      expect(e.llmAssessment).toBeUndefined();
      expect(e.classifiedBy).toBe('L0-rules');
      expect(e.severity).toBe('high');
      expect(e.category).toBe('anomaly');
    }
  });

  it('leaves the events of a batch the budget refuses at their L0/L1 classification', async () => {
    // The batch estimate is 0.008, more than this budget holds.
    const events = await assessBatch('1: KILL — exfiltration\n2: KILL — exfiltration', 2, { budgetUsd: 0.001 });

    expect(stub.state.calls).toBe(0);
    for (const e of events) {
      expect(e.llmAssessment).toBeUndefined();
      expect(e.classifiedBy).toBe('L0-rules');
      expect(e.severity).toBe('high');
    }
  });
});
