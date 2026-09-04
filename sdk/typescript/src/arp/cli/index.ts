#!/usr/bin/env node

import * as os from 'os';
import * as path from 'path';
import { AgentRuntimeProtection, VERSION, loadConfig } from '../index';
import { ARPProxy } from '../proxy/server';
import { PromptInterceptor } from '../interceptors/prompt';
import { MCPProtocolInterceptor } from '../interceptors/mcp-protocol';
import { A2AProtocolInterceptor } from '../interceptors/a2a-protocol';
import { EventEngine } from '../engine/event-engine';
import { IntelligenceCoordinator, describeL2Status } from '../intelligence/coordinator';
import { SequenceLogWriter } from '../intelligence/sequence-log-writer';
import { isOptedOut, enrollSensor, manualEnrollCurl } from '../telemetry/signature';
import { runTelemetrySubcommand, telemetryHelpText } from './telemetry';
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
  // Ask the coordinator's own predicate rather than restating it here. A previous
  // version of this line restated it twice and was wrong both times.
  const l2 = describeL2Status(config.intelligence);
  console.log(
    `  Intelligence: ${
      l2.running
        ? `3-Layer (L0+L1+L2 via ${l2.adapter})`
        : `L0+L1 only (L2 not running: ${l2.reason})`
    }`,
  );
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

  // `register` is internal-only (see ./telemetry.ts); every other subcommand is
  // the shared shipped surface, dispatched through the same code path the
  // `aim-arp` bin uses so the two cannot drift.
  if (sub === 'register') {
    // Same contract as runTelemetrySubcommand: a help request is answered
    // with help, never by running the command — register builds a signed
    // enrollment proof and POSTs it, the worst possible answer to a question.
    const rest = args.slice(2);
    if (rest.some((a) => a === '--help' || a === '-h')) {
      printTelemetryHelpWithInternal();
      return;
    }
    const unknown = rest.find((a) => a.startsWith('-'));
    if (unknown !== undefined) {
      console.error(`  Unknown option for register: ${unknown}`);
      console.error('  Run: arp-guard telemetry --help');
      process.exit(1);
    }
    await telemetryRegister(tcfg);
    return;
  }
  if (sub === undefined || sub === '--help' || sub === '-h') {
    printTelemetryHelpWithInternal();
    return;
  }
  const code = await runTelemetrySubcommand(sub, args.slice(2), tcfg);
  if (code !== 0) process.exit(code);
}

function printTelemetryHelpWithInternal(): void {
  console.log(telemetryHelpText());
  console.log('  Internal-only (this CLI, not the shipped bin):');
  console.log('    register      Enroll this sensor with the registry (pending admin approval)\n');
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
    console.log('  even after approval. Re-enable with: aim-arp telemetry opt-in');
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
    console.log('  Check state anytime with: aim-arp telemetry status\n');
    return;
  }

  // Fail open: enrollment is best-effort; tell the user how to retry by hand.
  console.log(`  Could not reach the registry to enroll (${result.error ?? 'unknown error'}).`);
  console.log('  No verified status was granted; reports (if enabled) still send unverified.');
  console.log('  Retry later by re-running: telemetry register');
  console.log('  Or run it directly:');
  console.log(`    ${manualEnrollCurl(result)}\n`);
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
