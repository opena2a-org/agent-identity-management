import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { mkdtempSync, rmSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import * as ed25519 from '@noble/ed25519';
import { buildCanonical, generateNonce, buildSignedSubmission, SCHEMA_VERSION } from './wire';
import type { RedactedSignal } from './redaction';

// Registry wire invariants — copied verbatim from
// opena2a-registry/internal/domain/telemetry_signature.go so a producer drift
// fails HERE, before it would be rejected at ingestion.
const RE_BEHAVIORAL_HASH = /^[a-f0-9]{64}$/;
const RE_TECHNIQUE_ID = /^[A-Z][A-Z0-9]{1,9}(-[A-Z0-9]{1,9}){0,3}$/;
const RE_ORG_PSEUDONYM = /^[a-f0-9]{16,64}$/;
const RE_NONCE = /^[A-Za-z0-9_-]{8,128}$/;
const VALID_SEVERITIES = new Set(['critical', 'high', 'medium', 'low', 'info']);
const VALID_OUTCOMES = new Set(['detected', 'blocked', 'allowed']);
const RE_UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

const signal: RedactedSignal = {
  tacticId: 'exfiltration',
  techniqueId: 'ATM-T8002',
  actionClass: 'network_egress',
  targetClass: 'network',
  sequencePattern: '',
  outcomeClass: 'blocked',
  severity: 'high',
};

let home: string;
beforeAll(() => {
  home = mkdtempSync(join(tmpdir(), 'arp-tel-'));
  process.env.OPENA2A_HOME = home;
});
afterAll(() => {
  delete process.env.OPENA2A_HOME;
  rmSync(home, { recursive: true, force: true });
});

describe('buildCanonical — byte-parity with the Go server Canonical()', () => {
  it('joins fields in the exact registry order', () => {
    const c = buildCanonical({
      schemaVersion: 'telemetry-sig-v1',
      behavioralHash: 'a'.repeat(64),
      techniqueId: 'ATM-T8002',
      severity: 'high',
      outcome: 'blocked',
      sensorId: '11111111-1111-1111-1111-111111111111',
      orgPseudonym: 'b'.repeat(32),
      count: 3,
      nonce: 'NONCE_value-1',
      signedAt: 1750000000,
    });
    expect(c).toBe(
      'telemetry-sig-v1|' + 'a'.repeat(64) + '|ATM-T8002|high|blocked|' +
        '11111111-1111-1111-1111-111111111111|' + 'b'.repeat(32) + '|3|NONCE_value-1|1750000000',
    );
  });

  it('normalizes a zero/omitted count to 1 (mirrors Go normalizedCount)', () => {
    const c = buildCanonical({
      schemaVersion: 'telemetry-sig-v1', behavioralHash: 'a'.repeat(64), techniqueId: 'ATM-T1',
      severity: 'low', outcome: 'detected', sensorId: '11111111-1111-1111-1111-111111111111',
      orgPseudonym: 'b'.repeat(32), count: 0, nonce: 'abcdefgh', signedAt: 1,
    });
    expect(c.split('|')[7]).toBe('1');
  });
});

describe('generateNonce', () => {
  it('matches the registry nonce floor', () => {
    for (let i = 0; i < 50; i++) expect(generateNonce()).toMatch(RE_NONCE);
  });
});

describe('buildSignedSubmission', () => {
  it('emits a request that satisfies EVERY registry field invariant', async () => {
    const { request, body } = await buildSignedSubmission(signal, { count: 2 });
    expect(request.schemaVersion).toBe(SCHEMA_VERSION);
    expect(request.behavioralHash).toMatch(RE_BEHAVIORAL_HASH);
    expect(request.techniqueId).toMatch(RE_TECHNIQUE_ID);
    expect(request.techniqueId.length).toBeLessThanOrEqual(32);
    expect(VALID_SEVERITIES.has(request.severity)).toBe(true);
    expect(VALID_OUTCOMES.has(request.outcome)).toBe(true);
    expect(request.count).toBeGreaterThanOrEqual(1);
    expect(request.sensorId).toMatch(RE_UUID);
    expect(request.orgPseudonym).toMatch(RE_ORG_PSEUDONYM);
    expect(request.nonce).toMatch(RE_NONCE);
    expect(request.signedAt).toBeGreaterThan(0);
    expect(request.publicKey.length).toBeLessThanOrEqual(256);
    expect(request.signature.length).toBeLessThanOrEqual(256);
    // hex envelope: 32-byte key -> 64 hex, 64-byte sig -> 128 hex
    expect(request.publicKey).toMatch(/^[a-f0-9]{64}$/);
    expect(request.signature).toMatch(/^[a-f0-9]{128}$/);
    // body is exactly the JSON of the request (what the audit log records)
    expect(body).toBe(JSON.stringify(request));
  });

  it('produces an Ed25519 signature the server would accept (raw-bytes verify)', async () => {
    const { request, canonical } = await buildSignedSubmission(signal, { count: 1 });
    // The server hex-decodes publicKey/signature to raw bytes, then ed25519.Verify
    // over the canonical bytes. Reproduce that exact check here.
    const pub = Buffer.from(request.publicKey, 'hex');
    const sig = Buffer.from(request.signature, 'hex');
    const msg = new TextEncoder().encode(canonical);
    expect(await ed25519.verifyAsync(sig, msg, pub)).toBe(true);
  });

  it('the canonical the producer signs reconstructs from the wire fields', async () => {
    const { request, canonical } = await buildSignedSubmission(signal, { count: 5 });
    const rebuilt = buildCanonical({
      schemaVersion: request.schemaVersion,
      behavioralHash: request.behavioralHash,
      techniqueId: request.techniqueId,
      severity: request.severity,
      outcome: request.outcome,
      sensorId: request.sensorId,
      orgPseudonym: request.orgPseudonym,
      count: request.count,
      nonce: request.nonce,
      signedAt: request.signedAt,
    });
    expect(rebuilt).toBe(canonical);
  });
});
