#!/usr/bin/env node

import * as os from 'os';
import * as path from 'path';
import { AgentRuntimeProtection, VERSION, loadConfig } from '../index';
import { ARPProxy } from '../proxy/server';
import { PromptInterceptor } from '../interceptors/prompt';
import { MCPProtocolInterceptor } from '../interceptors/mcp-protocol';
import { A2AProtocolInterceptor } from '../interceptors/a2a-protocol';
import { EventEngine } from '../engine/event-engine';
import { IntelligenceCoordinator } from '../intelligence/coordinator';
import { SequenceLogWriter } from '../intelligence/sequence-log-writer';
import {
  readAuditRecords,
  auditLogPath,
  isOptedOut,
  signatureTelemetryEnabled,
  resolveRegistryUrl,
  writeOptOutMarker,
  clearOptOutMarker,
  disclosureText,
  loadSensorId,
  currentOrgPseudonym,
  purgeRemoteSignatures,
  manualPurgeCurl,
  enrollSensor,
  manualEnrollCurl,
  readEnrollmentRecord,
} from '../telemetry/signature';
import type { SignatureTelemetryConfig } from '../types';

const args = process.argv.slice(2);
const command = args[0];

async function main(): Promise<void> {
  switch (command) {
    case 'start':
      await startGuard();
      break;
    case 'stop':
      console.log('Stop not implemented in foreground mode. Use Ctrl+C.');
      break;
    case 'status':
      await showStatus();
      break;
    case 'tail':
      await tailEvents();
      break;
    case 'budget':
      await showBudget();
      break;
    case 'proxy':
      await startProxy();
      break;
    case 'telemetry':
      await telemetryCommand();
      break;
    case '--version':
    case '-v':
      console.log(`arp-guard v${VERSION}`);
      break;
    case '--help':
    case '-h':
    case undefined:
      showHelp();
      break;
    default:
      console.error(`Unknown command: ${command}`);
      showHelp();
      process.exit(1);
  }
}

async function startGuard(): Promise<void> {
  const configPath = args.find((a) => a.startsWith('--config='))?.split('=')[1]
    ?? (args.indexOf('--config') !== -1 ? args[args.indexOf('--config') + 1] : undefined);

  const config = loadConfig(configPath);

  console.log(`\n  ARP Guard v${VERSION}`);
  console.log(`  Agent: ${config.agentName}`);
  console.log(`  Intelligence: ${config.intelligence?.enabled !== false ? '3-Layer (L0+L1+L2)' : 'L0+L1 only'}`);
  console.log(`  Budget: $${config.intelligence?.budgetUsd ?? 5.00}/month`);
  console.log(`  Monitors: ${Object.entries(config.monitors ?? {}).filter(([, v]) => (v as { enabled: boolean }).enabled).map(([k]) => k).join(', ') || 'all'}`);
  console.log();

  const arp = new AgentRuntimeProtection(config);

  // Graceful shutdown
  const shutdown = async () => {
    console.log('\n  Stopping ARP Guard...');
    await arp.stop();
    const status = arp.getStatus();
    console.log(`  Budget used: $${status.budget.spent} / $${status.budget.budget} (${status.budget.percentUsed}%)`);
    console.log(`  Total L2 calls: ${status.budget.totalCalls}`);
    console.log('  Stopped.\n');
    process.exit(0);
  };

  process.on('SIGINT', shutdown);
  process.on('SIGTERM', shutdown);

  await arp.start();
  console.log('  Monitoring... (press Ctrl+C to stop)\n');

  // Keep alive
  const keepAlive = setInterval(() => {}, 60000);
  keepAlive.unref();
}

async function showStatus(): Promise<void> {
  const config = loadConfig();
  const arp = new AgentRuntimeProtection(config);
  const status = arp.getStatus();

  console.log(`\n  ARP Guard Status`);
  console.log(`  Running: ${status.running}`);
  console.log(`  Budget: $${status.budget.spent} / $${status.budget.budget} (${status.budget.percentUsed}% used)`);
  console.log(`  L2 calls this period: ${status.budget.totalCalls}`);
  console.log(`  L2 calls this hour: ${status.budget.callsThisHour} / ${status.budget.maxCallsPerHour}`);
  console.log(`  Paused PIDs: ${status.pausedPids.length > 0 ? status.pausedPids.join(', ') : 'none'}`);
  console.log();
}

