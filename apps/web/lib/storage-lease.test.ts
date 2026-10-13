import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * The turn tabs take through localStorage where the browser has no Web Locks.
 *
 * A tab here is a separate instance of the module: its own claim name and the
 * same localStorage, which is what two documents of one origin share.
 */

type Tab = typeof import("@/lib/storage-lease");

async function openTab(): Promise<Tab> {
  vi.resetModules();
  return import("@/lib/storage-lease");
}

const KEY = "turn";

// A lease long enough that none runs out while a test waits on it.
const TIMINGS = { leaseMs: 10_000, settleMs: 20, pollMs: 10 };

const wait = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

/** The lease another tab would hold, expiring `ms` from now. */
const leaseOfAnotherTab = (ms: number) => `another-tab.1:${Date.now() + ms}`;

/** Work that counts how many of its runs are under way at once. */
function recorder() {
  const log = { runs: 0, mostAtOnce: 0 };
  let running = 0;
  const work = (ms: number) => async () => {
    running += 1;
    log.runs += 1;
    log.mostAtOnce = Math.max(log.mostAtOnce, running);
    await wait(ms);
    running -= 1;
    return log.runs;
  };
  return { log, work };
}

beforeEach(() => {
  localStorage.clear();
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("a lease in localStorage", () => {
  it("two tabs that ask together work one after the other", async () => {
    const first = await openTab();
    const second = await openTab();
    const { log, work } = recorder();

    const results = await Promise.all([
      first.withStorageLease(KEY, work(40), TIMINGS),
      second.withStorageLease(KEY, work(40), TIMINGS),
    ]);

    expect(results).toEqual([1, 2]);
    expect(log).toEqual({ runs: 2, mostAtOnce: 1 });
    expect(localStorage.getItem(KEY)).toBeNull();
  });

  it("two callers in one tab work one after the other", async () => {
    const tab = await openTab();
    const { log, work } = recorder();

    await Promise.all([
      tab.withStorageLease(KEY, work(40), TIMINGS),
      tab.withStorageLease(KEY, work(40), TIMINGS),
    ]);

    expect(log).toEqual({ runs: 2, mostAtOnce: 1 });
  });

  it("a tab waits while another tab holds the lease and works once it is released", async () => {
    const tab = await openTab();
    const { log, work } = recorder();
    localStorage.setItem(KEY, leaseOfAnotherTab(TIMINGS.leaseMs));

    const pending = tab.withStorageLease(KEY, work(1), TIMINGS);
    await wait(120);
    expect(log.runs).toBe(0);

    localStorage.removeItem(KEY);
    await pending;
    expect(log.runs).toBe(1);
    expect(localStorage.getItem(KEY)).toBeNull();
  });

  it("a claim that another tab's claim replaced before it settled gives way", async () => {
    const tab = await openTab();
    const { log, work } = recorder();

    const pending = tab.withStorageLease(KEY, work(1), TIMINGS);
    expect(localStorage.getItem(KEY)).not.toBeNull();
    // The claim another tab wrote at the same moment arrives after this tab's own.
    localStorage.setItem(KEY, leaseOfAnotherTab(TIMINGS.leaseMs));
    await wait(120);
    expect(log.runs).toBe(0);

    localStorage.removeItem(KEY);
    await pending;
    expect(log.runs).toBe(1);
  });

  it("a lease left by a tab that was closed stops the others only until it runs out", async () => {
    const tab = await openTab();
    const expires = Date.now() + 150;
    localStorage.setItem(KEY, `another-tab.1:${expires}`);

    let startedAt = 0;
    await tab.withStorageLease(KEY, async () => void (startedAt = Date.now()), TIMINGS);

    expect(startedAt).toBeGreaterThanOrEqual(expires);
    expect(localStorage.getItem(KEY)).toBeNull();
  });

  it.each([
    ["text that is not a lease", () => "not-a-lease"],
    ["a lease with no claim", () => `:${Date.now() + 1_000}`],
    ["a lease with no expiry", () => "another-tab.1:"],
    ["a lease that has run out", () => leaseOfAnotherTab(-1)],
    ["a lease that claims to last longer than a lease can", () => leaseOfAnotherTab(3_600_000)],
  ])("%s holds nothing", async (_name, value) => {
    const tab = await openTab();
    const { work } = recorder();
    localStorage.setItem(KEY, value());

    const outcome = await Promise.race([
      tab.withStorageLease(KEY, work(1), TIMINGS),
      wait(1_000).then(() => "still waiting"),
    ]);

    expect(outcome).toBe(1);
  });

  it("the holder renews its lease, so work that outlasts a lease is not joined by another tab", async () => {
    const first = await openTab();
    const second = await openTab();
    const { log, work } = recorder();
    const short = { leaseMs: 400, settleMs: 20, pollMs: 10 };
    vi.useFakeTimers();

    const long = first.withStorageLease(KEY, work(1_000), short);
    await vi.advanceTimersByTimeAsync(50);
    const next = second.withStorageLease(KEY, work(1), short);
    // More than two leases later the first tab is still at work.
    await vi.advanceTimersByTimeAsync(900);
    expect(log).toEqual({ runs: 1, mostAtOnce: 1 });

    await vi.advanceTimersByTimeAsync(200);
    await Promise.all([long, next]);
    expect(log).toEqual({ runs: 2, mostAtOnce: 1 });
  });

  it("the lease is released when the work fails", async () => {
    const tab = await openTab();

    await expect(
      tab.withStorageLease(
        KEY,
        async () => {
          throw new Error("refused");
        },
        TIMINGS
      )
    ).rejects.toThrow("refused");

    expect(localStorage.getItem(KEY)).toBeNull();
  });

  it("a tab whose lease another tab took over leaves that tab's lease in place", async () => {
    const tab = await openTab();
    const taken = leaseOfAnotherTab(TIMINGS.leaseMs);

    await tab.withStorageLease(KEY, async () => localStorage.setItem(KEY, taken), TIMINGS);

    expect(localStorage.getItem(KEY)).toBe(taken);
  });

  it("the work runs once, without a lease, where the store cannot be written", async () => {
    const tab = await openTab();
    const { log, work } = recorder();
    vi.spyOn(Object.getPrototypeOf(localStorage), "setItem").mockImplementation(() => {
      throw new DOMException("The quota has been exceeded.", "QuotaExceededError");
    });

    await expect(tab.withStorageLease(KEY, work(1), TIMINGS)).resolves.toBe(1);
    expect(log.runs).toBe(1);
  });
});
