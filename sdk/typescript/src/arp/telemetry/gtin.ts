/**
 * GTIN (Global Threat Intelligence Network) Telemetry Module
 *
 * When opted in, anonymized anomalous ARP events are shared with the
 * OpenA2A Registry to build community threat intelligence.
 *
 * PRIVACY: Only event type, package name, runtime environment, and timing
 * are transmitted. No PII, file paths, credentials, command arguments,
 * payloads, or conversation content are ever included.
 */

import { createHash, randomBytes } from 'crypto';
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'fs';
import { homedir, hostname, userInfo } from 'os';
import { join } from 'path';
import { ARPEvent, EventCategory } from '../types';
import { VERSION } from '../index';

// --- Types ---

/** GTIN event types accepted by the registry API */
export type GTINEventType =
  | 'unexpected_dns'
  | 'unexpected_network'
  | 'eval_detected'
  | 'capability_escalation'
  | 'suspicious_sequence'
  | 'idle_activation'
  | 'env_probe'
  | 'policy_parse_failure';

/** Runtime environment identifier */
export type GTINRuntimeEnv = 'node' | 'python' | 'deno';

/** Payload submitted to the GTIN telemetry endpoint */
export interface GTINPayload {
  sensorToken: string;
  eventType: GTINEventType;
  packageName: string;
  packageVersion: string;
  daySinceInstall: number;
  runtimeEnv: GTINRuntimeEnv;
  triggeredAt: string;
}

/** Result of submitting a GTIN event */
export interface GTINSubmitResult {
  success: boolean;
  eventId?: string;
  error?: string;
}

// --- Sensor Token ---

/**
 * Resolve the path to the OpenA2A home directory.
 * Respects the OPENA2A_HOME env var, defaults to ~/.opena2a.
 */
function getOpena2aHome(): string {
  return process.env.OPENA2A_HOME || join(homedir(), '.opena2a');
}

/**
 * Generate a stable per-device sensor token.
 *
 * SHA256(hostname + username + random salt stored at ~/.opena2a/gtin-sensor-salt).
 * The salt is generated once on first call and persisted locally.
 */
export function generateSensorToken(): string {
  const home = getOpena2aHome();
  const saltPath = join(home, 'gtin-sensor-salt');

  let salt: string;
  if (existsSync(saltPath)) {
    salt = readFileSync(saltPath, 'utf-8').trim();
  } else {
    salt = randomBytes(32).toString('hex');
    mkdirSync(home, { recursive: true });
    writeFileSync(saltPath, salt, { mode: 0o600 });
  }

  const input = `${hostname()}|${userInfo().username}|${salt}`;
  return createHash('sha256').update(input).digest('hex');
}

// --- Event Type Mapping ---

/**
 * Map an ARP event to a GTIN event type based on source, category, and data fields.
 *
 * Mapping logic:
 *   - network source + dns hint in data           -> unexpected_dns
 *   - network source + anomaly/violation           -> unexpected_network
 *   - process source + eval pattern in data        -> eval_detected
 *   - mcp-protocol/a2a-protocol + violation        -> capability_escalation
 *   - Any sequence detection (data.sequence)       -> suspicious_sequence
 *   - Idle activation (data.idle)                  -> idle_activation
 *   - filesystem + env variable probe              -> env_probe
 *   - Fallback for anomaly/violation/threat        -> unexpected_network
 */
export function mapEventType(event: ARPEvent): GTINEventType {
  const { source, data } = event;

  // CR-001: Policy parse failure events get their own telemetry type
  if (data.policyParseFailure) {
    return 'policy_parse_failure';
  }

  // Check for sequence detection first (can come from any source)
  if (data.sequence || data.sequenceDetected || data.patternType === 'sequence') {
    return 'suspicious_sequence';
  }

  // Check for idle activation
  if (data.idle || data.idleActivation || data.patternType === 'idle') {
    return 'idle_activation';
  }

  // Source-specific mappings
  switch (source) {
    case 'network': {
      // DNS-specific detection
      if (data.dns || data.dnsLookup || data.type === 'dns' || data.protocol === 'dns') {
        return 'unexpected_dns';
      }
      return 'unexpected_network';
    }

    case 'process': {
      // Eval pattern detection
      const desc = String(data.command || data.description || '').toLowerCase();
      if (data.eval || data.evalDetected || desc.includes('eval')) {
        return 'eval_detected';
      }
      return 'unexpected_network'; // process doing unexpected network-like activity
    }

    case 'mcp-protocol':
    case 'a2a-protocol':
      return 'capability_escalation';

    case 'filesystem': {
      // Environment variable probing
      const filePath = String(data.path || data.file || '').toLowerCase();
      if (
        data.envProbe ||
        filePath.includes('.env') ||
        filePath.includes('environment') ||
        filePath.includes('/etc/passwd') ||
        filePath.includes('credentials')
      ) {
        return 'env_probe';
      }
      return 'env_probe'; // filesystem anomalies default to env_probe
    }

    case 'prompt':
      return 'capability_escalation';

    default:
      return 'unexpected_network';
  }
}

