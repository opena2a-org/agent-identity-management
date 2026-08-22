/**
 * First-run disclosure (G7).
 *
 * The channel is opt-in, so this text is shown to someone who has already turned
 * it on: it confirms plainly what that choice shares, what it never shares, and
 * how to reverse it. Intentionally plain and non-marketing (CPO/legal voice).
 * Shown once (a marker is written after the first show) and always available on
 * demand via `disclosureText()` or `aim-arp telemetry disclosure`.
 *
 * Every command this text cites must be registered by the shipped `aim-arp`
 * bin — enforced by cli/cited-commands.test.ts (#400).
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
    'What is shared: the STRUCTURAL SHAPE of an anomalous behavior the runtime',
    'observed — a technique class, a severity, an outcome, an event count, and a',
    'one-way hash of allowlisted structural tokens (action/target categories',
    'among them). Each report also carries a random per-install sensor id, a',
    'monthly-rotating org pseudonym, and the sensor key signature, so the',
    'registry can tell one sensor seeing an attack shape twice from the same',
    'shape appearing at many deployments.',
    '',
    'What a report NEVER contains: prompts, model responses, tool arguments, file',
    'contents or paths, command lines, environment values, secrets, IP addresses,',
    'hostnames, account ids, or the names of your tools, agents, or models.',
    '',
    'Why: shared structural signatures let an attack first seen at one deployment',
    'protect every other deployment. Nothing is sent until you have visibility:',
    'every payload is written to a local audit log BEFORE it is sent, so you can',
    'verify exactly what left your machine.',
    '',
    `Audit log:  ${auditLogPath()}`,
    'Review it:  read the audit log file above, or run `aim-arp telemetry log` /',
    '            `aim-arp telemetry status` (also: `npx @opena2a/aim-sdk telemetry log`).',
    '',
    'This channel is OFF unless you turn it on (either one):',
    '  - set AIM_TELEMETRY=1',
    '  - signatureTelemetry.enabled: true in your ARP config',
    '',
    'How to turn it off again (any one):',
    '  - set OPENA2A_TELEMETRY=off (the ecosystem switch opena2a.org/privacy',
    '    documents; OPENA2A_TELEMETRY_OPTOUT=1 and ARP_TELEMETRY_DISABLED=1 also work)',
    '  - signatureTelemetry.enabled: false in your ARP config',
    '  - run `aim-arp telemetry opt-out` (or `npx @opena2a/aim-sdk telemetry opt-out`)',
    'An opt-out always wins over an opt-in. It disables every channel this',
    'runtime-protection module produces: structural signatures, the legacy GTIN',
    'channel, and fleet behavioral gradients. The `aim-arp telemetry opt-out`',
    'command additionally asks the registry to delete signatures this sensor',
    'already sent (right-to-delete); the env and config opt-outs stop sending but',
    'do not delete — run `aim-arp telemetry purge` for that. Fleet gradients',
    'carry no identifier, so there is nothing about them to purge.',
    'The AIM client has a separate causal-denial relay with its own switch,',
    'off unless you enabled it; this opt-out does not govern it.',
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
