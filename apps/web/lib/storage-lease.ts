/**
 * One holder at a time across the tabs of an origin, for a browser without the
 * Web Locks API: a page served over plain HTTP from a host other than localhost
 * has none. The turn is a lease kept in localStorage, which those tabs share.
 *
 * localStorage has no compare-and-set, and a write made in one tab reaches the
 * others a moment later. So a tab claims the lease by writing its claim,
 * waiting for a claim another tab wrote at the same moment to arrive, and
 * reading the lease back: the claim that is still there holds the turn, and the
 * other tab waits. This narrows the overlap to two claims that take longer than
 * the wait to reach each other; it does not rule that out, as a Web Lock does.
 *
 * A lease expires, and its holder renews it while it works, so a tab closed in
 * the middle of its turn stops the others only until the lease runs out.
 */

export type LeaseTimings = {
  /** How long a lease is honoured after its holder last wrote it. */
  leaseMs: number;
  /** How long a claim waits for a claim written at the same moment to arrive. */
  settleMs: number;
  /** How often a waiting tab reads the lease again. */
  pollMs: number;
};

export const LEASE_TIMINGS: LeaseTimings = { leaseMs: 10_000, settleMs: 50, pollMs: 50 };

type Lease = { claim: string; expires: number };

// This tab's name, and how many claims it has made: a claim is named after
// both, so two callers in one tab hold two claims.
let tab: string | null = null;
let claims = 0;

function nextClaim(): string {
  if (tab === null) {
    const words = new Uint32Array(2);
    crypto.getRandomValues(words);
    tab = Array.from(words, (word) => word.toString(36)).join("");
  }
  claims += 1;
  return `${tab}.${claims}`;
}

function readLease(key: string): Lease | null {
  const value = localStorage.getItem(key);
  const cut = value ? value.lastIndexOf(":") : -1;
  if (!value || cut < 1) return null;
  const expires = Number(value.slice(cut + 1));
  return Number.isFinite(expires) ? { claim: value.slice(0, cut), expires } : null;
}

function writeLease(key: string, claim: string, leaseMs: number): void {
  localStorage.setItem(key, `${claim}:${Date.now() + leaseMs}`);
}

// A lease written here expires within leaseMs of now. One that has run out, or
// that claims to last longer than a lease can, holds nothing.
function isLive(lease: Lease | null, leaseMs: number): lease is Lease {
  const now = Date.now();
  return lease !== null && lease.expires > now && lease.expires <= now + leaseMs;
}

const sleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

async function claimLease(key: string, claim: string, timings: LeaseTimings): Promise<void> {
  for (;;) {
    if (isLive(readLease(key), timings.leaseMs)) {
      await sleep(timings.pollMs);
      continue;
    }
    writeLease(key, claim, timings.leaseMs);
    await sleep(timings.settleMs);
    const settled = readLease(key);
    if (isLive(settled, timings.leaseMs) && settled.claim === claim) return;
  }
}

// Renewing and releasing touch the lease only while it still names this claim.
function renewLease(key: string, claim: string, leaseMs: number): void {
  try {
    if (readLease(key)?.claim === claim) writeLease(key, claim, leaseMs);
  } catch {
    // The lease runs out on its own.
  }
}

function releaseLease(key: string, claim: string): void {
  try {
    if (readLease(key)?.claim === claim) localStorage.removeItem(key);
  } catch {
    // The lease runs out on its own.
  }
}

/**
 * Runs `work` once, while this caller holds the lease stored under `key`. Where
 * the store cannot be read or written there is nothing to share a turn through,
 * and the work runs without one.
 */
export async function withStorageLease<T>(
  key: string,
  work: () => Promise<T>,
  timings: LeaseTimings = LEASE_TIMINGS
): Promise<T> {
  let claim: string;
  try {
    claim = nextClaim();
    await claimLease(key, claim, timings);
  } catch {
    return work();
  }
  const renewal = setInterval(() => renewLease(key, claim, timings.leaseMs), timings.leaseMs / 4);
  try {
    return await work();
  } finally {
    clearInterval(renewal);
    releaseLease(key, claim);
  }
}
