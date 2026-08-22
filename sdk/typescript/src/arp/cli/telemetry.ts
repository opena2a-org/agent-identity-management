/**
 * The telemetry command surface shared by the shipped `aim-arp` bin and the
 * internal arp-guard CLI (src/arp/cli/index.ts). One implementation, two
 * registrations, so the shipped commands and the internal ones cannot drift.
 *
 * `register` (sensor enrollment) is deliberately NOT here and NOT in the shipped
 * bin: whether the verified-sensor program continues is an open question, and a
 * published npm surface pre-commits the answer. It stays internal-only until
 * that is decided (CPO ruling, 2026-08-22). No string in this module may cite
 * it — enforced by cited-commands.test.ts.
 */

import {
  readAuditRecords,
  auditLogPath,
  isOptedOut,
  signatureTelemetryEnabled,
  resolveRegistryUrl,
  writeOptOutMarker,
  clearOptOutMarker,
  disclosureText,
  peekSensorId,
  peekOrgPseudonym,
  purgeRemoteSignatures,
  manualPurgeCurl,
  readEnrollmentRecord,
} from '../telemetry/signature';
import type { SignatureTelemetryConfig } from '../types';

/** The install-facing invocation every user-visible citation uses. */
export const SHIPPED_INVOCATION = 'aim-arp telemetry';

/** The subcommands the shipped bin registers. The citation test pins against this. */
export const TELEMETRY_SUBCOMMANDS = [
  'status',
  'log',
  'disclosure',
  'opt-out',
  'opt-in',
  'purge',
] as const;

export type TelemetrySubcommand = (typeof TELEMETRY_SUBCOMMANDS)[number];

export function telemetryHelpText(): string {
  return `
  ${SHIPPED_INVOCATION} <subcommand>

    status        Show telemetry state, sensor identity, and send counts
    log [N]       Show the last N audited payloads sent (default 20)
    disclosure    Print the full install-time disclosure
    opt-out       Disable ALL OpenA2A telemetry (also deletes already-sent
                  signatures from the registry; use --no-purge to skip that)
    opt-in        Remove the local opt-out marker (does not turn the channel on)
    purge         Ask the registry to delete already-sent signatures (right-to-delete)

  Structural signatures are OFF unless you turn them on with AIM_TELEMETRY=1
  or signatureTelemetry.enabled: true. Only the SHAPE of an anomalous
  behavior is shared, never payloads. Every byte sent is recorded locally
  first — review it with: ${SHIPPED_INVOCATION} log
`;
}

export async function telemetryLog(countArg?: string): Promise<void> {
  const n = parseInt(countArg ?? '', 10) || 20;
  const records = await readAuditRecords(n);
  console.log(`\n  Telemetry audit log: ${auditLogPath()}`);
  if (records.length === 0) {
    console.log('  No telemetry has been sent yet (or the log is empty).\n');
    return;
  }
  console.log(`  Last ${records.length} record(s) — every byte that left this machine:\n`);
  for (const r of records) {
    const phase = r.phase.toUpperCase().padEnd(9);
    console.log(`  ${r.ts}  ${phase}  ${r.techniqueId.padEnd(10)} ${r.severity}/${r.outcome}`);
    if (r.body) console.log(`      payload: ${r.body}`);
    if (r.detail) console.log(`      detail:  ${r.detail}`);
  }
  console.log('\n  This is exactly what was transmitted. No payloads, prompts, args,');
  console.log('  paths, secrets, or identities are present by design.\n');
}

export async function telemetryStatus(tcfg?: SignatureTelemetryConfig): Promise<void> {
  const enabled = signatureTelemetryEnabled(tcfg);
  const optedOut = isOptedOut(tcfg);
  const records = await readAuditRecords(100000);
  const counts: Record<string, number> = {};
  for (const r of records) counts[r.phase] = (counts[r.phase] ?? 0) + 1;

  console.log('\n  OpenA2A structural signature telemetry');
  console.log('  ─────────────────────────────────────');
  // Off-because-refused and off-because-never-enabled are different states and
  // must not be collapsed: only one of them has a reason to report.
  console.log(
    `  State:        ${enabled ? 'ON' : optedOut ? 'OFF (opted out)' : 'OFF (not turned on)'}`,
  );
  if (optedOut) {
    console.log(`  Opted out by: ${optOutReason(tcfg)}`);
  } else if (!enabled) {
    console.log('  Turn on with: AIM_TELEMETRY=1 (or signatureTelemetry.enabled: true)');
  }
  console.log(`  Registry:     ${resolveRegistryUrl(tcfg)}`);
  // Read-only: status must never CREATE identity state. Peeks return null until
  // the first send mints an identity.
  const sensorId = peekSensorId();
  const orgPseudonym = peekOrgPseudonym();
  console.log(`  Sensor id:    ${sensorId ?? 'none yet (created on first send)'}`);
  console.log(
    `  Org pseudonym:${' '}${orgPseudonym ? `${orgPseudonym} (rotates monthly)` : 'none yet (created on first send)'}`,
  );
  const enrollment = readEnrollmentRecord();
  if (enrollment) {
    const label = enrollment.state === 'verified' ? 'verified' : 'pending admin approval';
    console.log(`  Enrollment:   ${label}`);
  } else {
    console.log('  Enrollment:   not enrolled');
  }
  console.log(`  Audit log:    ${auditLogPath()}`);
  console.log('  Sent:         ' + (counts['sent'] ?? 0));
  console.log('  Buffered:     ' + (counts['buffered'] ?? 0));
  console.log('  Failed:       ' + (counts['failed'] ?? 0));
  console.log('  Dropped:      ' + (counts['dropped'] ?? 0));
  console.log(`\n  Review payloads: ${SHIPPED_INVOCATION} log`);
  console.log(`  Disclosure:      ${SHIPPED_INVOCATION} disclosure`);
  console.log(
    `  ${
      enabled
        ? `Opt out:         ${SHIPPED_INVOCATION} opt-out`
        : optedOut
          ? `Opt back in:     ${SHIPPED_INVOCATION} opt-in`
          : 'Turn on:         AIM_TELEMETRY=1'
    }\n`,
  );
}

