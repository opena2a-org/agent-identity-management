import { describe, it, expect, vi } from 'vitest';
import { CrlCache, type CrlData } from './CrlCache';

// A controllable clock so TTL behavior is deterministic (no real timers).
function clock(start = 0) {
  let t = start;
  return { now: () => t, advance: (ms: number) => (t += ms) };
}

const LIST_A: CrlData = { entries: [{ agentId: 'agent-1', reason: 'key compromise' }] };
const LIST_B: CrlData = { entries: [{ agentId: 'agent-2' }] };

describe('CrlCache', () => {
  it('serves a fetched list while fresh and null once stale', async () => {
    const c = clock();
    const cache = new CrlCache({ fetch: async () => LIST_A, ttlMs: 1000, now: c.now });

    expect(cache.current()).toBeNull(); // nothing fetched yet
    expect(cache.status().loaded).toBe(false);

    await cache.refreshNow();
    expect(cache.current()).toEqual(LIST_A);
    expect(cache.isFresh()).toBe(true);

    c.advance(1000); // exactly at TTL is still fresh (age <= ttl)
    expect(cache.current()).toEqual(LIST_A);

    c.advance(1); // now beyond TTL
    expect(cache.current()).toBeNull();
    expect(cache.isFresh()).toBe(false);
    expect(cache.status().loaded).toBe(true); // still loaded, just stale
  });

  it('retains the previous list and records the error when a refresh fails', async () => {
    const c = clock();
    let call = 0;
    const cache = new CrlCache({
      fetch: async () => {
        call += 1;
        if (call === 1) return LIST_A;
        throw new Error('feed down');
      },
      ttlMs: 1000,
      now: c.now,
      onError: () => {}, // silence in test
    });

    await cache.refreshNow();
    expect(cache.current()).toEqual(LIST_A);

    c.advance(500);
    await cache.refreshNow(); // fails
    expect(cache.current()).toEqual(LIST_A); // previous list retained within TTL
    expect(cache.status().lastError).toBeInstanceOf(Error);

    c.advance(501); // past TTL from the original successful fetch
    expect(cache.current()).toBeNull(); // stale -> caller applies onStale
  });

  it('updates to the newest list on a successful refresh', async () => {
    const c = clock();
    let list = LIST_A;
    const cache = new CrlCache({ fetch: async () => list, ttlMs: 1000, now: c.now });
    await cache.refreshNow();
    expect(cache.current()).toEqual(LIST_A);
    list = LIST_B;
    await cache.refreshNow();
    expect(cache.current()).toEqual(LIST_B);
  });

  it('coalesces concurrent refreshes into a single fetch', async () => {
    const fetch = vi.fn(async () => {
      await new Promise((r) => setTimeout(r, 5));
      return LIST_A;
    });
    const cache = new CrlCache({ fetch, ttlMs: 1000 });
    await Promise.all([cache.refreshNow(), cache.refreshNow(), cache.refreshNow()]);
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it('rejects an invalid fetch shape without throwing on refresh', async () => {
    const cache = new CrlCache({
      // @ts-expect-error intentionally wrong shape
      fetch: async () => ({ nope: true }),
      ttlMs: 1000,
      onError: () => {},
    });
    await expect(cache.refreshNow()).resolves.toBeUndefined();
    expect(cache.current()).toBeNull();
    expect(cache.status().lastError).toBeInstanceOf(TypeError);
  });

  it('validates configuration', () => {
    // @ts-expect-error missing fetch
    expect(() => new CrlCache({})).toThrow(TypeError);
    expect(() => new CrlCache({ fetch: async () => LIST_A, ttlMs: 0 })).toThrow(RangeError);
    expect(
      () => new CrlCache({ fetch: async () => LIST_A, ttlMs: 1000, refreshIntervalMs: 0 }),
    ).toThrow(RangeError);
  });

  it('defaults onStale to soft-open and honors an explicit reject policy', () => {
    expect(new CrlCache({ fetch: async () => LIST_A }).onStale).toBe('soft-open');
    expect(new CrlCache({ fetch: async () => LIST_A, onStale: 'reject' }).onStale).toBe('reject');
  });

  it('start() refreshes immediately and stop() is idempotent', async () => {
    const fetch = vi.fn(async () => LIST_A);
    const cache = new CrlCache({ fetch, ttlMs: 10_000, refreshIntervalMs: 10_000 });
    cache.start();
    await Promise.resolve(); // let the immediate refresh settle
    await cache.refreshNow();
    expect(fetch).toHaveBeenCalled();
    cache.stop();
    cache.stop(); // no throw
  });
});