async function tailEvents(): Promise<void> {
  const config = loadConfig();
  const arp = new AgentRuntimeProtection(config);
  const events = arp.getEvents(parseInt(args[1]) || 20);

  if (events.length === 0) {
    console.log('\n  No events recorded yet.\n');
    return;
  }

  console.log(`\n  Last ${events.length} events:\n`);
  for (const event of events) {
    const severity = event.severity.toUpperCase().padEnd(8);
    const source = event.source.padEnd(10);
    const llm = event.classifiedBy === 'L2-llm' ? ' [LLM]' : '';
    console.log(`  ${event.timestamp}  ${severity}  ${source}  ${event.description.slice(0, 80)}${llm}`);
  }
  console.log();
}

async function showBudget(): Promise<void> {
  const config = loadConfig();
  const arp = new AgentRuntimeProtection(config);
  const status = arp.getStatus();
  const b = status.budget;

  console.log(`\n  ARP Intelligence Budget`);
  console.log(`  ─────────────────────────────`);
  console.log(`  Spent:     $${b.spent.toFixed(4)}`);
  console.log(`  Budget:    $${b.budget.toFixed(2)}`);
  console.log(`  Remaining: $${b.remaining.toFixed(4)}`);
  console.log(`  Used:      ${b.percentUsed}%`);
  console.log(`  ─────────────────────────────`);
  console.log(`  Total L2 calls: ${b.totalCalls}`);
  console.log(`  This hour:      ${b.callsThisHour} / ${b.maxCallsPerHour}`);
  console.log();
}

async function startProxy(): Promise<void> {
  const configPath = args.find((a) => a.startsWith('--config='))?.split('=')[1]
    ?? (args.indexOf('--config') !== -1 ? args[args.indexOf('--config') + 1] : undefined);

  const config = loadConfig(configPath);

  if (!config.proxy) {
    console.error('  Error: No proxy configuration found. Add a "proxy" section to your config.');
    console.error('');
    console.error('  Example:');
    console.error('    proxy:');
    console.error('      listen: 127.0.0.1:8080');
    console.error('      target: http://localhost:3000');
    console.error('      tls: false');
    process.exit(1);
  }

  const engine = new EventEngine(config);
  const promptInterceptor = new PromptInterceptor(engine);
  const mcpInterceptor = new MCPProtocolInterceptor(engine, config.aiLayer?.mcp?.allowedTools);
  const a2aInterceptor = new A2AProtocolInterceptor(engine, config.aiLayer?.a2a?.trustedAgents);

  await promptInterceptor.start();
  await mcpInterceptor.start();
  await a2aInterceptor.start();

  // Detection-mode wiring: a coordinator that classifies and
  // runs the comply gate (which stays inert unless the signed manifest opts in
  // with comply.enforce=true), plus a sequence-log tee that records the ordered
  // in-scope action corpus for the offline consistency signal. Neither touches
  // the deny path.
  const dataDir = config.dataDir ?? path.join(os.homedir(), '.opena2a', 'arp');
  // Constructed with no behavioral-risk and no guard-anomaly source (the 4th
  // and 5th args default to null). That is deliberate: the only consumer of
  // event.data.classification on this coordinator is the comply gate, which is
  // itself gated by comply.enforce (default off). So a classification reaching
  // this detection coordinator can never raise severity or deny by default.
  const coordinator = new IntelligenceCoordinator(config, dataDir, null);
  const sequenceLog = new SequenceLogWriter(
    path.join(dataDir, 'sequence-events.jsonl'),
    config.agentName ?? 'arp-proxy',
  );

  // Resolve the Guard public key for classification annotation. Gated on the
  // intelligence layer being enabled (default on); the loader already merged
  // config + ARP_GUARD_PUBLIC_KEY. When present AND a manifest is configured,
  // the proxy builds the annotator at start() so events carry a cleared
  // classification for DETECTION (the sequence corpus). Absent key → no
  // annotator, classification stays null.
  const guardPublicKey =
    config.intelligence?.enabled === false
      ? undefined
      : config.intelligence?.guardPublicKey;

  const proxy = new ARPProxy(config.proxy, {
    engine,
    promptInterceptor,
    mcpInterceptor,
    a2aInterceptor,
    coordinator,
    guardPublicKey,
    onInScopeEvent: (event) => sequenceLog.append(event),
  });

  // Log detections to console
  let detectionCount = 0;
  engine.onEvent((event) => {
    if (event.category === 'threat' || event.category === 'violation') {
      detectionCount++;
      const sev = event.severity.toUpperCase().padEnd(8);
      console.log(`  [${sev}] ${event.description}`);
    }
  });

  const shutdown = async () => {
    console.log('\n  Stopping ARP Proxy...');
    await proxy.stop();
    await promptInterceptor.stop();
    await mcpInterceptor.stop();
    await a2aInterceptor.stop();
    console.log(`  Total detections: ${detectionCount}`);
    console.log('  Stopped.\n');
    process.exit(0);
  };

  process.on('SIGINT', shutdown);
  process.on('SIGTERM', shutdown);

  await proxy.start();

  console.log(`\n  ARP Proxy v${VERSION}`);
  console.log(`  Listening on port ${config.proxy.port}`);
  console.log(`  Block mode: ${config.proxy.blockOnDetection ? 'ENABLED' : 'alert-only'}`);
  console.log(`  Upstreams:`);
  for (const u of config.proxy.upstreams) {
    console.log(`    ${u.pathPrefix} -> ${u.target} (${u.protocol})`);
  }
  console.log(`\n  Scanning: prompt injection, jailbreak, data leak, MCP exploitation, A2A spoofing`);
  console.log('  Press Ctrl+C to stop\n');

  const keepAlive = setInterval(() => {}, 60000);
  keepAlive.unref();
}

