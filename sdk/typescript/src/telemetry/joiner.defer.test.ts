/**
 * The joiner's DEFAULT sink must defer the synchronous disk write off the
 * caller's stack, so the full-record case never adds fs.appendFileSync latency
 * to the verifyAction path. local-writer is mocked so this test touches no disk.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

vi.mock('./local-writer', () => ({
  writeCorrelatedRecord: vi.fn(() => true),
  readCorrelatedRecords: vi.fn(() => []),
  defaultDataDir: vi.fn(() => '/tmp/never-written'),
}));

import { CorrelationJoiner } from './joiner';
import { writeCorrelatedRecord } from './local-writer';

const CID = 'cde_000000000_aabbccddeeff';

function feedFullRecord(joiner: CorrelationJoiner): void {
  joiner.ingestEnforcement({
    correlationId: CID,
    agentId: 'agent-1',
    enforcement: {
      decision: 'deny',
      outcome: 'DENY_INTENT',
      capability: 'fs:write',
      resource: '/etc/x',
      occurredAt: '2026-06-06T00:00:00.000Z',
      source: 'aim-pdp',
    },
  });
  joiner.ingestIntent({
    correlationId: CID,
    intent: { intentClass: 'exfil', confidence: 0.7, blocked: true, source: 'nm-intent' },
  });
  joiner.ingestDetection({
    correlationId: CID,
    detection: {
      injectionDetected: true,
      techniqueId: 'T-2002',
      techniqueSource: 'interim-mapping',
      confidence: 0.84,
      detector: 'nanomind-guard',
      detectedAt: '2026-06-06T00:00:00.000Z',
    },
  });
}

describe('default sink defers the disk write', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('does not write synchronously on emit, but writes on the next tick', () => {
    const joiner = new CorrelationJoiner({ windowMs: 60_000 }); // default (deferred) sink
    feedFullRecord(joiner);

    // The record assembled synchronously, but the disk write is deferred.
    expect(writeCorrelatedRecord).not.toHaveBeenCalled();

    vi.runAllTimers();
    expect(writeCorrelatedRecord).toHaveBeenCalledTimes(1);
  });

  it('forwards the configured dataDir to the writer (so it matches the relay)', () => {
    const joiner = new CorrelationJoiner({ windowMs: 60_000, dataDir: '/custom/telemetry/dir' });
    feedFullRecord(joiner);
    vi.runAllTimers();
    expect(writeCorrelatedRecord).toHaveBeenCalledTimes(1);
    expect((writeCorrelatedRecord as unknown as { mock: { calls: unknown[][] } }).mock.calls[0][1]).toBe(
      '/custom/telemetry/dir'
    );
  });
});
