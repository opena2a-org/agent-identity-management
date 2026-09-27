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
  sanitizeTerminalText,
  ecosystemOptOut,
  envTruthy,
  optOutMarkerExists,
  readAuditLog,
} from '../telemetry/signature';
import { homeReadError, opena2aHome } from '../telemetry/signature/paths';
import type { SignatureTelemetryConfig } from '../types';
import { SDK_VERSION } from '../../version';

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

/** Usage and description per subcommand: the group help and each subcommand's own help read this. */
const SUBCOMMAND_HELP: Record<TelemetrySubcommand, { usage: string; lines: string[] }> = {
  status: { usage: 'status', lines: ['Show telemetry state, sensor identity, and send counts'] },
  log: { usage: 'log [N]', lines: ['Show the last N audited payloads (default 20)'] },
  disclosure: { usage: 'disclosure', lines: ['Print the telemetry disclosure'] },
  'opt-out': {
    usage: 'opt-out',
    lines: [
      'Disable ALL OpenA2A telemetry (also asks the registry to',
      'delete already-sent signatures; use --no-purge to skip that)',
    ],
  },
  'opt-in': { usage: 'opt-in', lines: ['Remove the local opt-out marker (does not turn the channel on)'] },
  purge: { usage: 'purge', lines: ['Ask the registry to delete already-sent signatures (right-to-delete)'] },
};

/** Arguments each subcommand takes, for its own --help. */
const SUBCOMMAND_ARGS: Partial<Record<TelemetrySubcommand, string[]>> = {
  log: ['N             How many records to show: a positive integer (default 20)'],
  'opt-out': ['--no-purge    Keep already-sent signatures on the registry (delete later with purge)'],
};

export function telemetryHelpText(): string {
  const rows = TELEMETRY_SUBCOMMANDS.map((sub) => {
    const { usage, lines } = SUBCOMMAND_HELP[sub];
    return lines
      .map((line, i) => `    ${(i === 0 ? usage : '').padEnd(14)}${line}`)
      .join('\n');
  }).join('\n');
  return `
  ${SHIPPED_INVOCATION} <subcommand>

${rows}

  Structural signatures are OFF unless you turn them on with AIM_TELEMETRY=1
  or signatureTelemetry.enabled: true. The SHAPE of an anomalous behavior is
  shared (plus a sensor id and org pseudonym), never payloads. Every byte
  sent is recorded locally first — review it with: ${SHIPPED_INVOCATION} log
`;
}

/** Help for one subcommand: its usage, what it does, and the arguments it takes. */
export function telemetrySubcommandHelp(sub: TelemetrySubcommand): string {
  const { usage, lines } = SUBCOMMAND_HELP[sub];
  const args = SUBCOMMAND_ARGS[sub] ?? [];
  const out = ['', `  ${SHIPPED_INVOCATION} ${usage}${sub === 'opt-out' ? ' [--no-purge]' : ''}`, ''];
  out.push(...lines.map((l) => `    ${l}`));
  if (args.length > 0) {
    out.push('', ...args.map((a) => `    ${a}`));
  } else {
    out.push('', '    Takes no arguments.');
  }
  out.push('', `  All subcommands: ${SHIPPED_INVOCATION} --help`, '');
  return out.join('\n');
}

export async function telemetryLog(countArg?: string): Promise<number> {
  const n = parseInt(countArg ?? '', 10) || 20;
  const { records, skipped, error } = await readAuditLog(n);
  console.log(`\n  Telemetry audit log: ${auditLogPath()}`);
  if (error !== undefined) {
    // An unreadable log is not an empty one: saying "nothing sent" here would
    // be a claim about what left the machine that this command did not check.
    console.error(`  Cannot read the audit log (${error}); nothing can be said about what was sent.`);
    console.error(`  Check its permissions: ls -l ${auditLogPath()}\n`);
    return 1;
  }
  const skippedNote =
    skipped > 0
      ? `  Skipped ${skipped} line(s) that are not a complete record (a partial write or a missing field).`
      : null;
  if (records.length === 0) {
    console.log('  No telemetry has been sent yet (or the log is empty).');
    if (skippedNote) console.log(skippedNote);
    console.log('');
    return 0;
  }
  console.log(
    `  Last ${records.length} record(s), audited before any send (phase shows whether each was sent):\n`,
  );
  for (const r of records) {
    const phase = r.phase.toUpperCase().padEnd(9);
    console.log(`  ${r.ts}  ${phase}  ${r.techniqueId.padEnd(10)} ${r.severity}/${r.outcome}`);
    if (r.body) console.log(`      payload: ${r.body}`);
    if (r.detail) console.log(`      detail:  ${r.detail}`);
  }
  console.log("\n  A sent record's payload is byte-for-byte what left this machine. No");
  console.log('  prompts, tool args, paths, file contents, or secrets are present by');
  console.log('  design; the only identifiers are the sensor id and the rotating org');
  console.log('  pseudonym in the payload.');
  if (skippedNote) console.log(skippedNote);
  console.log('');
  return 0;
}

