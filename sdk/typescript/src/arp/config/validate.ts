import type { AILayerConfig, ARPConfig, InterceptorConfig, MonitorConfig } from '../types';
import { sanitizeTerminalLine } from '../telemetry/signature/sanitize';

/**
 * Shape check for the parts of an ARP config that decide what runs: the
 * `monitors`, `interceptors` and `aiLayer` blocks. The AgentRuntimeProtection
 * constructor runs it on the config it was given or loaded, before it builds
 * anything.
 *
 * Each entry of those blocks is read as `entry?.enabled`, so a shape the types
 * do not allow was read as "not set": `monitors: { filesystem: false }` left the
 * filesystem monitor running, `aiLayer: { prompt: true }` built no prompt
 * interceptor, and an `intervalMs` of 'abc' or -5 reached setInterval, which
 * then polls every millisecond. A JavaScript caller and every arp.yaml or
 * arp.json reach the constructor without the types, so each of these throws a
 * TypeError naming the key path and the shape it takes.
 *
 * A top-level key ARPConfig does not have is warned about, not refused, so a
 * config file written for another version still starts the agent.
 *
 * Not checked here: `intelligence`, `dataDir`, `rules`, `declaredCapabilities`,
 * `gtin`, `signatureTelemetry` and `proxy`.
 */

/**
 * Lowest `intervalMs` accepted for the process and network monitors, in
 * milliseconds. The poll cost it was chosen from is measured and written in
 * tests/arp/config/validate-config.test.ts.
 */
export const MIN_MONITOR_INTERVAL_MS = 1000;

/** What a documented field of an entry holds. */
type FieldKind = 'interval' | 'positive' | 'strings';
type EntryFields = Readonly<Record<string, FieldKind>>;

const MONITOR_ENTRIES: Record<keyof MonitorConfig, EntryFields> = {
  process: { intervalMs: 'interval' },
  network: { intervalMs: 'interval', allowedHosts: 'strings' },
  filesystem: { watchPaths: 'strings', allowedPaths: 'strings' },
  skill: {},
  heartbeat: { maxStaleMs: 'positive' },
};

const INTERCEPTOR_ENTRIES: Record<keyof InterceptorConfig, EntryFields> = {
  process: {},
  network: { allowedHosts: 'strings' },
  filesystem: { allowedPaths: 'strings' },
};

const AI_LAYER_ENTRIES: Record<keyof AILayerConfig, EntryFields> = {
  prompt: {},
  mcp: { allowedTools: 'strings' },
  a2a: { trustedAgents: 'strings' },
};

const BLOCKS: ReadonlyArray<readonly [keyof ARPConfig, Readonly<Record<string, EntryFields>>]> = [
  ['monitors', MONITOR_ENTRIES],
  ['interceptors', INTERCEPTOR_ENTRIES],
  ['aiLayer', AI_LAYER_ENTRIES],
];

/** Every top-level key of ARPConfig; a key ARPConfig gains fails to compile until it is listed here. */
const TOP_LEVEL_KEYS: Record<keyof ARPConfig, true> = {
  agentName: true,
  agentDescription: true,
  declaredCapabilities: true,
  dataDir: true,
  monitors: true,
  rules: true,
  intelligence: true,
  interceptors: true,
  aiLayer: true,
  proxy: true,
  gtin: true,
  signatureTelemetry: true,
};

const ENABLED_SHAPE = "an object with a boolean 'enabled' (for example { enabled: false })";

/**
 * Throw a TypeError for the first `monitors`, `interceptors` or `aiLayer` key
 * or value that is not in its documented shape, then warn once, with
 * console.warn, about any top-level key ARPConfig does not have.
 */
