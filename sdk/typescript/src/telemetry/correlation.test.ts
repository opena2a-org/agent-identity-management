/**
 * Tests for the causal-denial correlation envelope.
 */
import { describe, it, expect } from 'vitest';
import {
  CORRELATION_HEADER,
  mintCorrelationId,
  isCorrelationId,
  correlationHeaders,
  extractCorrelationId,
} from './correlation';

describe('mintCorrelationId', () => {
  it('produces a well-formed, prefixed ID', () => {
    const id = mintCorrelationId();
    expect(id.startsWith('cde_')).toBe(true);
    expect(isCorrelationId(id)).toBe(true);
  });

  it('produces unique IDs', () => {
    const ids = new Set(Array.from({ length: 1000 }, () => mintCorrelationId()));
    expect(ids.size).toBe(1000);
  });

  it('is lexicographically ordered by time', () => {
    const earlier = mintCorrelationId();
    // The timestamp prefix is fixed-width, so string order tracks time order.
    const later = `cde_${'z'.repeat(9)}_${'f'.repeat(12)}`;
    expect(earlier < later).toBe(true);
  });
});

describe('isCorrelationId', () => {
  it('rejects malformed values', () => {
    expect(isCorrelationId(undefined)).toBe(false);
    expect(isCorrelationId('')).toBe(false);
    expect(isCorrelationId('cde_short')).toBe(false);
    expect(isCorrelationId('xyz_000000000_aabbccddeeff')).toBe(false);
    expect(isCorrelationId(42)).toBe(false);
  });
});

describe('correlationHeaders', () => {
  it('attaches the header for a valid ID', () => {
    const id = mintCorrelationId();
    expect(correlationHeaders(id)).toEqual({ [CORRELATION_HEADER]: id });
  });

  it('returns an empty object for an invalid ID (spreadable)', () => {
    expect(correlationHeaders(undefined)).toEqual({});
    expect(correlationHeaders('garbage')).toEqual({});
  });
});

describe('extractCorrelationId', () => {
  it('reads the header case-insensitively', () => {
    const id = mintCorrelationId();
    expect(extractCorrelationId({ 'X-OpenA2A-Correlation-Id': id })).toBe(id);
    expect(extractCorrelationId({ [CORRELATION_HEADER]: id })).toBe(id);
  });

  it('handles array-valued headers', () => {
    const id = mintCorrelationId();
    expect(extractCorrelationId({ [CORRELATION_HEADER]: [id, 'other'] })).toBe(id);
  });

  it('returns undefined when absent or malformed', () => {
    expect(extractCorrelationId(undefined)).toBeUndefined();
    expect(extractCorrelationId({})).toBeUndefined();
    expect(extractCorrelationId({ [CORRELATION_HEADER]: 'garbage' })).toBeUndefined();
  });
});