async function telemetryCommand(): Promise<void> {
  const sub = args[1];
  const config = loadConfig();
  const tcfg = config.signatureTelemetry;

  switch (sub) {
    case 'log':
      await telemetryLog();
      break;
    case 'status':
      await telemetryStatus(tcfg);
      break;
    case 'register':
      await telemetryRegister(tcfg);
      break;
    case 'disclosure':
      console.log('\n' + disclosureText(tcfg) + '\n');
      break;
    case 'opt-out': {
      // Write the local marker FIRST — it is what actually stops transmission. The
      // remote purge below is best-effort cleanup of already-sent data and must
      // never block or undo the opt-out (fail OPEN).
      const p = writeOptOutMarker();
      console.log('\n  OpenA2A telemetry DISABLED (both signature and GTIN channels).');
      console.log(`  Marker: ${p}`);

      if (args.includes('--no-purge')) {
        console.log('  Skipped deleting already-sent signatures (--no-purge).');
        console.log('  Delete them later with: arp telemetry purge');
      } else {
        await runRemotePurge(tcfg);
      }
      console.log('  Re-enable with: arp telemetry opt-in\n');
      break;
    }
    case 'purge': {
      // Standalone right-to-delete: request the registry delete already-sent
      // signatures without changing the opt-out state.
      await runRemotePurge(tcfg);
      console.log('');
      break;
    }
    case 'opt-in': {
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
      break;
    }
    case undefined:
    case '--help':
    case '-h':
      console.log(`
  arp telemetry <subcommand>

    status        Show telemetry state, sensor identity, and send counts
    register      Enroll this sensor with the registry (pending admin approval)
    log [N]       Show the last N audited payloads sent (default 20)
    disclosure    Print the full install-time disclosure
    opt-out       Disable ALL OpenA2A telemetry (also deletes already-sent
                  signatures from the registry; use --no-purge to skip that)
    opt-in        Remove the local opt-out marker (does not turn the channel on)
    purge         Ask the registry to delete already-sent signatures (right-to-delete)

  Structural signatures are OFF unless you turn them on with AIM_TELEMETRY=1
  or signatureTelemetry.enabled: true. Only the SHAPE of an anomalous
  behavior is shared, never payloads. Every byte sent is recorded locally
  first — review it with: arp telemetry log
`);
      break;
    default:
      console.error(`  Unknown telemetry subcommand: ${sub}`);
      console.error('  Run: arp telemetry --help');
      process.exit(1);
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
  console.log('  Retry the deletion later with: arp telemetry purge');
  console.log('  Or run it directly:');
  console.log(`    ${manualPurgeCurl(result)}`);
}

// telemetryRegister enrolls this sensor with the registry (the producer half of the
// verified-sensor flow). Enrollment creates a PENDING enrollment that an admin must
// approve before reports count as verified; until then reports still send at the
// unverified weight. Fails OPEN: a network/registry failure is reported with a manual
// retry command but never throws.
async function telemetryRegister(tcfg?: SignatureTelemetryConfig): Promise<void> {
  console.log('\n  Enrolling this sensor with the OpenA2A registry...');
  if (isOptedOut(tcfg)) {
    console.log('  NOTE: telemetry is currently opted out, so no reports will be sent');
    console.log('  even after approval. Re-enable with: arp telemetry opt-in');
  }

  const result = await enrollSensor(tcfg);
  if (result.ok) {
    const state = result.state ?? 'pending';
    console.log(`  Enrollment ${state === 'verified' ? 'VERIFIED' : 'PENDING'}.`);
    if (result.sensorId) console.log(`  Sensor id: ${result.sensorId}`);
    if (state === 'verified') {
      console.log('  This sensor is approved; its reports are weighted as verified.');
    } else {
      console.log('  Next step: an OpenA2A admin must approve this enrollment.');
      console.log('  Until then, reports still send at the unverified weight.');
    }
    console.log('  Check state anytime with: arp telemetry status\n');
    return;
  }

  // Fail open: enrollment is best-effort; tell the user how to retry by hand.
  console.log(`  Could not reach the registry to enroll (${result.error ?? 'unknown error'}).`);
  console.log('  No verified status was granted; reports (if enabled) still send unverified.');
  console.log('  Retry later with: arp telemetry register');
  console.log('  Or run it directly:');
  console.log(`    ${manualEnrollCurl(result)}\n`);
}

async function telemetryLog(): Promise<void> {
  const n = parseInt(args[2], 10) || 20;
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

async function telemetryStatus(tcfg?: import('../types').SignatureTelemetryConfig): Promise<void> {
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
  try {
    console.log(`  Sensor id:    ${loadSensorId()}`);
    console.log(`  Org pseudonym:${' '}${currentOrgPseudonym()} (rotates monthly)`);
  } catch {
    // identity files may not exist until first send; do not fail status
  }
  const enrollment = readEnrollmentRecord();
  if (enrollment) {
    const label = enrollment.state === 'verified' ? 'verified' : 'pending admin approval';
    console.log(`  Enrollment:   ${label}`);
  } else {
    console.log('  Enrollment:   not enrolled (run: arp telemetry register)');
  }
  console.log(`  Audit log:    ${auditLogPath()}`);
  console.log('  Sent:         ' + (counts['sent'] ?? 0));
  console.log('  Buffered:     ' + (counts['buffered'] ?? 0));
  console.log('  Failed:       ' + (counts['failed'] ?? 0));
  console.log('  Dropped:      ' + (counts['dropped'] ?? 0));
  console.log('\n  Review payloads: arp telemetry log');
  console.log('  Disclosure:      arp telemetry disclosure');
  console.log(
    `  ${enabled ? 'Opt out:         arp telemetry opt-out' : optedOut ? 'Opt back in:     arp telemetry opt-in' : 'Turn on:         AIM_TELEMETRY=1'}\n`,
  );
}

function optOutReason(tcfg?: import('../types').SignatureTelemetryConfig): string {
  if (tcfg?.enabled === false) return 'config (signatureTelemetry.enabled: false)';
  if (process.env.OPENA2A_TELEMETRY_OPTOUT || process.env.ARP_TELEMETRY_DISABLED) return 'environment variable';
  return 'local opt-out marker (arp telemetry opt-in to clear)';
}

function showHelp(): void {
  console.log(`
  ARP Guard v${VERSION} — Agent Runtime Protection

  USAGE
    arp-guard <command> [options]

  COMMANDS
    start [--config <path>]   Start monitoring the agent
    proxy [--config <path>]   Start HTTP reverse proxy with AI-layer scanning
    stop                      Stop monitoring
    status                    Show current protection status
    tail [N]                  Show last N events (default: 20)
    budget                    Show LLM intelligence budget usage
    telemetry <sub>           Community signature telemetry (status|log|opt-out)

  INTELLIGENCE
    ARP uses a 3-layer intelligence stack:
      L0: Rules (free)        Pattern matching on every event
      L1: Statistical (free)  Z-score anomaly detection
      L2: LLM-Assisted ($)    Micro-prompts to the agent's LLM

    99% of events never reach L2. Default budget: $5/month.
    Auto-detects Anthropic, OpenAI, or Ollama from environment.

  AI-LAYER SCANNING (proxy mode)
    Prompt injection, jailbreak, data exfiltration detection
    MCP parameter exploitation (path traversal, command injection, SSRF)
    A2A identity spoofing and delegation abuse detection
    Output scanning for leaked secrets, PII, and system prompts

  EXAMPLES
    arp-guard start                     Start with auto-detected config
    arp-guard start --config arp.yaml   Start with custom config
    arp-guard proxy --config arp.yaml   Start proxy with AI-layer scanning
    arp-guard status                    Check budget and monitor status
    arp-guard tail 50                   Show last 50 events
    arp-guard budget                    Show intelligence spending
`);
}

main().catch((err) => {
  console.error(`Error: ${err instanceof Error ? err.message : String(err)}`);
  process.exit(1);
});