export async function telemetryStatus(tcfg?: SignatureTelemetryConfig): Promise<void> {
  const enabled = signatureTelemetryEnabled(tcfg);
  const optedOut = isOptedOut(tcfg);
  const { records, error: auditError } = await readAuditLog(100000);
  const counts: Record<string, number> = {};
  for (const r of records) counts[r.phase] = (counts[r.phase] ?? 0) + 1;
  const count = (phase: string): string =>
    auditError !== undefined ? 'unknown (audit log unreadable)' : String(counts[phase] ?? 0);

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
  // the first send mints an identity. Peeked values pass the terminal strip as
  // defence in depth -- an on-disk id predating enroll's shape validation, or
  // one written by another tool, must not drive the terminal.
  const sensorId = peekSensorId();
  const orgPseudonym = peekOrgPseudonym();
  console.log(
    `  Sensor id:    ${sensorId ? sanitizeTerminalText(sensorId) : 'none yet (created on first send)'}`,
  );
  console.log(
    `  Org pseudonym:${' '}${orgPseudonym ? `${sanitizeTerminalText(orgPseudonym)} (rotates monthly)` : 'none yet (created on first send)'}`,
  );
  const enrollment = readEnrollmentRecord();
  if (enrollment) {
    const label = enrollment.state === 'verified' ? 'verified' : 'pending admin approval';
    console.log(`  Enrollment:   ${label}`);
  } else {
    console.log('  Enrollment:   not enrolled');
  }
  console.log(
    `  Audit log:    ${auditLogPath()}${auditError !== undefined ? ` (cannot read: ${auditError})` : ''}`,
  );
  console.log('  Sent:         ' + count('sent'));
  console.log('  Buffered:     ' + count('buffered'));
  console.log('  Failed:       ' + count('failed'));
  console.log('  Dropped:      ' + count('dropped'));
  console.log(`\n  Review payloads: ${SHIPPED_INVOCATION} log`);
  console.log(`  Disclosure:      ${SHIPPED_INVOCATION} disclosure`);
  console.log(
    `  ${
      enabled
        ? `Opt out:         ${SHIPPED_INVOCATION} opt-out`
        : optedOut
          ? `Opt back in:     ${SHIPPED_INVOCATION} opt-in`
          : 'Turn on:         AIM_TELEMETRY=1'
    }`,
  );
  // The footer cites the bin name, which a local (non-global) install does not
  // put on PATH; name the form that works there once rather than on every line.
  console.log('  Not on your PATH? Run any of these as: npx @opena2a/aim-sdk telemetry <subcommand>\n');
}

export function telemetryDisclosure(tcfg?: SignatureTelemetryConfig): void {
  // Same layout as every other telemetry command: two-space indent and a
  // box-drawing rule under the title. disclosureText() itself is unchanged: it
  // is public API and is also what the first-run notice prints.
  const lines = disclosureText(tcfg).split('\n');
  const title = lines[0] ?? '';
  const styled = lines.map((l) => (/^-+$/.test(l) ? '─'.repeat(title.length) : l));
  console.log('\n' + styled.map((l) => (l.length > 0 ? `  ${l}` : '')).join('\n') + '\n');
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
    if (peekSensorId() === null) {
      console.log('  Nothing was ever sent from this machine, so there is nothing to delete.');
    } else {
      console.log('  Skipped deleting already-sent signatures (--no-purge).');
      console.log(`  Delete them later with: ${SHIPPED_INVOCATION} purge`);
    }
  } else {
    await runRemotePurge(tcfg, 'opt-out');
  }
  console.log(`  Re-enable with: ${SHIPPED_INVOCATION} opt-in\n`);
}

export async function telemetryPurge(tcfg?: SignatureTelemetryConfig): Promise<void> {
  // Standalone right-to-delete: request the registry delete already-sent
  // signatures without changing the opt-out state.
  await runRemotePurge(tcfg, 'purge');
  console.log('');
}

