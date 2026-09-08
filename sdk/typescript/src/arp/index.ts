/**
 * The version this ARP build reports -- the package version, deliberately.
 *
 * This used to be a separate literal tracking an "ARP engine lineage"
 * independent of `@opena2a/aim-sdk`. It read `0.2.0` from the day the module
 * landed and no release ever moved it, so five published package versions
 * (1.0.0 through 1.2.0) all reported the same engine version. The independence
 * was asserted by a comment, never exercised by a bump.
 *
 * That is not cosmetic, because this constant goes on the wire: both telemetry
 * channels send it as `User-Agent: OpenA2A-ARP/<VERSION>`, and it is the only
 * build signal the registry records for a submission. Every ARP request the
 * registry has ever logged carries `OpenA2A-ARP/0.2.0`, so no ingested
 * signature can be attributed to the sensor build that produced it.
 *
 * There is one npm package, so there is one version line. A second
 * hand-maintained version literal is the defect `SDK_VERSION` hit at 1.2.0
 * (package.json said 1.2.0, the export still said 1.1.0), and a comment asking
 * a human to remember is not a mechanism. `version.test.ts` beside this file is.
 *
 * Before changing this back: the `0.2.0` was not arbitrary. ARP shipped as its
 * own npm package, `arp-guard`, and that package is still published -- at
 * 0.3.0, last released 2026-03-23. So `0.2.0` was a fossil of a standalone
 * lineage that this copy left behind. It matched neither the package it ships
 * in (1.2.0) nor the standalone engine's current version (0.3.0), and the code
 * here has moved a long way past what `arp-guard` 0.3.0 contains. There is no
 * reading under which a frozen `0.2.0` was the right answer, and tracking
 * `arp-guard` instead would report a version describing different bytes than
 * the ones the caller installed.
 */
import { SDK_VERSION } from '../version';

export const VERSION = SDK_VERSION;

// Re-export types
export type {
  ARPConfig,
  ARPEvent,
  MonitorType,
  EventCategory,
  EventSeverity,
  LLMAdapter,
  LLMAdapterType,
  LLMAssessment,
  LLMResponse,
  IntelligenceConfig,
  BudgetState,
  AlertRule,
  AlertCondition,
  MonitorConfig,
  InterceptorConfig,
  AILayerConfig,
  ProxyConfig,
  ProxyUpstream,
  EnforcementAction,
  EnforcementResult,
  Monitor,
  GTINConfig,
} from './types';

// Re-export components
export { EventEngine } from './engine/event-engine';
export { CorrelationEngine } from './engine/correlation';
export { IntelligenceCoordinator } from './intelligence/coordinator';
export {
  SequenceProjector,
  projectSequences,
  projectSequencesFromFile,
  type LoggedActionEvent,
  type ProjectedAction,
  type InScopeActionSequence,
  type SequenceProjectorOptions,
} from './intelligence/sequence-projector';
export {
  ClassificationAnnotator,
  NanoMindGuardClassificationProvider,
  eventToClassifyText,
  classifyTextHash,
  toSignedResult,
  type ClassificationProvider,
  type ClassificationAnnotatorOptions,
} from './intelligence/classification-annotator';
export {
  SequenceLogWriter,
  arpEventToLoggedAction,
  type SequenceLogContext,
} from './intelligence/sequence-log-writer';
export { BudgetController } from './intelligence/budget';
export { AnomalyDetector } from './intelligence/anomaly';
export { RuntimeTwin } from './intelligence/runtime-twin';
export { AnthropicAdapter, OpenAIAdapter, OllamaAdapter, createAdapter, autoDetectAdapter } from './intelligence/adapters';
export { ProcessMonitor } from './monitors/process';
export { NetworkMonitor } from './monitors/network';
export { FilesystemMonitor } from './monitors/filesystem';
export { SkillCapabilityMonitor, createCapabilityMonitor, parseDeclaredCapabilities } from './monitors/skill-capability-monitor';
export type { DeclaredCapabilities, ObservedBehavior, CapabilityViolation } from './monitors/skill-capability-monitor';
export { ProcessInterceptor } from './interceptors/process';
export { NetworkInterceptor } from './interceptors/network';
export { FilesystemInterceptor } from './interceptors/filesystem';
export { PromptInterceptor } from './interceptors/prompt';
export { MCPProtocolInterceptor } from './interceptors/mcp-protocol';
export { A2AProtocolInterceptor } from './interceptors/a2a-protocol';
export { EnforcementEngine, type AlertCallback } from './enforcement/kill-switch';
export { LocalLogger } from './reporting/local-log';
export { loadConfig, defaultConfig } from './config/loader';
export { scanText, PATTERN_SETS, ALL_PATTERNS, type ThreatPattern, type ScanResult } from './patterns/ai-threats';
export { ARPProxy, type ARPProxyDeps } from './proxy/server';
export {
  checkLicense,
  hasFeature,
  registerLicenseValidator,
  PREMIUM_FEATURES,
  type LicenseTier,
  type LicenseInfo,
} from './license';

