/**
 * What the internal CLI's `telemetry register <args>` does before anything
 * runs. `register` builds a signed enrollment proof and POSTs it, so a help
 * request or a stray argument is answered without running it.
 *
 * The decision lives here rather than inline in ./index.ts so it has a test:
 * index.ts is a top-level script (it calls main() on import), and ./telemetry.ts
 * is the shipped surface, which may not mention the internal-only subcommand.
 */

export type RegisterAction =
  | { kind: 'help' }
  | { kind: 'error'; message: string }
  | { kind: 'run' };

export function registerAction(rest: string[]): RegisterAction {
  if (rest.some((a) => a === '--help' || a === '-h')) return { kind: 'help' };
  const unknown = rest.find((a) => a.startsWith('-'));
  if (unknown !== undefined) return { kind: 'error', message: `Unknown option for register: ${unknown}` };
  const extra = rest.find((a) => !a.startsWith('-'));
  if (extra !== undefined) return { kind: 'error', message: `Unexpected argument for register: ${extra}` };
  return { kind: 'run' };
}
