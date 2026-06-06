/**
 * Tests for the local correlated-record writer. Key guarantees: round-trip
 * persistence, best-effort writes that never throw, and oversize truncation
 * that preserves the authoritative enforcement fact.
 */
import { describe, it, expect, afterEach } from 'vitest';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { writeCorrelatedRecord, readCorrelatedRecords } from './local-writer';
import { buildCorrelatedRecord, type BuildCorrelatedRecordInput } from './correlated-record';

const tmpDirs: string[] = [];

function freshDir(): string {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'cde-telemetry-'));
  tmpDirs.push(dir);
  return dir;
}

afterEach(() => {
  for (const dir of tmpDirs.splice(0)) {
    try {
      fs.rmSync(dir, { recursive: true, force: true });
    } catch {
      // best-effort cleanup
    }
  }
});

const input = (): BuildCorrelatedRecordInput => ({
  correlationId: 'cde_000000000_aabbccddeeff',
  agentId: 'agent-1',
  enforcement: {
    decision: 'deny',
    outcome: 'DENY',
    capability: 'net:connect',
    resource: 'https://evil.example',
    occurredAt: '2026-06-05T00:00:00.000Z',
    source: 'aim-pdp',
  },
});

describe('writeCorrelatedRecord', () => {
  it('appends and reads back a record', () => {
    const dir = freshDir();
    const record = buildCorrelatedRecord(input());
    expect(writeCorrelatedRecord(record, dir)).toBe(true);

    const read = readCorrelatedRecords(dir);
    expect(read).toHaveLength(1);
    expect(read[0].correlationId).toBe(record.correlationId);
    expect(read[0].enforcement.decision).toBe('deny');
  });

  it('appends multiple records in order', () => {
    const dir = freshDir();
    writeCorrelatedRecord(buildCorrelatedRecord(input()), dir);
    writeCorrelatedRecord(buildCorrelatedRecord(input()), dir);
    expect(readCorrelatedRecords(dir)).toHaveLength(2);
  });

  it('never throws and returns false when the target is unusable', () => {
    // Point the data dir at a regular file so mkdir/append cannot succeed.
    const fileAsDir = path.join(freshDir(), 'not-a-dir');
    fs.writeFileSync(fileAsDir, 'x');
    const nested = path.join(fileAsDir, 'sub');
    let result = true;
    expect(() => {
      result = writeCorrelatedRecord(buildCorrelatedRecord(input()), nested);
    }).not.toThrow();
    expect(result).toBe(false);
  });

  it('truncates an oversize record but keeps the enforcement fact', () => {
    const dir = freshDir();
    const big = buildCorrelatedRecord({
      ...input(),
      enforcement: { ...input().enforcement, deniedReason: 'x'.repeat(2 * 1024 * 1024) },
      detection: {
        injectionDetected: true,
        techniqueId: 'T-2002',
        techniqueSource: 'interim-mapping',
        confidence: 0.9,
        detector: 'nanomind-guard',
        detectedAt: '2026-06-05T00:00:00.000Z',
      },
    });
    expect(writeCorrelatedRecord(big, dir)).toBe(true);

    const read = readCorrelatedRecords(dir);
    expect(read).toHaveLength(1);
    expect(read[0].correlationId).toBe(big.correlationId);
    expect(read[0].enforcement.decision).toBe('deny');
    // Oversize detail dropped on truncation.
    expect(read[0].detection).toBeUndefined();
    expect(read[0].assembly.completeness).toBe('partial-missing-both');
  });

  it('returns an empty array when no log exists', () => {
    expect(readCorrelatedRecords(freshDir())).toEqual([]);
  });
});