// --- Runtime Detection ---

/** Detect the current runtime environment */
export function detectRuntimeEnv(): GTINRuntimeEnv {
  // Check for Deno
  if (typeof (globalThis as Record<string, unknown>).Deno !== 'undefined') {
    return 'deno';
  }
  // Check for Python (unlikely in this context, but included for completeness)
  if (process.env.PYTHON_VERSION || process.env.VIRTUAL_ENV) {
    return 'python';
  }
  return 'node';
}

// --- Day Since Install ---

/**
 * Calculate the number of days since ARP was first installed.
 * Uses a marker file at ~/.opena2a/gtin-install-date.
 */
export function getDaySinceInstall(): number {
  const home = getOpena2aHome();
  const markerPath = join(home, 'gtin-install-date');

  let installDate: Date;
  if (existsSync(markerPath)) {
    const stored = readFileSync(markerPath, 'utf-8').trim();
    installDate = new Date(stored);
    if (isNaN(installDate.getTime())) {
      installDate = new Date();
      writeFileSync(markerPath, installDate.toISOString(), { mode: 0o600 });
    }
  } else {
    installDate = new Date();
    mkdirSync(home, { recursive: true });
    writeFileSync(markerPath, installDate.toISOString(), { mode: 0o600 });
  }

  const now = new Date();
  const diffMs = now.getTime() - installDate.getTime();
  return Math.max(0, Math.floor(diffMs / (24 * 60 * 60 * 1000)));
}

// --- Payload Builder ---

/**
 * Build a GTIN telemetry payload from an ARP event.
 *
 * PRIVACY: This function intentionally includes ONLY:
 *   - sensorToken (anonymized device ID)
 *   - eventType (mapped category)
 *   - packageName / packageVersion (what was being monitored)
 *   - daySinceInstall (relative timing)
 *   - runtimeEnv (node/python/deno)
 *   - triggeredAt (ISO timestamp)
 *
 * NEVER included: credentials, payloads, user data, conversation content,
 * file paths, command arguments, IP addresses, hostnames.
 */
export function buildGTINPayload(
  event: ARPEvent,
  packageName: string,
  packageVersion?: string,
): GTINPayload {
  return {
    sensorToken: generateSensorToken(),
    eventType: mapEventType(event),
    packageName,
    packageVersion: packageVersion || '',
    daySinceInstall: getDaySinceInstall(),
    runtimeEnv: detectRuntimeEnv(),
    triggeredAt: event.timestamp,
  };
}

// --- Anomaly Filter ---

/** Categories that qualify as anomalous (eligible for GTIN transmission) */
const ANOMALOUS_CATEGORIES: Set<EventCategory> = new Set([
  'anomaly',
  'violation',
  'threat',
]);

/**
 * Determine if an event is anomalous and eligible for GTIN transmission.
 * Only anomalous events are transmitted -- normal events are NEVER sent.
 */
export function isAnomalousEvent(event: ARPEvent): boolean {
  return ANOMALOUS_CATEGORIES.has(event.category);
}

// --- Submission ---

/**
 * Submit a single GTIN event to the OpenA2A Registry.
 *
 * POST to https://api.oa2a.org/api/v1/telemetry/runtime
 * Timeout: 10 seconds. Non-blocking: failures are logged as warnings, never crash.
 */
export async function submitGTINEvent(
  payload: GTINPayload,
  registryUrl?: string,
): Promise<GTINSubmitResult> {
  const baseUrl = registryUrl || 'https://api.oa2a.org';
  const url = `${baseUrl}/api/v1/telemetry/runtime`;

  try {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 10_000);

    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'User-Agent': `OpenA2A-ARP/${VERSION}`,
      },
      body: JSON.stringify(payload),
      signal: controller.signal,
    });

    clearTimeout(timeout);

    if (!response.ok) {
      const body = await response.text().catch(() => '');
      return {
        success: false,
        error: `HTTP ${response.status}: ${body}`.substring(0, 200),
      };
    }

    const result = (await response.json()) as Record<string, unknown>;
    return {
      success: true,
      eventId: (result.eventId as string) || undefined,
    };
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : 'Unknown error';
    if (message.includes('abort') || message.includes('Abort')) {
      return { success: false, error: 'Request timed out (10s)' };
    }
    return { success: false, error: message };
  }
}
