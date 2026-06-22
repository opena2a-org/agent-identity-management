/**
 * Asynchronously-refreshed revocation-list (CRL) cache for local ATX verification.
 *
 * Revocation is the one credential check designed to ride on a short-lived, cached
 * list (AAP §6): the decision path never makes a network call. A background timer
 * refreshes the list on an interval; {@link CrlCache.current} returns the cached
 * list while it is within its TTL and `null` once it goes stale.
 *
 * By default a stale cache *soft-fails open on revocation only* — signature, expiry
 * and issuer-trust stay hard — so a transient CRL-feed outage degrades revocation
 * freshness instead of taking down all verification. The TTL bounds the window in
 * which a revoked credential could slip through; set `onStale: 'reject'` for a
 * fail-closed deployment where availability-over-revocation-freshness is not
 * acceptable.
 */

/** A federated certificate revocation list. */
export interface CrlData {
  entries: Array<{ agentId: string; reason?: string }>;
}

/** What a verifier should do when the cached CRL is stale (beyond TTL). */
export type CrlStalePolicy = 'soft-open' | 'reject';

export interface CrlCacheConfig {
  /** Fetches the current CRL. Invoked only off the decision path (background timer + `refreshNow`). */
  fetch: () => Promise<CrlData>;
  /** Max age, in ms, that a cached CRL is served. After this, `current()` returns null. Default 300_000 (5 min). */
  ttlMs?: number;
  /** Background refresh interval, in ms. Default `max(1, floor(ttlMs / 2))`. */
  refreshIntervalMs?: number;
  /** Behavior when the cache is stale: `'soft-open'` (default) skips revocation; `'reject'` fails closed. */
  onStale?: CrlStalePolicy;
  /** Injectable clock (tests). Defaults to `Date.now`. */
  now?: () => number;
  /** Sink for refresh errors / degraded-state notices. Defaults to `console.warn`. */
  onError?: (err: unknown) => void;
}

export interface CrlCacheStatus {
  /** A list has been successfully fetched at least once. */
  loaded: boolean;
  /** Age of the cached list in ms, or null if never loaded. */
  ageMs: number | null;
  /** Loaded and within TTL. */
  fresh: boolean;
  /** The most recent refresh error, or null. */
  lastError: unknown;
}

export class CrlCache {
  private readonly fetchFn: () => Promise<CrlData>;
  private readonly ttlMs: number;
  private readonly refreshIntervalMs: number;
  /** Policy a verifier applies when this cache is stale. */
  readonly onStale: CrlStalePolicy;
  private readonly now: () => number;
  private readonly onError: (err: unknown) => void;

  private data: CrlData | null = null;
  private fetchedAt: number | null = null;
  private lastError: unknown = null;
  private timer: ReturnType<typeof setInterval> | null = null;
  private inFlight: Promise<void> | null = null;

  constructor(config: CrlCacheConfig) {
    if (typeof config.fetch !== 'function') {
      throw new TypeError('CrlCache requires a fetch function');
    }
    this.fetchFn = config.fetch;
    this.ttlMs = config.ttlMs ?? 300_000;
    if (!(this.ttlMs > 0)) {
      throw new RangeError('CrlCache ttlMs must be > 0');
    }
    this.refreshIntervalMs = config.refreshIntervalMs ?? Math.max(1, Math.floor(this.ttlMs / 2));
    if (!(this.refreshIntervalMs > 0)) {
      throw new RangeError('CrlCache refreshIntervalMs must be > 0');
    }
    this.onStale = config.onStale ?? 'soft-open';
    this.now = config.now ?? Date.now;
    this.onError =
      config.onError ??
      ((err) => console.warn('[CrlCache] refresh failed; serving cached list until TTL:', err));
  }

  /**
   * Fetch the CRL once and update the cache. Resolves even when the fetch fails — the
   * error is captured ({@link status}) and the previous list is retained until it goes
   * stale. Concurrent calls are coalesced into a single in-flight fetch.
   */
  async refreshNow(): Promise<void> {
    if (this.inFlight) {
      return this.inFlight;
    }
    this.inFlight = (async () => {
      try {
        const next = await this.fetchFn();
        if (!next || !Array.isArray(next.entries)) {
          throw new TypeError('CRL fetch returned an invalid shape (expected { entries: [...] })');
        }
        this.data = next;
        this.fetchedAt = this.now();
        this.lastError = null;
      } catch (err) {
        this.lastError = err;
        this.onError(err);
      } finally {
        this.inFlight = null;
      }
    })();
    return this.inFlight;
  }

  /**
   * Start the background refresh timer (idempotent) and kick off an immediate refresh.
   * On Node the timer is `unref`-ed so it never keeps the process alive on its own.
   */
  start(): void {
    if (this.timer) {
      return;
    }
    void this.refreshNow();
    this.timer = setInterval(() => void this.refreshNow(), this.refreshIntervalMs);
    (this.timer as { unref?: () => void } | undefined)?.unref?.();
  }

  /** Stop the background refresh timer (idempotent). */
  stop(): void {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }

  /** Whether the cache holds a list that is within its TTL. */
  isFresh(): boolean {
    return this.fetchedAt !== null && this.now() - this.fetchedAt <= this.ttlMs;
  }

  /**
   * The cached CRL while fresh (within TTL), otherwise `null`. Synchronous and never
   * throws — safe on the verification decision path. A `null` return means the caller
   * must apply its {@link onStale} policy (soft-open: skip revocation; reject: fail closed).
   */
  current(): CrlData | null {
    return this.isFresh() ? this.data : null;
  }

  /** Observability snapshot; never throws. */
  status(): CrlCacheStatus {
    const ageMs = this.fetchedAt === null ? null : this.now() - this.fetchedAt;
    return { loaded: this.data !== null, ageMs, fresh: this.isFresh(), lastError: this.lastError };
  }
}