// Re-export telemetry
export {
  GTINForwarder,
  generateSensorToken,
  buildGTINPayload,
  submitGTINEvent,
  isAnomalousEvent,
  mapEventType,
  type GTINForwarderConfig,
  type GTINEventType,
  type GTINRuntimeEnv,
  type GTINPayload,
  type GTINSubmitResult,
} from './telemetry';

// Re-export the structural signature producer (G2/G4/G5/G7).
export {
  SignatureEmitter,
  deriveOutcome,
  redactEvent,
  behavioralHash,
  canonicalShape,
  currentOrgPseudonym,
  computeOrgPseudonym,
  buildSignedSubmission,
  buildCanonical,
  generateNonce,
  readAuditRecords,
  auditLogPath,
  appendAuditRecord,
  isOptedOut as isTelemetryOptedOut,
  signatureTelemetryEnabled,
  resolveRegistryUrl as resolveTelemetryRegistryUrl,
  writeOptOutMarker,
  clearOptOutMarker,
  disclosureText,
  maybeShowDisclosure,
  hasShownDisclosure,
  SCHEMA_VERSION as TELEMETRY_SCHEMA_VERSION,
  SIGNATURE_INGEST_PATH,
  ACTION_CLASSES,
  TARGET_CLASSES,
  TACTIC_IDS,
  OUTCOME_CLASSES,
  // Sensor lifecycle tooling used by the arp-guard CLI (opt-out purge,
  // enrollment, sensor identity) — exported so the CLI can run against this
  // module's public surface instead of deep paths.
  loadSensorId,
  purgeRemoteSignatures,
  manualPurgeCurl,
  enrollSensor,
  manualEnrollCurl,
  readEnrollmentRecord,
} from './telemetry/signature';
export type {
  RedactedSignal,
  ActionClass,
  TargetClass,
  TacticId,
  OutcomeClass,
  TelemetrySignatureRequest,
  AuditRecord,
  SignatureEmitterConfig,
} from './telemetry/signature';
export type { SignatureTelemetryConfig } from './types';

import * as path from 'path';
import type { ARPConfig, ARPEvent, Monitor } from './types';
import { EventEngine } from './engine/event-engine';
import { IntelligenceCoordinator } from './intelligence/coordinator';
import { RuntimeTwin } from './intelligence/runtime-twin';
import {
  InProcessBehavioralRiskSource,
  type BehavioralRiskSource,
} from './intelligence/behavioral-risk';
import {
  GuardAnomalyDetector,
  type GuardAnomalySource,
} from './intelligence/guard-anomaly';
import { EnforcementEngine, type AlertCallback } from './enforcement/kill-switch';
import { LocalLogger } from './reporting/local-log';
import { ProcessMonitor } from './monitors/process';
import { NetworkMonitor } from './monitors/network';
import { FilesystemMonitor } from './monitors/filesystem';
import { ProcessInterceptor } from './interceptors/process';
import { NetworkInterceptor } from './interceptors/network';
import { FilesystemInterceptor } from './interceptors/filesystem';
import { PromptInterceptor } from './interceptors/prompt';
import { MCPProtocolInterceptor } from './interceptors/mcp-protocol';
import { A2AProtocolInterceptor } from './interceptors/a2a-protocol';
import { loadConfig } from './config/loader';
import { GTINForwarder } from './telemetry/forwarder';
import { generateSensorToken } from './telemetry/gtin';
import { SignatureEmitter } from './telemetry/signature/emitter';
import {
  signatureTelemetryEnabled,
  isOptedOut,
  resolveRegistryUrl,
} from './telemetry/signature/config';
import { maybeShowDisclosure } from './telemetry/signature/disclosure';

/**
 * Agent Runtime Protection — the main entry point.
 *
 * Provides 3-layer intelligent monitoring for AI agents:
 * - L0: Rule-based event classification (free, every event)
 * - L1: Statistical anomaly detection (free, flagged events)
 * - L2: LLM-assisted assessment (micro-prompts, budget-controlled)
 *
 * Usage:
 *   const arp = new AgentRuntimeProtection({ agentName: 'my-agent' });
 *   await arp.start();
 *   // ... agent runs ...
 *   await arp.stop();
 */
