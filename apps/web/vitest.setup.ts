/**
 * Test-environment storage.
 *
 * jsdom 27 under this Node version does not expose `window.localStorage` even with a
 * real origin configured — Node's own experimental `localStorage` is gated behind
 * `--localstorage-file` and jsdom's is absent, so anything touching storage failed
 * with "Cannot read properties of undefined (reading 'clear')".
 *
 * This installs a `Storage`-shaped shim ONLY when one is missing, so a future
 * environment that provides the real thing is used instead of being shadowed. The
 * behaviours the tests depend on are the ones the spec defines and code relies on:
 * values are coerced to strings, a missing key reads as `null` (not `undefined`), and
 * `removeItem`/`clear` actually drop keys.
 */
class MemoryStorage implements Storage {
  private map = new Map<string, string>();

  get length(): number {
    return this.map.size;
  }

  clear(): void {
    this.map.clear();
  }

  getItem(key: string): string | null {
    // Must be null, not undefined: callers do `Number(getItem(k)) || fallback` and
    // `=== null` checks, and undefined would change both.
    const v = this.map.get(String(key));
    return v === undefined ? null : v;
  }

  key(index: number): string | null {
    return Array.from(this.map.keys())[index] ?? null;
  }

  removeItem(key: string): void {
    this.map.delete(String(key));
  }

  setItem(key: string, value: string): void {
    this.map.set(String(key), String(value));
  }
}

function install(name: "localStorage" | "sessionStorage") {
  const g = globalThis as unknown as Record<string, unknown>;
  const existing = g[name] as Storage | undefined;
  // Treat a present-but-broken implementation as absent.
  let usable = false;
  try {
    usable = !!existing && typeof existing.setItem === "function";
  } catch {
    usable = false;
  }
  if (usable) return;

  const store = new MemoryStorage();
  Object.defineProperty(globalThis, name, {
    value: store,
    configurable: true,
    writable: true,
  });
  if (typeof window !== "undefined") {
    Object.defineProperty(window, name, {
      value: store,
      configurable: true,
      writable: true,
    });
  }
}

install("localStorage");
install("sessionStorage");