export function telemetryOptIn(tcfg?: SignatureTelemetryConfig): void {
  const hadMarker = optOutMarkerExists();
  clearOptOutMarker();
  const stillOff = isOptedOut(tcfg);
  console.log(hadMarker ? '\n  Opt-out marker removed.' : '\n  No opt-out marker was present.');
  if (stillOff) {
    console.log('  NOTE: telemetry is still disabled by an env var or config');
    console.log('  (OPENA2A_TELEMETRY=off, OPENA2A_TELEMETRY_OPTOUT, ARP_TELEMETRY_DISABLED,');
    console.log('  or signatureTelemetry.enabled: false). Clear those to re-enable.\n');
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
//
// A machine with no sensor identity has never sent anything, so there is
// nothing to purge — and building the purge proof would MINT the identity it
// is about to purge, which falsifies status's 'created on first send'. Peek
// first; skip the remote call when no identity exists.
async function runRemotePurge(
  tcfg: SignatureTelemetryConfig | undefined,
  context: 'opt-out' | 'purge',
): Promise<void> {
  if (peekSensorId() === null) {
    console.log('  No sensor identity exists; nothing has ever been sent from this');
    console.log('  machine, so there is nothing to purge.');
    return;
  }
  console.log('  Requesting deletion of already-sent signatures from the registry...');
  const result = await purgeRemoteSignatures(tcfg);
  if (result.ok) {
    const n = typeof result.deleted === 'number' ? result.deleted : 'unknown';
    console.log(`  Registry purge complete (deleted: ${n}).`);
    return;
  }
  // Fail open; the two callers' true states differ and must not share a claim.
  console.log(`  Could not reach the registry to delete sent signatures (${result.error ?? 'unknown error'}).`);
  if (context === 'opt-out') {
    console.log('  Your opt-out still took effect locally; no new data will be sent.');
  } else {
    console.log('  Nothing was deleted; your local telemetry state is unchanged.');
  }
  console.log(`  Retry the deletion later with: ${SHIPPED_INVOCATION} purge`);
  console.log('  Or run it directly:');
  console.log(`    ${manualPurgeCurl(result)}`);
}

function optOutReason(tcfg?: SignatureTelemetryConfig): string {
  if (tcfg?.enabled === false) return 'config (signatureTelemetry.enabled: false)';
  // The published ecosystem spelling gets its own attribution: the clear
  // instruction differs (unset the variable; opt-in cannot clear it).
  if (ecosystemOptOut()) return 'environment variable (OPENA2A_TELEMETRY; unset it to clear)';
  if (envTruthy('OPENA2A_TELEMETRY_OPTOUT')) return 'environment variable (OPENA2A_TELEMETRY_OPTOUT; unset it to clear)';
  if (envTruthy('ARP_TELEMETRY_DISABLED')) return 'environment variable (ARP_TELEMETRY_DISABLED; unset it to clear)';
  return `local opt-out marker (${SHIPPED_INVOCATION} opt-in to clear)`;
}

/**
 * Dispatch a telemetry subcommand. Returns the process exit code. Shared by the
 * shipped bin and the internal CLI so the two surfaces cannot diverge.
 */
/** Options each subcommand recognises; any other option-shaped arg is an error. */
const KNOWN_SUBCOMMAND_OPTIONS: Record<string, readonly string[]> = {
  'opt-out': ['--no-purge'],
};

const isHelpFlag = (a: string): boolean => a === '--help' || a === '-h';

export async function runTelemetrySubcommand(
  sub: string | undefined,
  rest: string[],
  tcfg?: SignatureTelemetryConfig,
): Promise<number> {
  // A help request in the subcommand slot (or no subcommand at all) is
  // answered with help regardless of what else is on the line.
  if (sub === undefined || isHelpFlag(sub)) {
    console.log(telemetryHelpText());
    return 0;
  }
  // A version request in the subcommand slot is a question about the bin, not
  // a typo'd subcommand; answer it like the top-level flag does.
  if (sub === '--version' || sub === '-v') {
    console.log(`aim-arp v${SDK_VERSION}`);
    return 0;
  }
  // A typo'd subcommand is reported as the error even when --help rides
  // along: the typo signal outranks the help shortcut, and nothing executes
  // either way.
  if (!(TELEMETRY_SUBCOMMANDS as readonly string[]).includes(sub)) {
    console.error(`  Unknown telemetry subcommand: ${sub}`);
    // `telemetry --no-purge opt-out`: an option ahead of its subcommand. Name
    // the order that works instead of only calling the option unknown.
    const intended = rest.find((a) => (TELEMETRY_SUBCOMMANDS as readonly string[]).includes(a));
    if (sub.startsWith('-') && intended !== undefined) {
      const others = rest.filter((a) => a !== intended);
      console.error(
        `  Options go after the subcommand: ${[SHIPPED_INVOCATION, intended, sub, ...others].join(' ')}`,
      );
    } else {
      console.error(`  Run: ${SHIPPED_INVOCATION} --help`);
    }
    return 1;
  }
  // A help request anywhere in the args is answered with help, never by
  // running the command: `opt-out --help` asks what opt-out does, and
  // executing it would change the consent state the question was about. The
  // answer is that subcommand's own usage, not the whole group's.
  if (rest.some(isHelpFlag)) {
    console.log(telemetrySubcommandHelp(sub as TelemetrySubcommand));
    return 0;
  }
  const known = KNOWN_SUBCOMMAND_OPTIONS[sub] ?? [];
  const unknown = rest.find((a) => a.startsWith('-') && !known.includes(a));
  if (unknown !== undefined) {
    console.error(`  Unknown option for ${sub}: ${unknown}`);
    console.error(`  Run: ${SHIPPED_INVOCATION} --help`);
    return 1;
  }
  // Positional arguments are held to the same standard as options. Before
  // this, `log abc` silently showed 20 records (parseInt || 20 swallowed the
  // garbage) and `log 5 5` silently ignored the extra argument, while `log -1`
  // was refused above — garbage left of the dash errored, garbage right of it
  // was swallowed. `log` takes at most one positional, a positive integer
  // count; no other subcommand takes any.
  const positionals = rest.filter((a) => !a.startsWith('-'));
  if (sub === 'log') {
    if (positionals.length > 1) {
      console.error(`  log takes at most one argument, got: ${positionals.join(' ')}`);
      console.error(`  Run: ${SHIPPED_INVOCATION} --help`);
      return 1;
    }
    if (positionals[0] !== undefined && !/^[1-9]\d*$/.test(positionals[0])) {
      console.error(`  log expects a positive integer count, got: ${positionals[0]}`);
      console.error(`  Run: ${SHIPPED_INVOCATION} --help`);
      return 1;
    }
  } else if (positionals.length > 0) {
    console.error(`  Unexpected argument for ${sub}: ${positionals[0]}`);
    console.error(`  Run: ${SHIPPED_INVOCATION} --help`);
    return 1;
  }
  // Every subcommand reads or writes the OpenA2A home. When it exists but
  // cannot be read, the consent state is unknown: say that instead of printing
  // defaults ("OFF (not turned on)", "none yet"), which would hide an opt-out
  // marker behind a permissions problem.
  const unreadable = homeReadError();
  if (unreadable !== null) {
    console.error(`  Cannot read the telemetry state directory ${opena2aHome()} (${unreadable}).`);
    console.error('  The telemetry state is unknown, not off; nothing was read or changed.');
    console.error(`  Check its permissions: ls -ld ${opena2aHome()}`);
    console.error('  (or point OPENA2A_HOME at a directory you can read and write)');
    if (sub === 'opt-out') {
      console.error('  To opt out without this directory: set OPENA2A_TELEMETRY=off');
    }
    return 1;
  }
  try {
    return await dispatchTelemetrySubcommand(sub as TelemetrySubcommand, rest, positionals, tcfg);
  } catch (err) {
    const fsCode = (err as NodeJS.ErrnoException)?.code;
    const fsPath = (err as NodeJS.ErrnoException)?.path;
    if (typeof fsCode === 'string' && FS_ERROR_CODES.has(fsCode) && typeof fsPath === 'string') {
      // A bare Node message ("EACCES: permission denied, open '...'") names
      // neither what failed nor what to do about it.
      console.error(`  ${sub} could not write ${fsPath} (${fsCode}); nothing further was changed.`);
      if (sub === 'opt-out') {
        console.error('  The opt-out did NOT take effect. To opt out without this directory: set OPENA2A_TELEMETRY=off');
      }
      console.error(`  Check its permissions: ls -ld ${opena2aHome()}`);
      console.error('  (or point OPENA2A_HOME at a directory you can read and write)');
      return 1;
    }
    throw err;
  }
}

/** Filesystem failures the telemetry commands answer with guidance rather than a raw error. */
const FS_ERROR_CODES = new Set(['EACCES', 'EPERM', 'EROFS', 'ENOSPC', 'ENOTDIR', 'EISDIR']);

async function dispatchTelemetrySubcommand(
  sub: TelemetrySubcommand,
  rest: string[],
  positionals: string[],
  tcfg?: SignatureTelemetryConfig,
): Promise<number> {
  switch (sub) {
    case 'log':
      return telemetryLog(positionals[0]);
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
    default:
      // Unreachable while the list and the arms agree: `sub` was validated
      // against TELEMETRY_SUBCOMMANDS above. A list entry with no arm lands
      // here instead of silently succeeding.
      console.error(`  Unknown telemetry subcommand: ${sub}`);
      console.error(`  Run: ${SHIPPED_INVOCATION} --help`);
      return 1;
  }
}
