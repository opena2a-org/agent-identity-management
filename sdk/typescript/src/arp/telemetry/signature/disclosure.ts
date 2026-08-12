/**
 * First-run disclosure (G7).
 *
 * The channel is opt-in, so this text is shown to someone who has already turned
 * it on: it confirms plainly what that choice shares, what it never shares, and
 * how to reverse it. Intentionally plain and non-marketing (CPO/legal voice).
 * Shown once (a marker is written after the first show) and always available on
 * demand via `disclosureText()` (or `arp telemetry disclosure` for hackmyagent
 * CLI users).
 */

import { existsSync, mkdirSync, writeFileSync } from 'fs';
import { opena2aHome, homePath, DISCLOSURE_MARKER_FILE } from './paths';
import { auditLogPath } from './audit-log';
import { isOptedOut, signatureTelemetryEnabled, type SignatureTelemetryConfig } from './config';

/** Build the disclosure text. Reflects the CURRENT channel state for honesty. */
export function disclosureText(config?: SignatureTelemetryConfig): string {
  const optedOut = isOptedOut(config);
  const enabled = signatureTelemetryEnabled(config);
  const status = optedOut
    ? 'Status: OFF. You have opted out; nothing is shared.'
    : enabled
      ? 'Status: ON. You turned this on; structural signatures are shared (see below).'
      : 'Status: OFF. This channel is off unless you turn it on; nothing is shared.';

  return [
    'OpenA2A community threat intelligence',
    '----------------------------------------',
    status,
    '',
    'What is shared: only the STRUCTURAL SHAPE of an anomalous behavior the',
    'runtime observed — a technique class, an action/target category, an outcome,',
    'a severity, and a one-way hash of those tokens. Reports are signed so the',
    'registry can recognize the same attack shape seen across deployments.',
    '',
    'What is NEVER shared: prompts, model responses, tool arguments, file',
    'contents or paths, command lines, environment values, secrets, IP addresses,',
    'hostnames, account ids, or the names of your tools, agents, or models.',
    '',
    'Why: shared structural signatures let an attack first seen at one deployment',
    'protect every other deployment. Nothing is sent until you have visibility:',
    'every payload is written to a local audit log BEFORE it is sent, so you can',
    'verify exactly what left your machine.',
    '',
    `Audit log:  ${auditLogPath()}`,
    'Review it:  read the audit log file above. If you installed the hackmyagent',
    '            CLI you can also run `arp telemetry log` / `arp telemetry status`.',
    '',
    'This channel is OFF unless you turn it on (either one):',
    '  - set AIM_TELEMETRY=1',
    '  - signatureTelemetry.enabled: true in your ARP config',
    '',
    'How to turn it off again (any one):',
    '  - set OPENA2A_TELEMETRY_OPTOUT=1 (or ARP_TELEMETRY_DISABLED=1)',
    '  - signatureTelemetry.enabled: false in your ARP config',
    '  - run `arp telemetry opt-out` (hackmyagent CLI)',
    'An opt-out always wins over an opt-in. Opting out disables ALL OpenA2A',
    'telemetry and purges shared signatures on the registry (right-to-delete).',
  ].join('\n');
}

/** Whether the one-time disclosure marker has been written. */
export function hasShownDisclosure(): boolean {
  return existsSync(homePath(DISCLOSURE_MARKER_FILE));
}

/** Persist the one-time disclosure marker. */
export function markDisclosureShown(): void {
  const home = opena2aHome();
  if (!existsSync(home)) mkdirSync(home, { recursive: true });
  writeFileSync(homePath(DISCLOSURE_MARKER_FILE), new Date().toISOString() + '\n', { mode: 0o600 });
}

/**
 * Show the disclosure once (on first run). Returns true if it was shown this
 * call. Always safe to call; never throws.
 */
export function maybeShowDisclosure(
  print: (msg: string) => void = (m) => console.log(m),
  config?: SignatureTelemetryConfig,
): boolean {
  try {
    if (hasShownDisclosure()) return false;
    print('\n' + disclosureText(config) + '\n');
    markDisclosureShown();
    return true;
  } catch {
    return false;
  }
}
