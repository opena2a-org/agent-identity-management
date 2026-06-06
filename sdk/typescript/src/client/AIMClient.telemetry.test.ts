/**
 * Tests for AIMClient causal-denial telemetry wiring.
 *
 * Covers the opt-in behaviour: a correlation ID is minted at ingestion and
 * propagated on the verify request; the enforcement outcome (allow AND deny) is
 * fed to the joiner before the deny throw; supplied intent/detection assemble a
 * full record; and telemetry is fully inert when disabled and never affects the
 * verification result even when it throws.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { AIMClient } from './AIMClient';
import { ActionDeniedError } from '../exceptions';
import {
  CorrelationJoiner,
  CORRELATION_HEADER,
  isCorrelationId,
  type CorrelatedRecord,
  type IntentInput,
  type DetectionInput,
} from '../telemetry';
import { generateKeyPair, toBase64 } from '../crypto/ed25519';
import type { AgentCredentials, VerificationResult } from '../types';

const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

async function realCredentials(agentId = 'agent-tele-1'): Promise<AgentCredentials> {
  const { privateKey, publicKey } = await generateKeyPair();
  return {
    agentId,
    privateKey: toBase64(privateKey),
    publicKey: toBase64(publicKey),
    organizationId: 'org-tele',
    createdAt: new Date().toISOString(),
  };
}

// Credentialed requests acquire an OAuth token first; queue that response,
// then the verify response. The verify call is always the last fetch.
function mockVerifyResponse(overrides: Partial<VerificationResult> = {}): void {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    json: async () => ({ accessToken: 'test-token', tokenType: 'Bearer', expiresIn: 3600 }),
  });
  const body: VerificationResult = {
    verified: true,
    agentId: 'agent-tele-1',
    agentName: 'tele-agent',
    trustScore: 0.9,
    riskLevel: 'low' as VerificationResult['riskLevel'],
    actionAllowed: true,
    timestamp: '2026-06-06T00:00:00Z',
    eventId: 'evt-123',
    ...overrides,
  };
  mockFetch.mockResolvedValueOnce({
    ok: true,
    text: async () => JSON.stringify(body),
  });
}

function lastRequestHeaders(): Record<string, string> {
  const call = mockFetch.mock.calls.at(-1);
  return (call?.[1]?.headers ?? {}) as Record<string, string>;
}

const SAMPLE_INTENT: IntentInput = {
  intentClass: 'data_exfiltration',
  confidence: 0.91,
  blocked: true,
  source: 'nanomind-intent',
  modelVersion: '3.0.0',
};

const SAMPLE_DETECTION: DetectionInput = {
  injectionDetected: true,
  attackClass: 'indirect',
  techniqueId: 'T-2002',
  techniqueSource: 'interim-mapping',
  confidence: 0.84,
  detector: 'nanomind-guard',
  inputRef: 'sha256:abc',
  detectedAt: '2026-06-06T00:00:00Z',
};

describe('AIMClient causal-denial telemetry', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    delete process.env.AIM_BASE_URL;
    delete process.env.AIM_API_KEY;
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.stubGlobal('fetch', mockFetch);
  });

  describe('disabled by default', () => {
    it('does not attach a correlation header when telemetry is off', async () => {
      const client = new AIMClient();
      client.setCredentials(await realCredentials());
      mockVerifyResponse();

      await client.verifyAction({ action: 'db:read', resource: 'users' });

      const headers = lastRequestHeaders();
      expect(headers[CORRELATION_HEADER]).toBeUndefined();
    });

    it('does not feed a supplied joiner when telemetry is off', async () => {
      const records: CorrelatedRecord[] = [];
      const joiner = new CorrelationJoiner({ onRecord: (r) => records.push(r) });
      const client = new AIMClient({ telemetry: { joiner } }); // enabled not set
      client.setCredentials(await realCredentials());
      mockVerifyResponse();

      await client.verifyAction({ action: 'db:read', resource: 'users' });

      expect(joiner.pending).toBe(0);
      expect(records).toHaveLength(0);
    });
  });

  describe('enabled', () => {
    it('attaches a well-formed correlation ID header on the verify request', async () => {
      const client = new AIMClient({ telemetry: { enabled: true } });
      client.setCredentials(await realCredentials());
      mockVerifyResponse();

      await client.verifyAction({ action: 'db:read', resource: 'users' });

      const id = lastRequestHeaders()[CORRELATION_HEADER];
      expect(isCorrelationId(id)).toBe(true);
      client.closeTelemetry();
    });

    it('feeds an enforcement fact to the joiner on an allowed action', async () => {
      const joiner = new CorrelationJoiner({ windowMs: 60_000 });
      const client = new AIMClient({ telemetry: { enabled: true, joiner } });
      client.setCredentials(await realCredentials());
      mockVerifyResponse({ actionAllowed: true });

      await client.verifyAction({ action: 'db:read', resource: 'users' });

      // Enforcement-only: buffered (awaiting intent/detection), not yet emitted.
      expect(joiner.pending).toBe(1);
    });

    it('assembles a full record when intent + detection are supplied', async () => {
      const records: CorrelatedRecord[] = [];
      const joiner = new CorrelationJoiner({ windowMs: 60_000, onRecord: (r) => records.push(r) });
      const client = new AIMClient({ telemetry: { enabled: true, joiner } });
      client.setCredentials(await realCredentials());
      mockVerifyResponse({ actionAllowed: true });

      await client.verifyAction({
        action: 'file:read',
        resource: '/data/secret.json',
        telemetry: { intent: SAMPLE_INTENT, detection: SAMPLE_DETECTION },
      });

      expect(records).toHaveLength(1);
      const rec = records[0];
      expect(rec.assembly.completeness).toBe('full');
      expect(rec.assembly.joinedBy).toBe('correlationId');
      expect(rec.enforcement.observed).toBe(true);
      expect(rec.enforcement.decision).toBe('allow');
      expect(rec.enforcement.capability).toBe('file:read');
      expect(rec.enforcement.resource).toBe('/data/secret.json');
      expect(rec.enforcement.source).toBe('aim-pdp');
      expect(rec.intent?.observed).toBe(false);
      expect(rec.detection?.observed).toBe(false);
      expect(rec.detection?.techniqueId).toBe('T-2002');
    });

    it('captures the deny outcome before throwing ActionDeniedError', async () => {
      const records: CorrelatedRecord[] = [];
      const joiner = new CorrelationJoiner({ windowMs: 60_000, onRecord: (r) => records.push(r) });
      const client = new AIMClient({ telemetry: { enabled: true, joiner } });
      client.setCredentials(await realCredentials());
      mockVerifyResponse({ actionAllowed: false, denialReason: 'capability not granted' });

      await expect(
        client.verifyAction({
          action: 'file:write',
          resource: '/etc/passwd',
          telemetry: { intent: SAMPLE_INTENT, detection: SAMPLE_DETECTION },
        })
      ).rejects.toThrow(ActionDeniedError);

      expect(records).toHaveLength(1);
      expect(records[0].enforcement.decision).toBe('deny');
      expect(records[0].enforcement.outcome).toBe('DENY');
      expect(records[0].enforcement.deniedReason).toBe('capability not granted');
    });

    it('honors a custom enforcement source', async () => {
      const joiner = new CorrelationJoiner({ windowMs: 60_000 });
      let captured: CorrelatedRecord | undefined;
      const sinkJoiner = new CorrelationJoiner({
        windowMs: 60_000,
        onRecord: (r) => (captured = r),
      });
      void joiner;
      const client = new AIMClient({
        telemetry: { enabled: true, joiner: sinkJoiner, enforcementSource: 'custom-pdp' },
      });
      client.setCredentials(await realCredentials());
      mockVerifyResponse({ actionAllowed: true });

      await client.verifyAction({
        action: 'db:read',
        resource: 'users',
        telemetry: { intent: SAMPLE_INTENT, detection: SAMPLE_DETECTION },
      });

      expect(captured?.enforcement.source).toBe('custom-pdp');
    });
  });

  describe('best-effort isolation', () => {
    it('never lets a telemetry failure affect a successful verification', async () => {
      const throwingJoiner = new CorrelationJoiner();
      vi.spyOn(throwingJoiner, 'ingestEnforcement').mockImplementation(() => {
        throw new Error('joiner boom');
      });
      const client = new AIMClient({ telemetry: { enabled: true, joiner: throwingJoiner } });
      client.setCredentials(await realCredentials());
      mockVerifyResponse({ actionAllowed: true });

      const result = await client.verifyAction({ action: 'db:read', resource: 'users' });
      expect(result.actionAllowed).toBe(true);
    });

    it('still throws ActionDeniedError when telemetry fails on a deny', async () => {
      const throwingJoiner = new CorrelationJoiner();
      vi.spyOn(throwingJoiner, 'ingestEnforcement').mockImplementation(() => {
        throw new Error('joiner boom');
      });
      const client = new AIMClient({ telemetry: { enabled: true, joiner: throwingJoiner } });
      client.setCredentials(await realCredentials());
      mockVerifyResponse({ actionAllowed: false, denialReason: 'denied' });

      await expect(
        client.verifyAction({ action: 'file:write', resource: '/etc/passwd' })
      ).rejects.toThrow(ActionDeniedError);
    });
  });

  describe('lifecycle', () => {
    it('closeTelemetry is safe with no internally managed joiner', () => {
      const client = new AIMClient({ telemetry: { enabled: true } });
      expect(() => client.closeTelemetry()).not.toThrow();
    });
  });
});