export class AgentRuntimeProtection {
  private readonly config: ARPConfig;
  private readonly engine: EventEngine;
  private readonly intelligence: IntelligenceCoordinator;
  private readonly enforcement: EnforcementEngine;
  private readonly logger: LocalLogger;
  private readonly monitors: Monitor[] = [];
  private gtinForwarder: GTINForwarder | null = null;
  /**
   * Structural signature producer (G2/G4/G5/G7). OFF by default, opt-in — distinct
   * from the legacy opt-in GTIN runtime channel. Null only when the customer has
   * opted out (the single master opt-out, which also gates GTIN below).
   */
  private signatureEmitter: SignatureEmitter | null = null;
  private running = false;
  /**
   * In-process runtime twin (behavioral anomaly scorer). Held for the
   * lifetime of the ARP instance, attached to the event engine in
   * start() so every event trains the twin's baseline. Null when the
   * runtime twin is disabled in config.
   */
  private readonly runtimeTwin: RuntimeTwin | null;
  /**
   * Transport-agnostic view of the runtime twin passed to the
   * coordinator. Null when the twin is disabled.
   */
  private readonly behavioralRiskSource: BehavioralRiskSource | null;
  /**
   * Classification drift detector. Non-null only when the caller
   * provided a baseline in `config.intelligence.guardAnomaly.baseline`.
   */
  private readonly guardAnomaly: GuardAnomalySource | null;

  constructor(configOrPath?: ARPConfig | string) {
    if (typeof configOrPath === 'string') {
      this.config = loadConfig(configOrPath);
    } else {
      this.config = configOrPath ?? loadConfig();
    }

    const dataDir = this.config.dataDir ?? path.join(process.cwd(), '.opena2a', 'arp');

    this.engine = new EventEngine(this.config);

    // Build the runtime twin and behavioral risk source. Runtime twin is
    // on by default when intelligence is enabled; opt out via
    // `intelligence.runtimeTwin.enabled = false`. We do NOT attach the
    // twin to the event engine here; start() does that so a user who
    // constructs an ARP without calling start() does not get surprise
    // event handlers.
    this.runtimeTwin = buildRuntimeTwin(this.config);
    this.behavioralRiskSource = this.runtimeTwin
      ? new InProcessBehavioralRiskSource(this.runtimeTwin, 'runtime-twin-inproc')
      : null;

    // Build the guard anomaly detector. Only constructed when the caller
    // injected a baseline; otherwise drift detection is inert.
    this.guardAnomaly = buildGuardAnomaly(this.config);

    // Wire both sources into the coordinator. The third argument
    // (manifest) stays null here; capability manifests are loaded by
    // ARPProxy.start for HTTP proxy deployments and are not part of the
    // AgentRuntimeProtection constructor surface today.
    this.intelligence = new IntelligenceCoordinator(
      this.config,
      dataDir,
      null,
      this.behavioralRiskSource,
      this.guardAnomaly,
    );
    this.enforcement = new EnforcementEngine();
    this.logger = new LocalLogger(dataDir);

    // Wire up: events → intelligence → logger
    this.engine.onEvent(async (event) => {
      await this.intelligence.analyze(event);
      this.logger.logEvent(event);
    });

    // Wire up: enforcement → logger
    this.engine.onEnforcement(async (result) => {
      const enforced = await this.enforcement.execute(result.action, result.event);
      this.logger.logEnforcement(enforced);
    });

    // Create monitors based on config
    const mc = this.config.monitors;
    if (mc?.process?.enabled !== false) {
      this.monitors.push(new ProcessMonitor(this.engine, mc?.process?.intervalMs));
    }
    if (mc?.network?.enabled !== false) {
      this.monitors.push(new NetworkMonitor(this.engine, mc?.network?.intervalMs, mc?.network?.allowedHosts));
    }
    if (mc?.filesystem?.enabled !== false) {
      this.monitors.push(new FilesystemMonitor(this.engine, mc?.filesystem?.watchPaths, mc?.filesystem?.allowedPaths));
    }

    // Create interceptors (application-level hooks — zero latency, 100% accuracy)
    const ic = this.config.interceptors;
    if (ic?.process?.enabled) {
      this.monitors.push(new ProcessInterceptor(this.engine));
    }
    if (ic?.network?.enabled) {
      this.monitors.push(new NetworkInterceptor(this.engine, ic.network.allowedHosts));
    }
    if (ic?.filesystem?.enabled) {
      this.monitors.push(new FilesystemInterceptor(this.engine, ic.filesystem.allowedPaths, [dataDir]));
    }

    // Create AI-layer interceptors
    const al = this.config.aiLayer;
    if (al?.prompt?.enabled) {
      this.monitors.push(new PromptInterceptor(this.engine));
    }
    if (al?.mcp?.enabled) {
      this.monitors.push(new MCPProtocolInterceptor(this.engine, al.mcp.allowedTools));
    }
    if (al?.a2a?.enabled) {
      this.monitors.push(new A2AProtocolInterceptor(this.engine, al.a2a.trustedAgents));
    }

    // The master opt-out (env / marker file / config) disables the three
    // TELEMETRY channels this module produces — the two applied below plus fleet
    // gradients, which RuntimeTwin applies itself at construction and again
    // before each send. Resolved once here.
    //
    // It does NOT reach the L2 intelligence coordinator. That is not an
    // oversight left standing: L2 is governed by its own switch, which is now
    // off by default and additionally requires an explicitly chosen adapter, so
    // there is no default-on channel for this opt-out to have to catch. Said
    // plainly because the previous wording here claimed this opt-out disabled
    // "every channel THIS module produces", which was false while L2 defaulted
    // to on — and a false comment is how the next reader repeats the mistake.
    //
    // It does NOT govern src/telemetry/relay.ts, which is the AIM client's
    // causal-denial channel with its own switch, default off. That exclusion is
    // deliberate and documented in README.md; telemetry-egress-census.test.ts
    // pins the whole set so a new channel cannot join it silently.
    const optedOut = isOptedOut(this.config.signatureTelemetry);

    // Create GTIN forwarder if opted in AND not globally opted out. GTIN remains
    // a separate, opt-in/default-off legacy runtime channel.
    if (this.config.gtin?.enabled && !optedOut) {
      const sensorToken = this.config.gtin.sensorToken || generateSensorToken();
      this.gtinForwarder = new GTINForwarder({
        enabled: true,
        sensorToken,
        registryUrl: this.config.gtin.registryUrl,
        packageName: this.config.agentName,
      });

      // Subscribe forwarder to all events (it filters internally)
      this.engine.onEvent((event) => {
        this.gtinForwarder?.onEvent(event);
      });
    }

    // Create the structural signature emitter only if explicitly opted in
    // (OFF by default; see telemetry/signature/config.ts).
    if (signatureTelemetryEnabled(this.config.signatureTelemetry)) {
      this.signatureEmitter = new SignatureEmitter({
        registryUrl: resolveRegistryUrl(this.config.signatureTelemetry),
      });
      // Subscribe to all events; the emitter redacts fail-closed and filters
      // non-anomalous events internally.
      this.engine.onEvent((event) => {
        this.signatureEmitter?.onEvent(event);
      });
    }
  }

