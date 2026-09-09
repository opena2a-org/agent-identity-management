/**
 * Tests for wire-error handling on the verify path (AIM-11 items 4 and 5):
 *
 * - RateLimitError.retryAfter reflects the server's Retry-After header, falling
 *   back to the body field then a default only when absent.
 * - A wire 403 on /api/v1/verify with telemetry enabled produces a correlated
 *   deny enforcement record exactly as the 200 {actionAllowed:false} path does;
 *   an ALLOWED verify carrying a detection produces one too.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { AIMClient } from './AIMClient';
import { AuthorizationError, RateLimitError, parseAPIError } from '../exceptions';
import { CorrelationJoiner, type CorrelatedRecord, type DetectionInput } from '../telemetry';
import { generateKeyPair, toBase64 } from '../crypto/ed25519';
import type { AgentCredentials, VerificationResult } from '../types';

const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

async function realCredentials(agentId = 'agent-wire-1'): Promise<AgentCredentials> {
  const { privateKey, publicKey } = await generateKeyPair();
  return {
    agentId,
    privateKey: toBase64(privateKey),
    publicKey: toBase64(publicKey),
    organizationId: 'org-wire',
    createdAt: new Date().toISOString(),
  };
}

function mockTokenOk(): void {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    json: async () => ({ accessToken: 'test-token', tokenType: 'Bearer', expiresIn: 3600 }),
  });
}

function mockVerifyOk(overrides: Partial<VerificationResult> = {}): void {
  const body: VerificationResult = {
    verified: true,
    agentId: 'agent-wire-1',
    agentName: 'wire-agent',
    trustScore: 0.9,
    riskLevel: 'low' as VerificationResult['riskLevel'],
    actionAllowed: true,
    timestamp: '2026-09-09T00:00:00Z',
    eventId: 'evt-wire',
    ...overrides,
  };
  mockFetch.mockResolvedValueOnce({
    ok: true,
    text: async () => JSON.stringify(body),
  });
}

function mockVerifyFailure(
  status: number,
  body: Record<string, unknown>,
  headers?: Record<string, string>
): void {
  mockFetch.mockResolvedValueOnce({
    ok: false,
    status,
    headers: new Headers(headers ?? {}),
    json: async () => body,
  });
}

const SAMPLE_DETECTION: DetectionInput = {
  injectionDetected: true,
  attackClass: 'indirect',
  techniqueId: 'T-2002',
  techniqueSource: 'interim-mapping',
  confidence: 0.84,
  detector: 'nanomind-guard',
  inputRef: 'sha256:abc',
  detectedAt: '2026-09-09T00:00:00Z',
};

beforeEach(() => {
  vi.clearAllMocks();
  delete process.env.AIM_BASE_URL;
  delete process.env.AIM_API_KEY;
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.stubGlobal('fetch', mockFetch);
});

describe('Retry-After (item 4)', () => {
  it('AIM-11.AC4 a 429 with "Retry-After: 7" yields RateLimitError.retryAfter 7', async () => {
    const client = new AIMClient();
    client.setCredentials(await realCredentials());
    mockTokenOk();
    mockVerifyFailure(429, { message: 'rate limited' }, { 'Retry-After': '7' });

    let caught: unknown;
    try {
      await client.verifyAction({ action: 'db:read', resource: 'users' });
    } catch (err) {
      caught = err;
    }
    expect(caught).toBeInstanceOf(RateLimitError);
    expect((caught as RateLimitError).retryAfter).toBe(7);
  });

  it('AIM-11.AC4 falls back to the body retryAfter field when the header is absent', () => {
    const err = parseAPIError(429, { retryAfter: 13 });
    expect(err).toBeInstanceOf(RateLimitError);
    expect((err as RateLimitError).retryAfter).toBe(13);
  });

  it('AIM-11.AC4 falls back to the default only when both header and body are absent', () => {
    const err = parseAPIError(429, {});
    expect(err).toBeInstanceOf(RateLimitError);
    expect((err as RateLimitError).retryAfter).toBe(60);
  });

  it('AIM-11.AC4 the header wins over the body field when both are present', async () => {
    const client = new AIMClient();
    client.setCredentials(await realCredentials());
    mockTokenOk();
    mockVerifyFailure(429, { retryAfter: 999 }, { 'Retry-After': '7' });

    let caught: unknown;
    try {
      await client.verifyAction({ action: 'db:read', resource: 'users' });
    } catch (err) {
      caught = err;
    }
    expect((caught as RateLimitError).retryAfter).toBe(7);
  });
});

describe('telemetry on wire 403 (item 5)', () => {
  it('AIM-11.AC5 a wire 403 on /api/v1/verify with telemetry enabled produces a correlated deny record', async () => {
    const records: CorrelatedRecord[] = [];
    const joiner = new CorrelationJoiner({ windowMs: 60_000, onRecord: (r) => records.push(r) });
    const client = new AIMClient({ telemetry: { enabled: true, joiner } });
    client.setCredentials(await realCredentials());
    mockTokenOk();
    mockVerifyFailure(403, { message: 'agent suspended' });

    await expect(
      client.verifyAction({ action: 'file:write', resource: '/etc/passwd' })
    ).rejects.toThrow(AuthorizationError);

    // The enforcement fact must have been ingested (buffered awaiting
    // intent/detection) exactly as the 200 {actionAllowed:false} path buffers.
    expect(joiner.pending).toBe(1);

    // Force the window to expire so the record is emitted, and check the outcome.
    joiner.flushExpired(Date.now() + 120_000);
    expect(records).toHaveLength(1);
    expect(records[0].enforcement.decision).toBe('deny');
    expect(records[0].enforcement.outcome).toBe('DENY');
    expect(records[0].enforcement.capability).toBe('file:write');
  });

  it('AIM-11.AC5 an allowed verify carrying a detection produces an enforcement record too', async () => {
    const records: CorrelatedRecord[] = [];
    const joiner = new CorrelationJoiner({ windowMs: 60_000, onRecord: (r) => records.push(r) });
    const client = new AIMClient({ telemetry: { enabled: true, joiner } });
    client.setCredentials(await realCredentials());
    mockTokenOk();
    mockVerifyOk({ actionAllowed: true });

    await client.verifyAction({
      action: 'file:read',
      resource: '/data/report.json',
      telemetry: { detection: SAMPLE_DETECTION },
    });

    expect(joiner.pending).toBe(1);
    joiner.flushExpired(Date.now() + 120_000);
    expect(records).toHaveLength(1);
    expect(records[0].enforcement.decision).toBe('allow');
    expect(records[0].detection?.techniqueId).toBe('T-2002');
  });
});