export function telemetryDisclosure(tcfg?: SignatureTelemetryConfig): void {
  console.log('\n' + disclosureText(tcfg) + '\n');
}

export async function telemetryOptOut(
  tcfg: SignatureTelemetryConfig | undefined,
  opts: { noPurge?: boolean } = {},
): Promise<void> {
  // Write the local marker FIRST — it is what actually stops transmission. The
  // remote purge below is best-effort cleanup of already-sent data and must
  // never block or undo the opt-out (fail OPEN).
  const p = writeOptOutMarker();
  console.log('\n  OpenA2A telemetry DISABLED (signature, GTIN and fleet-gradient channels).');
  console.log(`  Marker: ${p}`);

  if (opts.noPurge) {
    console.log('  Skipped deleting already-sent signatures (--no-purge).');
    console.log(`  Delete them later with: ${SHIPPED_INVOCATION} purge`);
  } else {
    await runRemotePurge(tcfg);
  }
  console.log(`  Re-enable with: ${SHIPPED_INVOCATION} opt-in\n`);
}

export async function telemetryPurge(tcfg?: SignatureTelemetryConfig): Promise<void> {
  // Standalone right-to-delete: request the registry delete already-sent
  // signatures without changing the opt-out state.
  await runRemotePurge(tcfg);
  console.log('');
}

export function telemetryOptIn(tcfg?: SignatureTelemetryConfig): void {
  clearOptOutMarker();
  const stillOff = isOptedOut(tcfg);
  console.log('\n  Opt-out marker removed.');
  if (stillOff) {
    console.log('  NOTE: telemetry is still disabled by an env var or config');
    console.log('  (OPENA2A_TELEMETRY_OPTOUT / ARP_TELEMETRY_DISABLED, or');
    console.log('  signatureTelemetry.enabled: false). Clear those to re-enable.\n');
  } else if (signatureTelemetryEnabled(tcfg)) {
    console.log('  Structural signature telemetry is ON.\n');
  } else {
    // Clearing a refusal is not consent. Say so rather than implying the
    // channel just started.
    console.log('  This channel stays OFF until you turn it on:');
    console.log('    AIM_TELEMETRY=1  (or signatureTelemetry.enabled: true)\n');
  }
}

// runRemotePurge requests the registry delete this sensor's already-sent
// signatures (G6 right-to-delete). Fails OPEN: any network/registry failure is
// reported with a manual retry command but never throws — the caller's opt-out
// (or standalone purge intent) completes regardless.
async function runRemotePurge(tcfg?: SignatureTelemetryConfig): Promise<void> {
  console.log('  Requesting deletion of already-sent signatures from the registry...');
  const result = await purgeRemoteSignatures(tcfg);
  if (result.ok) {
    const n = typeof result.deleted === 'number' ? result.deleted : 'unknown';
    console.log(`  Registry purge complete (deleted: ${n}).`);
    return;
  }
  // Fail open: the local opt-out still stands; tell the user how to retry the purge.
  console.log(`  Could not reach the registry to delete sent signatures (${result.error ?? 'unknown error'}).`);
  console.log('  Your opt-out still took effect locally; no new data will be sent.');
  console.log(`  Retry the deletion later with: ${SHIPPED_INVOCATION} purge`);
  console.log('  Or run it directly:');
  console.log(`    ${manualPurgeCurl(result)}`);
}

function optOutReason(tcfg?: SignatureTelemetryConfig): string {
  if (tcfg?.enabled === false) return 'config (signatureTelemetry.enabled: false)';
  if (process.env.OPENA2A_TELEMETRY_OPTOUT || process.env.ARP_TELEMETRY_DISABLED) return 'environment variable';
  return `local opt-out marker (${SHIPPED_INVOCATION} opt-in to clear)`;
}

/**
 * Dispatch a telemetry subcommand. Returns the process exit code. Shared by the
 * shipped bin and the internal CLI so the two surfaces cannot diverge.
 */
export async function runTelemetrySubcommand(
  sub: string | undefined,
  rest: string[],
  tcfg?: SignatureTelemetryConfig,
): Promise<number> {
  switch (sub) {
    case 'log':
      await telemetryLog(rest[0]);
      return 0;
    case 'status':
      await telemetryStatus(tcfg);
      return 0;
    case 'disclosure':
      telemetryDisclosure(tcfg);
      return 0;
    case 'opt-out':
      await telemetryOptOut(tcfg, { noPurge: rest.includes('--no-purge') });
      return 0;
    case 'purge':
      await telemetryPurge(tcfg);
      return 0;
    case 'opt-in':
      telemetryOptIn(tcfg);
      return 0;
    case undefined:
    case '--help':
    case '-h':
      console.log(telemetryHelpText());
      return 0;
    default:
      console.error(`  Unknown telemetry subcommand: ${sub}`);
      console.error(`  Run: ${SHIPPED_INVOCATION} --help`);
      return 1;
  }
}