  /** Start all monitors */
  async start(): Promise<void> {
    if (this.running) return;

    // Attach the runtime twin to the event engine BEFORE monitors start
    // so the twin observes every event from the first tick. The twin
    // trains its own baseline over the first 100 events and does not
    // mutate the event or block the L0 decision.
    if (this.runtimeTwin) {
      this.runtimeTwin.attach(this.engine);
    }

    for (const monitor of this.monitors) {
      await monitor.start();
    }

    // Start GTIN forwarder if configured
    if (this.gtinForwarder) {
      this.gtinForwarder.start();
    }

    // Start the structural signature emitter and, on first run only, print the
    // plain first-run disclosure (G7) so a customer who turned this on knows exactly
    // what is shared and how to opt out.
    if (this.signatureEmitter) {
      maybeShowDisclosure(undefined, this.config.signatureTelemetry);
      this.signatureEmitter.start();
    }

    this.running = true;
  }

  /** Stop all monitors and flush logs */
  async stop(): Promise<void> {
    if (!this.running) return;

    for (const monitor of this.monitors) {
      await monitor.stop();
    }

    // Flush and shutdown GTIN forwarder
    if (this.gtinForwarder) {
      await this.gtinForwarder.shutdown();
    }

    // Flush and shutdown the signature emitter (sends any buffered signals).
    if (this.signatureEmitter) {
      await this.signatureEmitter.shutdown();
    }

    await this.intelligence.stop();
    this.running = false;
  }

  /** Check if ARP is running */
  isRunning(): boolean {
    return this.running;
  }

  /** Get current status */
  getStatus(): {
    running: boolean;
    monitors: Array<{ type: string; running: boolean }>;
    budget: ReturnType<IntelligenceCoordinator['getBudgetStatus']>;
    pausedPids: number[];
  } {
    return {
      running: this.running,
      monitors: this.monitors.map((m) => ({ type: m.type, running: m.isRunning() })),
      budget: this.intelligence.getBudgetStatus(),
      pausedPids: this.enforcement.getPausedPids(),
    };
  }

