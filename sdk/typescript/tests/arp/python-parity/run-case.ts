/**
 * Runs one case of the ARP behavioral suite that the Python SDK shares
 * (sdk/python/tests/fixtures/ts_arp/behavioral-cases.json) through this SDK's
 * ARP code, and returns the output the case pins.
 *
 * The Python port (sdk/python/aim_sdk/arp) runs the same cases in
 * tests/test_arp_engine_parity.py and must produce the same output, so a
 * change to either SDK that the other does not follow fails one of the two
 * suites. The test that drives this file mocks `child_process.execSync`, which
 * the network monitor's lsof, ss and netstat parsers read from.
 */
import { execSync } from 'child_process';
import { mkdtempSync, rmSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { vi } from 'vitest';
import { EventEngine } from '../../../src/arp/engine/event-engine';
import { FilesystemInterceptor } from '../../../src/arp/interceptors/filesystem';
import { NetworkInterceptor } from '../../../src/arp/interceptors/network';
import { ProcessInterceptor } from '../../../src/arp/interceptors/process';
import { IntelligenceCoordinator } from '../../../src/arp/intelligence/coordinator';
import { RuntimeTwin } from '../../../src/arp/intelligence/runtime-twin';
import { FilesystemMonitor } from '../../../src/arp/monitors/filesystem';
import { NetworkMonitor } from '../../../src/arp/monitors/network';
import { ProcessMonitor } from '../../../src/arp/monitors/process';
import type { ARPConfig, ARPEvent, LLMAdapter } from '../../../src/arp/types';

/* eslint-disable @typescript-eslint/no-explicit-any */
type Input = Record<string, any>;
type Emitted = Omit<ARPEvent, 'id' | 'timestamp' | 'classifiedBy'>;

/** An engine stand-in that records what a monitor or interceptor emits. */
function recordingEngine(): { engine: EventEngine; emitted: Emitted[] } {
  const emitted: Emitted[] = [];
  const engine = {
    emit: async (event: Emitted) => {
      emitted.push(event);
      return event;
    },
  } as unknown as EventEngine;
  return { engine, emitted };
}

/** An event as JSON, without the fields that differ on every run. */
function shape(event: ARPEvent): Record<string, unknown> {
  const { id: _id, timestamp: _timestamp, ...rest } = JSON.parse(JSON.stringify(event));
  return rest;
}

function plain(value: unknown): unknown {
  return JSON.parse(JSON.stringify(value));
}

function fullEvent(input: Input): ARPEvent {
  return {
    id: 'parity-event',
    timestamp: new Date().toISOString(),
    classifiedBy: 'L0-rules',
    ...(plain(input) as Record<string, unknown>),
  } as ARPEvent;
}

async function runEngine(input: Input): Promise<unknown> {
  const config: ARPConfig = { agentName: 'parity-agent' };
  if (input.rules !== null) config.rules = input.rules;
  const engine = new EventEngine(config);
  const seen: ARPEvent[] = [];
  const enforcement: Array<Record<string, unknown>> = [];
  engine.onEvent((event) => {
    seen.push(event);
  });
  engine.onEnforcement((result) => {
    enforcement.push({
      action: result.action,
      success: result.success,
      outcome: result.outcome,
      reason: result.reason,
      eventDescription: result.event.description,
    });
  });
  for (const event of input.events) await engine.emit(plain(event) as Emitted);
  return { events: seen.map(shape), enforcement };
}

async function runCoordinator(input: Input): Promise<unknown> {
  const dataDir = mkdtempSync(join(tmpdir(), 'arp-python-parity-coordinator-'));
  const prompts: string[] = [];
  try {
    const l2 = input.l2 as Input | undefined;
    const coordinator = new IntelligenceCoordinator(
      {
        agentName: 'parity-agent',
        agentDescription: 'Books flights for staff',
        declaredCapabilities: ['flights:search', 'flights:book'],
        intelligence: l2 ? { enabled: true, adapter: 'ollama' } : {},
      },
      dataDir,
      (input.manifest as any) ?? null,
      input.risk ? { getBehavioralRiskSignal: async () => plain(input.risk) as any, close: async () => {} } : null,
      input.guardStatus ? { record: () => plain(input.guardStatus) as any } as any : null,
    );
    if (l2) {
      const adapter: LLMAdapter = {
        name: 'parity',
        assess: async (prompt: string) => {
          prompts.push(prompt);
          return { content: l2.content, inputTokens: l2.inputTokens, outputTokens: l2.outputTokens, model: 'parity' };
        },
        estimateCost: () => l2.cost,
        healthCheck: async () => true,
      };
      (coordinator as unknown as { adapter: LLMAdapter }).adapter = adapter;
    }
    const event = fullEvent(input.event);
    const assessment = await coordinator.analyze(event);
    await coordinator.stop();
    return { event: shape(event), assessment: plain(assessment), prompts };
  } finally {
    rmSync(dataDir, { recursive: true, force: true });
  }
}

async function runBatchParse(input: Input): Promise<unknown> {
  const dataDir = mkdtempSync(join(tmpdir(), 'arp-python-parity-batch-'));
  try {
    const coordinator = new IntelligenceCoordinator(
      { agentName: 'parity-agent', intelligence: { enabled: true, adapter: 'ollama', enableBatching: true } },
      dataDir,
    );
    const adapter: LLMAdapter = {
      name: 'parity',
      assess: async () => ({ content: input.content, inputTokens: input.tokens, outputTokens: 0, model: 'parity' }),
      estimateCost: (_inputTokens: number, outputTokens: number) => (outputTokens === 0 ? input.cost : 0.001),
      healthCheck: async () => true,
    };
    (coordinator as unknown as { adapter: LLMAdapter }).adapter = adapter;
    const events: ARPEvent[] = [];
    for (let i = 0; i < input.count; i++) {
      const event = fullEvent({ source: 'network', category: 'anomaly', severity: 'medium', description: `event ${i + 1}`, data: {} });
      events.push(event);
      await coordinator.analyze(event);
    }
    await coordinator.stop();
    return events.map((event) => plain(event.llmAssessment));
  } finally {
    rmSync(dataDir, { recursive: true, force: true });
  }
}

function runTwinScore(input: Input): unknown {
  const twin = new RuntimeTwin('parity-agent', { enabled: true });
  twin._setBaselineForTest({ ...input.baseline, capabilitySet: new Set(input.baseline.capabilitySet) });
  return plain(twin.scoreARPEvent(fullEvent(input.event)));
}

function networkMonitorParser(name: 'parseSs' | 'parseLsof' | 'parseNetstat', output: string): unknown {
  vi.mocked(execSync).mockReturnValue(output);
  const monitor = new NetworkMonitor(null as unknown as EventEngine);
  return plain((monitor as unknown as Record<string, () => unknown>)[name]());
}

/** The output of one case, as the shared fixture's `expected` holds it. */
export async function runCase(kind: string, input: Input): Promise<unknown> {
  switch (kind) {
    case 'filesystem-interceptor': {
      const { engine, emitted } = recordingEngine();
      const interceptor = new FilesystemInterceptor(engine, input.allowedPaths) as any;
      if (input.op === 'write') interceptor.handleWrite(input.path, input.operation);
      else if (input.op === 'read') interceptor.handleRead(input.path);
      else interceptor.handleDelete(input.path);
      return plain(emitted);
    }
    case 'process-spawn': {
      const { engine, emitted } = recordingEngine();
      (new ProcessInterceptor(engine) as any).handleSpawn(input.command, input.args);
      return plain(emitted);
    }
    case 'process-exec': {
      const { engine, emitted } = recordingEngine();
      (new ProcessInterceptor(engine) as any).handleExec(input.command);
      return plain(emitted);
    }
    case 'network-interceptor': {
      const { engine, emitted } = recordingEngine();
      (new NetworkInterceptor(engine, input.allowedHosts) as any).handleConnection(input.host, input.port);
      return plain(emitted);
    }
    case 'engine':
      return runEngine(input);
    case 'coordinator':
      return runCoordinator(input);
    case 'batch-parse':
      return runBatchParse(input);
    case 'twin-score':
      return runTwinScore(input);
    case 'monitor-hex-addr':
      return plain((new NetworkMonitor(null as unknown as EventEngine) as any).parseHexAddr(input.value));
    case 'monitor-split-addr-port':
      return plain((new NetworkMonitor(null as unknown as EventEngine) as any).splitAddrPort(input.value));
    case 'monitor-ss':
      return networkMonitorParser('parseSs', input.output);
    case 'monitor-lsof':
      return networkMonitorParser('parseLsof', input.output);
    case 'monitor-netstat':
      return networkMonitorParser('parseNetstat', input.output);
    case 'process-monitor': {
      const { engine, emitted } = recordingEngine();
      const monitor = new ProcessMonitor(engine) as any;
      monitor.agentPid = input.agentPid;
      monitor.knownPids = new Set(input.known);
      monitor.readProcessTable = () => plain(input.table);
      monitor.poll();
      return plain(emitted);
    }
    case 'network-monitor': {
      const { engine, emitted } = recordingEngine();
      const monitor = new NetworkMonitor(engine, 10000, input.allowedHosts) as any;
      monitor.knownDestinations = new Set(input.known);
      for (const connections of input.polls) {
        monitor.getConnections = () => plain(connections);
        monitor.poll();
      }
      return plain(emitted);
    }
    case 'filesystem-monitor': {
      const { engine, emitted } = recordingEngine();
      const monitor = new FilesystemMonitor(engine, [], input.allowedPaths) as any;
      monitor.handleEvent(input.eventType, input.filename, input.basePath);
      await monitor.stop();
      return plain(emitted);
    }
    default:
      throw new Error(`unknown case kind: ${kind}`);
  }
}
