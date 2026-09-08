import * as fs from 'fs';
import * as path from 'path';
import type { ARPConfig } from '../types';
import { nodeRequire } from '../node-require';

/**
 * Load ARP config from YAML or JSON file.
 * Falls back to sensible defaults if no config found.
 */
export function loadConfig(configPath?: string): ARPConfig {
  const config = loadConfigFromDisk(configPath);
  resolveGuardPublicKey(config);
  return config;
}

function loadConfigFromDisk(configPath?: string): ARPConfig {
  if (configPath) {
    return parseConfigFile(configPath);
  }

  // Auto-discover config
  const candidates = [
    'arp.yaml', 'arp.yml', 'arp.json',
    '.opena2a/arp.yaml', '.opena2a/arp.yml', '.opena2a/arp.json',
  ];

  for (const candidate of candidates) {
    const fullPath = path.resolve(process.cwd(), candidate);
    if (fs.existsSync(fullPath)) {
      return parseConfigFile(fullPath);
    }
  }

  return defaultConfig();
}

/**
 * Resolve the NanoMind-Guard public key for classification annotation, in
 * precedence order: an explicit config value wins, then the
 * `ARP_GUARD_PUBLIC_KEY` environment variable, otherwise `undefined`.
 *
 * Applied to the final merged config (regardless of source) because
 * `parseConfigFile` shallow-merges the `intelligence` block — a file that
 * declares `intelligence` would otherwise wipe the env fallback. There is no
 * fabricated default: an absent key leaves `guardPublicKey` undefined and the
 * proxy simply does not build the annotator (classification stays null).
 */
function resolveGuardPublicKey(config: ARPConfig): void {
  if (!config.intelligence) {
    config.intelligence = {};
  }
  config.intelligence.guardPublicKey =
    config.intelligence.guardPublicKey ??
    process.env.ARP_GUARD_PUBLIC_KEY ??
    undefined;
}

function parseConfigFile(filePath: string): ARPConfig {
  const content = fs.readFileSync(filePath, 'utf-8');
  const ext = path.extname(filePath).toLowerCase();

  if (ext === '.json') {
    return { ...defaultConfig(), ...JSON.parse(content) };
  }

  // YAML parsing (dynamic import to keep it optional)
  try {
    const yaml = nodeRequire('js-yaml');
    return { ...defaultConfig(), ...yaml.load(content) };
  } catch {
    throw new Error(`Failed to parse config: ${filePath}. Install js-yaml for YAML support.`);
  }
}

export function defaultConfig(): ARPConfig {
  return {
    agentName: path.basename(process.cwd()),
    agentDescription: undefined,
    declaredCapabilities: [],
    dataDir: path.join(process.cwd(), '.opena2a', 'arp'),
    monitors: {
      process: { enabled: true, intervalMs: 5000 },
      network: { enabled: true, intervalMs: 10000 },
      filesystem: { enabled: true },
      skill: { enabled: false },
      heartbeat: { enabled: false },
    },
    rules: [],
    // L2 (LLM-assisted analysis) is OFF unless the operator turns it on and says
    // WHERE inference runs. Both omissions below are load-bearing.
    //
    // `enabled` is ABSENT, not `false`, and the difference matters in two
    // directions. Every gate that can start L2 tests `enabled === true`, so absent
    // is off exactly as firmly as false is — the security property does not depend
    // on which one is written here. But `intelligence.enabled === false` is also a
    // documented kill switch for the local behavioral twin (see buildRuntimeTwin in
    // index.ts), so writing an explicit false here would have switched the twin off
    // for everyone by default, disabling L1's behavioral signal as a side effect of
    // an egress fix. Absent keeps "the operator switched intelligence off"
    // distinguishable from "the operator said nothing".
    //
    // `adapter` is ABSENT on purpose too. It used to default to 'agent-proxy',
    // which resolved a destination from whichever model key happened to be exported
    // (ANTHROPIC_API_KEY, else OPENAI_API_KEY, else a local Ollama). A security
    // tool must not pick who receives its observations from ambient environment
    // state. Choosing a destination is now an explicit act.
    intelligence: {
      budgetUsd: 5.0,
      maxTokensPerCall: 300,
      maxCallsPerHour: 20,
      minSeverityForLlm: 'medium',
      enableBatching: true,
      batchWindowMs: 300000,
    },
    // Structural signature telemetry is OFF unless the operator turns it on
    // (AIM_TELEMETRY=1, or signatureTelemetry.enabled: true). Deliberately
    // ABSENT here rather than `{ enabled: false }`: an explicit false is an
    // opt-OUT, and an opt-out beats an opt-in, so writing one here would make
    // AIM_TELEMETRY=1 silently do nothing. Absent means "nobody has chosen",
    // which is the only value that leaves both switches working.
  };
}