export function validateARPConfig(config: unknown): void {
  if (!isRecord(config)) {
    throw new TypeError(`ARP config must be an object; got ${describe(config)}`);
  }

  for (const [block, entries] of BLOCKS) {
    validateBlock(block, config[block], entries);
  }

  const unknown = Object.keys(config).filter((key) => !hasOwn(TOP_LEVEL_KEYS, key));
  if (unknown.length > 0) {
    const names = unknown.map(showKey).join(', ');
    const verb = unknown.length === 1 ? 'is not an ARP config key' : 'are not ARP config keys';
    console.warn(
      `ARP config: ${names} ${verb} and ${unknown.length === 1 ? 'is' : 'are'} ignored. ` +
        `The keys are ${Object.keys(TOP_LEVEL_KEYS).join(', ')}.`,
    );
  }
}

function validateBlock(block: string, value: unknown, entries: Readonly<Record<string, EntryFields>>): void {
  if (value === undefined) return;
  const names = Object.keys(entries);
  if (!isRecord(value)) {
    throw new TypeError(
      `ARP config: ${block} must be an object whose keys are ${names.join(', ')}, ` +
        `each ${ENABLED_SHAPE}; got ${describe(value)}`,
    );
  }

  for (const key of Object.keys(value)) {
    const path = `${block}.${showKey(key)}`;
    if (!hasOwn(entries, key)) {
      throw new TypeError(
        `ARP config: ${path} is not a ${block} entry; the ${block} entries are ${names.join(', ')}, ` +
          `each ${ENABLED_SHAPE}`,
      );
    }

    const entry = value[key];
    if (entry === undefined) continue;
    if (!isRecord(entry)) {
      throw new TypeError(`ARP config: ${path} must be ${ENABLED_SHAPE}; got ${describe(entry)}`);
    }
    // Fields first, so { intervalMs: 'abc' } is refused for its intervalMs
    // rather than only for the missing enabled.
    for (const [field, kind] of Object.entries(entries[key])) {
      if (entry[field] !== undefined) checkField(`${path}.${field}`, kind, entry[field]);
    }
    if (typeof entry.enabled !== 'boolean') {
      throw new TypeError(`ARP config: ${path} must be ${ENABLED_SHAPE}; got ${describe(entry)}`);
    }
  }
}

function checkField(path: string, kind: FieldKind, value: unknown): void {
  switch (kind) {
    case 'interval':
      if (typeof value !== 'number' || !Number.isFinite(value) || value < MIN_MONITOR_INTERVAL_MS) {
        throw new TypeError(
          `ARP config: ${path} must be a finite number of milliseconds, at least ${MIN_MONITOR_INTERVAL_MS}; ` +
            `got ${describe(value)}`,
        );
      }
      return;
    case 'positive':
      if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) {
        throw new TypeError(
          `ARP config: ${path} must be a finite number of milliseconds above 0; got ${describe(value)}`,
        );
      }
      return;
    case 'strings':
      if (!Array.isArray(value) || !isStringArray(value)) {
        throw new TypeError(`ARP config: ${path} must be an array of strings; got ${describe(value)}`);
      }
      return;
  }
}

function isStringArray(value: unknown[]): boolean {
  for (let i = 0; i < value.length; i++) {
    if (typeof value[i] !== 'string') return false;
  }
  return true;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function hasOwn(object: object, key: string): boolean {
  return Object.prototype.hasOwnProperty.call(object, key);
}

/** A config key as one printable line: a key from a file is printed inside the message. */
function showKey(key: string): string {
  return sanitizeTerminalLine(key, 64);
}

/** A config value as one short printable line. */
function describe(value: unknown): string {
  if (typeof value === 'number' || typeof value === 'boolean' || value === undefined || value === null) {
    return String(value);
  }
  if (typeof value === 'bigint') return `${value}n`;
  if (typeof value === 'function') return 'a function';
  if (typeof value === 'symbol') return 'a symbol';
  let text: string;
  try {
    text = JSON.stringify(value) ?? String(value);
  } catch {
    text = Object.prototype.toString.call(value);
  }
  return sanitizeTerminalLine(text, 80);
}