  /** Get recent events */
  getEvents(limit?: number): ARPEvent[] {
    return this.logger.readEvents(limit);
  }

  /** Resume a paused process */
  resume(pid: number): boolean {
    return this.enforcement.resume(pid);
  }

  /** Subscribe to all ARP events (for external integrations, test harnesses, etc.) */
  onEvent(handler: (event: ARPEvent) => void | Promise<void>): void {
    this.engine.onEvent(handler);
  }

  /** Subscribe to all enforcement results */
  onEnforcement(handler: (result: import('./types').EnforcementResult) => void | Promise<void>): void {
    this.engine.onEnforcement(handler);
  }

  /** Set the alert callback for the enforcement engine */
  setAlertCallback(callback: AlertCallback): void {
    this.enforcement.setAlertCallback(callback);
  }

  /** Get the event engine (for custom integrations) */
  getEngine(): EventEngine {
    return this.engine;
  }

  /** Get the enforcement engine (for test harnesses) */
  getEnforcement(): EnforcementEngine {
    return this.enforcement;
  }

  /**
   * Get the intelligence coordinator. Exposed so tests can assert the
   * coordinator was constructed with the expected behavioral risk and
   * guard anomaly sources, and so advanced integrations can swap
   * sources at runtime via `setBehavioralRiskSource` / `setGuardAnomaly`.
   */
  getIntelligence(): IntelligenceCoordinator {
    return this.intelligence;
  }

  /** The runtime twin instance, or null when disabled. */
  getRuntimeTwin(): RuntimeTwin | null {
    return this.runtimeTwin;
  }
}

/**
 * Construct an in-process `RuntimeTwin` from the ARP config, or return
 * null when the caller disabled intelligence or the runtime twin
 * explicitly. Kept as a free function so the constructor stays short
 * and the default policy is visible in one place.
 *
 * Default policy:
 *   - When `intelligence.enabled === false`: no twin (L2 disabled explicitly
 *     implies the behavioral layer is also unwanted).
 *   - When `intelligence.runtimeTwin.enabled === false`: no twin.
 *   - Otherwise: construct a twin seeded from `config.agentName`, with
 *     fleet federation opt-in from the config (default off).
 *
 * Note the asymmetry with the L2 gates, which require `enabled === true`. Here an
 * ABSENT `enabled` still builds the twin, and that difference is load-bearing:
 * `defaultConfig()` deliberately leaves `intelligence.enabled` absent rather than
 * setting it to false, so that "the operator explicitly switched intelligence off"
 * stays distinguishable from "the operator said nothing". L2 treats both as off.
 * The twin, which is local and feeds L1's risk signal, only turns off for the
 * explicit one — otherwise flipping the L2 default would have silently disabled
 * the behavioral signal for everyone as a side effect of an egress fix.
 */
function buildRuntimeTwin(config: ARPConfig): RuntimeTwin | null {
  const ic = config.intelligence;
  if (ic?.enabled === false) return null;
  const twinCfg = ic?.runtimeTwin;
  if (twinCfg?.enabled === false) return null;
  return new RuntimeTwin(config.agentName, {
    enabled: true,
    fleetEnabled: twinCfg?.fleetEnabled ?? false,
    agentCategory: twinCfg?.agentCategory ?? 'general',
  });
}

/**
 * Construct a `GuardAnomalyDetector` from the ARP config, or return
 * null when no baseline was provided or the caller explicitly disabled
 * guard anomaly detection. A baseline is required: drift detection
 * without a reference distribution is nonsense, so we refuse to
 * auto-bootstrap one from observations. The caller supplies a
 * Registry-exported training distribution in production, or a
 * snapshotted JSON file pre-Registry.
 */
function buildGuardAnomaly(config: ARPConfig): GuardAnomalySource | null {
  const gaCfg = config.intelligence?.guardAnomaly;
  if (!gaCfg) return null;
  if (gaCfg.enabled === false) return null;
  const baseline = gaCfg.baseline;
  if (!baseline || Object.keys(baseline).length === 0) return null;
  return new GuardAnomalyDetector({
    baseline,
    windowSize: gaCfg.windowSize,
    alarmThreshold: gaCfg.alarmThreshold,
    smoothing: gaCfg.smoothing,
    minObservations: gaCfg.minObservations,
    sourceName: gaCfg.sourceName,
  });
}
