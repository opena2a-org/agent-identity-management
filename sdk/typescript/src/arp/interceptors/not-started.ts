/**
 * Warn that an interceptor's scan method was called before start().
 *
 * The scan still returns `scanned: false`; this line names the fix so the
 * caller does not read that result's `detected: false` as a verdict. Each
 * interceptor instance prints it once, and again only after a later start()
 * and stop().
 */
export function warnNotStarted(interceptor: string, method: string): void {
  // eslint-disable-next-line no-console
  console.warn(
    `[ARP] ${interceptor}.${method}() was called while it is not started, so nothing was scanned (scanned: false). ` +
      `Call await start() on the ${interceptor} first.`,
  );
}
