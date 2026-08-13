/**
 * RuntimeTwin - Behavioral Anomaly Detection Layer (L1).
 *
 * Integrates the RuntimeTwin behavioral scorer into ARP's event pipeline.
 * Processes every L0 event through the twin for anomaly scoring.
 *
 * Three-tier ARP model:
 *   L0: Rule-based (EventEngine)           microseconds, always runs
 *   L1: RuntimeTwin behavioral twin        milliseconds, this module
 *   L2: Claude or local LLM intelligence   seconds, IntelligenceCoordinator
 *
 * L1 runs in parallel with L0. It never blocks the L0 decision.
 *
 * === SOURCE-OF-TRUTH NOTE ===
 *
 * The canonical implementation of this class lives at:
 *     opena2a-org/nanomind/packages/nanomind-runtime-core/src/index.ts
 *
 * This file is a temporary integration-layer mirror maintained in lockstep
 * with the canonical source until PR 1b publishes @nanomind/runtime-core
 * to npm. Until that cut-over, any change to the LSTM scoring logic,
 * gradient accumulation, or differential privacy path MUST be made in
 * both files. See todo/NANOMIND_V3_AUDIT.md Section 8 PR 1 and Section 9
 * item 1 for the rationale.
 *
 * This mirror retains the ARP-module-local imports of ARPEvent and
 * ARPConfig so ARP integration stays type-safe. The canonical package
 * defines a structural ARPEventInput equivalent to keep itself free of
 * cross-repo dependencies.
 */

import * as crypto from 'crypto';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
import { isOptedOut } from '../telemetry/signature/config';
import type { ARPEvent } from '../types';
import type { EventEngine } from '../engine/event-engine';

// === Types (mirroring @nanomind/runtime-core) ===

type EventType =
  | 'TOOL_CALL'
  | 'CAPABILITY_CHECK'
  | 'MCP_CALL'
  | 'MEMORY_READ'
  | 'MEMORY_WRITE'
  | 'EXTERNAL_CALL';

type ARPAction = 'allow' | 'alert' | 'throttle' | 'suspend' | 'kill';

interface BehavioralEvent {
  agentId: string;
  sessionId: string;
  sequenceNum: number;
  eventType: EventType;
  capability: string;
  toolName: string | null;
  argHash: string;
  timestampDelta: number;
  wallClock: number;
  responseSize: number;
  responseCode: number;
  l0Decision: 'allow' | 'block' | 'alert';
}

interface BaselineStats {
  eventTypeCounts: Record<string, number>;
  avgTimingDelta: number;
  stdTimingDelta: number;
  capabilitySet: Set<string>;
  avgResponseSize: number;
  totalEvents: number;
  errorRate: number;
}

interface AnomalyResult {
  score: number;
  action: ARPAction;
  reason: string;
}

// Response thresholds
const RESPONSE_TABLE: Array<{ threshold: number; action: ARPAction; label: string }> = [
  { threshold: 0.2, action: 'allow', label: 'Normal behavior' },
  { threshold: 0.4, action: 'alert', label: 'Unusual pattern detected' },
  { threshold: 0.6, action: 'throttle', label: 'Suspicious behavior — rate limited' },
  { threshold: 0.8, action: 'suspend', label: 'High anomaly — agent suspended' },
  { threshold: 1.0, action: 'kill', label: 'Critical anomaly — agent terminated' },
];

// === L1 Anomaly Detector ===

// Gradient dimensions (one per event type + timing + error + capability novelty)
const GRADIENT_DIMS = 9; // 6 event types + timing_z + error_rate + novelty_rate
const GRADIENT_FLUSH_INTERVAL = 300; // events between gradient submissions
const EVENT_TYPE_INDEX: Record<string, number> = {
  TOOL_CALL: 0, CAPABILITY_CHECK: 1, MCP_CALL: 2,
  MEMORY_READ: 3, MEMORY_WRITE: 4, EXTERNAL_CALL: 5,
};

export class RuntimeTwin {
  private agentId: string;
  private sessionId: string;
  private baseline: BaselineStats | null = null;
  private eventBuffer: BehavioralEvent[] = [];
  private sequenceNum = 0;
  private lastEventTime = Date.now();
  private eventLogPath: string;
  private enabled: boolean;
  private gradientAccumulator: number[] = new Array(GRADIENT_DIMS).fill(0);
  private gradientEventCount = 0;
  private gradientLossSum = 0;
  private fleetEnabled: boolean;
  private agentCategory: string;

  constructor(agentId: string, config?: { enabled?: boolean; fleetEnabled?: boolean; agentCategory?: string }) {
    this.agentId = agentId;
    this.sessionId = crypto.randomUUID();
    this.enabled = config?.enabled ?? true;
    // Opt-in only, and an opt-out beats the opt-in. Resolved once here so the
    // per-event accumulation path costs no syscall; flushGradient re-checks
    // before sending so an opt-out written after startup is still honored.
    this.fleetEnabled = (config?.fleetEnabled ?? false) && !isOptedOut();
    this.agentCategory = config?.agentCategory ?? 'general';
    this.eventLogPath = path.join(os.homedir(), '.opena2a', 'arp', 'events.jsonl');

    // Load baseline if exists
    this.loadBaseline();
  }

  /**
   * Attach to an ARP EventEngine — processes every event for anomaly detection.
   * Non-blocking: L1 runs in parallel, L0 decision is returned immediately.
   */
  attach(engine: EventEngine): void {
    if (!this.enabled) return;

    engine.onEvent((event: ARPEvent) => {
      // Convert ARP event to behavioral event
      const behavioral = this.convertEvent(event);
      if (!behavioral) return;

      // Process asynchronously — never block L0
      setImmediate(() => {
        const result = this.processEvent(behavioral);

        // If L1 detects high anomaly, escalate
        if (result.score > 0.4) {
          // Log escalation
          this.logEscalation(event, result);
        }
      });
    });
  }

  /**
   * Score an ARP event for behavioral risk WITHOUT mutating any twin state.
   *
   * Used by the behavioral risk IPC server (behavioral-risk-server.ts) to
   * answer on-demand scoring requests from a coordinator running in another
   * process, or by InProcessBehavioralRiskSource for single-process wiring.
   * Unlike `processEvent`, this does not advance the sequence number, append
   * to the event log, add to the event buffer, or accumulate the gradient.
   * It is a pure read against the current baseline.
   *
   * Returns null when the twin is disabled or the baseline is not yet
   * trained (totalEvents < 100). The caller must surface this to the IPC
   * client as a NOT_READY signal; silently returning a zero score would
   * misrepresent the twin's confidence.
   */
  scoreARPEvent(event: ARPEvent): AnomalyResult | null {
    if (!this.enabled) return null;
    if (!this.baseline || this.baseline.totalEvents < 100) return null;
    const behavioral = this.convertEventReadonly(event);
    if (!behavioral) return null;
    const score = this.computeAnomalyScore(behavioral);
    const response = this.getResponse(score);
    return { score, action: response.action, reason: response.label };
  }

  /**
   * Readonly variant of convertEvent that does not mutate lastEventTime or
   * sequenceNum. Used by scoreARPEvent for on-demand IPC scoring.
   */
  private convertEventReadonly(event: ARPEvent): BehavioralEvent | null {
    const now = Date.now();
    const delta = this.lastEventTime > 0 ? now - this.lastEventTime : 0;
    const sequenceNum = this.sequenceNum + 1;

    let eventType: EventType = 'TOOL_CALL';
    const source = String(event.data?.source || event.data?.monitor || '');
    if (source.includes('network')) eventType = 'EXTERNAL_CALL';
    else if (source.includes('filesystem')) eventType = 'MEMORY_READ';
    else if (source.includes('mcp')) eventType = 'MCP_CALL';
    else if (source.includes('capability')) eventType = 'CAPABILITY_CHECK';

    let l0Decision: 'allow' | 'block' | 'alert' = 'allow';
    if (event.data?._initialAction === 'kill' || event.data?._initialAction === 'pause') l0Decision = 'block';
    else if (event.category === 'threat' || event.category === 'violation') l0Decision = 'alert';
    else if (event.severity === 'high' || event.severity === 'critical') l0Decision = 'alert';

    const capability = String(event.data?.capability || event.data?.type || 'unknown');
    const toolName = event.data?.toolName ? String(event.data.toolName) : null;
    const responseSize = typeof event.data?.responseSize === 'number' ? event.data.responseSize : 0;

    return {
      agentId: this.agentId,
      sessionId: this.sessionId,
      sequenceNum,
      eventType,
      capability,
      toolName,
      argHash: crypto.createHash('sha256').update(JSON.stringify(event.data || {})).digest('hex').substring(0, 16),
      timestampDelta: delta,
      wallClock: now,
      responseSize,
      responseCode: event.data?.error ? 1 : 0,
      l0Decision,
    };
  }

  /**
   * Test-only seam: force-install a baseline so unit tests can exercise
   * scoreARPEvent without first calling processEvent a hundred times. Marked
   * with a leading underscore so it is easy to grep for in production code.
   */
  _setBaselineForTest(baseline: BaselineStats): void {
    this.baseline = baseline;
  }

  /**
   * Process a behavioral event and return anomaly score.
   */
  processEvent(event: BehavioralEvent): AnomalyResult {
    this.eventBuffer.push(event);
    this.appendToLog(event);

    if (this.eventBuffer.length > 1000) {
      this.eventBuffer = this.eventBuffer.slice(-500);
    }

    const score = this.computeAnomalyScore(event);
    const response = this.getResponse(score);

    // Accumulate gradient for fleet submission
    this.accumulateGradient(event, score);

    return {
      score,
      action: response.action,
      reason: response.label,
    };
  }

  /**
   * Convert an ARP event to a NanoMind behavioral event.
   */
  private convertEvent(event: ARPEvent): BehavioralEvent | null {
    const now = Date.now();
    const delta = now - this.lastEventTime;
    this.lastEventTime = now;
    this.sequenceNum++;

    // Map ARP event category to behavioral event type
    let eventType: EventType = 'TOOL_CALL';
    const source = String(event.data?.source || event.data?.monitor || '');
    if (source.includes('network')) eventType = 'EXTERNAL_CALL';
    else if (source.includes('filesystem')) eventType = 'MEMORY_READ';
    else if (source.includes('mcp')) eventType = 'MCP_CALL';
    else if (source.includes('capability')) eventType = 'CAPABILITY_CHECK';

    // Map L0 decision (CR-002: use _initialAction from fail-closed enforcement)
    let l0Decision: 'allow' | 'block' | 'alert' = 'allow';
    if (event.data?._initialAction === 'kill' || event.data?._initialAction === 'pause') l0Decision = 'block';
    else if (event.category === 'threat' || event.category === 'violation') l0Decision = 'alert';
    else if (event.severity === 'high' || event.severity === 'critical') l0Decision = 'alert';

    const capability = String(event.data?.capability || event.data?.type || 'unknown');
    const toolName = event.data?.toolName ? String(event.data.toolName) : null;
    const responseSize = typeof event.data?.responseSize === 'number' ? event.data.responseSize : 0;

    return {
      agentId: this.agentId,
      sessionId: this.sessionId,
      sequenceNum: this.sequenceNum,
      eventType,
      capability,
      toolName,
      argHash: crypto.createHash('sha256').update(JSON.stringify(event.data || {})).digest('hex').substring(0, 16),
      timestampDelta: delta,
      wallClock: now,
      responseSize,
      responseCode: event.data?.error ? 1 : 0,
      l0Decision,
    };
  }

  /**
   * Compute anomaly score (same algorithm as @nanomind/runtime-core).
   */
  private computeAnomalyScore(event: BehavioralEvent): number {
    if (!this.baseline || this.baseline.totalEvents < 100) {
      this.updateBaseline(event);
      return 0.0;
    }

    let score = 0.0;

    // Unknown capability
    if (!this.baseline.capabilitySet.has(event.capability)) score += 0.3;

    // Timing anomaly
    if (this.baseline.stdTimingDelta > 0) {
      const z = Math.abs(event.timestampDelta - this.baseline.avgTimingDelta) / this.baseline.stdTimingDelta;
      if (z > 3) score += 0.2;
      if (z > 5) score += 0.2;
    }

    // Rare event type
    const typeRatio = (this.baseline.eventTypeCounts[event.eventType] || 0) / this.baseline.totalEvents;
    if (typeRatio < 0.01) score += 0.15;

    // L0 flagged
    if (event.l0Decision === 'block') score += 0.3;
    if (event.l0Decision === 'alert') score += 0.15;

    // Burst
    const recent = this.eventBuffer.slice(-10);
    const avgDelta = recent.reduce((s, e) => s + e.timestampDelta, 0) / recent.length;
    if (avgDelta < 10 && recent.length >= 10) score += 0.2;

    // Error spike
    if (recent.filter(e => e.responseCode > 0).length > 5) score += 0.15;

    return Math.min(score, 1.0);
  }

  private updateBaseline(event: BehavioralEvent): void {
    if (!this.baseline) {
      this.baseline = {
        eventTypeCounts: {},
        avgTimingDelta: 0,
        stdTimingDelta: 0,
        capabilitySet: new Set(),
        avgResponseSize: 0,
        totalEvents: 0,
        errorRate: 0,
      };
    }
    const b = this.baseline;
    b.totalEvents++;
    b.eventTypeCounts[event.eventType] = (b.eventTypeCounts[event.eventType] || 0) + 1;
    b.capabilitySet.add(event.capability);

    const n = b.totalEvents;
    const oldAvg = b.avgTimingDelta;
    b.avgTimingDelta = oldAvg + (event.timestampDelta - oldAvg) / n;
    const diff = event.timestampDelta - b.avgTimingDelta;
    b.stdTimingDelta = Math.sqrt(
      (b.stdTimingDelta ** 2 * (n - 1) + (event.timestampDelta - oldAvg) * diff) / n
    );
  }

  private getResponse(score: number): { action: ARPAction; label: string } {
    for (const entry of RESPONSE_TABLE) {
      if (score <= entry.threshold) return entry;
    }
    return { action: 'kill', label: 'Critical anomaly' };
  }

  private loadBaseline(): void {
    const bp = path.join(os.homedir(), '.opena2a', 'arp', `baseline-${this.agentId}.json`);
    try {
      if (fs.existsSync(bp)) {
        const data = JSON.parse(fs.readFileSync(bp, 'utf-8'));
        data.capabilitySet = new Set(data.capabilitySet || []);
        this.baseline = data;
      }
    } catch {}
  }

  private appendToLog(event: BehavioralEvent): void {
    try {
      const dir = path.dirname(this.eventLogPath);
      if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true });
      fs.appendFileSync(this.eventLogPath, JSON.stringify(event) + '\n');
    } catch {}
  }

  private logEscalation(original: ARPEvent, result: AnomalyResult): void {
    try {
      const logDir = path.join(os.homedir(), '.opena2a', 'arp');
      if (!fs.existsSync(logDir)) fs.mkdirSync(logDir, { recursive: true });
      fs.appendFileSync(
        path.join(logDir, 'l1-escalations.jsonl'),
        JSON.stringify({
          timestamp: new Date().toISOString(),
          agentId: this.agentId,
          eventId: original.id,
          score: result.score,
          action: result.action,
          reason: result.reason,
          category: original.category,
        }) + '\n'
      );
    } catch {}
  }

  // === Fleet Gradient Submission ===

  /**
   * Accumulate behavioral features into gradient vector.
   * Each event contributes to a running gradient that captures the
   * distribution of event types, timing anomalies, error rates,
   * and capability novelty seen by this agent.
   */
  private accumulateGradient(event: BehavioralEvent, anomalyScore: number): void {
    if (!this.fleetEnabled) return;

    this.gradientEventCount++;
    this.gradientLossSum += anomalyScore;

    // Event type distribution (dims 0-5)
    const typeIdx = EVENT_TYPE_INDEX[event.eventType] ?? 0;
    this.gradientAccumulator[typeIdx] += 1.0;

    // Timing z-score (dim 6)
    if (this.baseline && this.baseline.stdTimingDelta > 0) {
      const z = (event.timestampDelta - this.baseline.avgTimingDelta) / this.baseline.stdTimingDelta;
      this.gradientAccumulator[6] += Math.min(Math.abs(z), 10.0);
    }

    // Error indicator (dim 7)
    if (event.responseCode > 0) {
      this.gradientAccumulator[7] += 1.0;
    }

    // Capability novelty (dim 8)
    if (this.baseline && !this.baseline.capabilitySet.has(event.capability)) {
      this.gradientAccumulator[8] += 1.0;
    }

    // Flush when we've accumulated enough events
    if (this.gradientEventCount >= GRADIENT_FLUSH_INTERVAL) {
      this.flushGradient();
    }
  }

  /**
   * Submit accumulated gradient to Registry fleet endpoint.
   * Normalizes by event count before submission (the fleet module
   * handles clipping and differential privacy noise).
   */
  private async flushGradient(): Promise<void> {
    if (this.gradientEventCount === 0) return;

    // Pre-send re-check of the master opt-out. This is the only point at which
    // fleet gradients leave the process, so it is the point that has to honor
    // "opting out disables all OpenA2A telemetry". Accumulators are reset on the
    // way out so a refusal does not turn this into a per-event check.
    if (isOptedOut()) {
      this.gradientAccumulator = new Array(GRADIENT_DIMS).fill(0);
      this.gradientEventCount = 0;
      this.gradientLossSum = 0;
      return;
    }

    // Normalize gradient by event count
    const normalized = this.gradientAccumulator.map(
      g => g / this.gradientEventCount,
    );
    const avgLoss = this.gradientLossSum / this.gradientEventCount;
    const count = this.gradientEventCount;

    // Reset accumulators
    this.gradientAccumulator = new Array(GRADIENT_DIMS).fill(0);
    this.gradientEventCount = 0;
    this.gradientLossSum = 0;

    // Submit gradient with differential privacy inline.
    // This reimplements the fleet.ts logic locally to avoid cross-package
    // import issues. The privacy guarantees are identical:
    // gradient clipping (L2 norm <= 1.0) + Gaussian noise (epsilon=1.0, delta=1e-5)
    try {
      const noisyGradient = this.addPrivacyNoise(normalized);
      await fetch('https://api.oa2a.org/api/v1/telemetry/behavioral-gradient', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          agentCategory: this.agentCategory,
          gradientVector: noisyGradient,
          localLoss: avgLoss,
          eventCount: count,
          privacyEpsilon: 1.0,
          modelVersion: '1.0.0',
          timestamp: new Date().toISOString(),
        }),
        signal: AbortSignal.timeout(5000),
      });
    } catch {
      // Non-fatal: gradient submission failure should never affect ARP
    }
  }

  /**
   * Apply differential privacy: clip to L2 norm 1.0, add Gaussian noise.
   * Matches the privacy guarantees inlined in @nanomind/runtime-core
   * (epsilon=1.0, delta=1e-5). The statistical twin's @nanomind/runtime
   * package was retired in the Q5 split (audit Section 9 item 1).
   */
  private addPrivacyNoise(gradient: number[]): number[] {
    // Clip to max L2 norm = 1.0
    const norm = Math.sqrt(gradient.reduce((s, g) => s + g * g, 0));
    const clipped = norm <= 1.0 ? [...gradient] : gradient.map(g => g / norm);

    // Gaussian noise: sigma = sensitivity * sqrt(2*ln(1.25/delta)) / epsilon
    const sigma = 1.0 * Math.sqrt(2 * Math.log(1.25 / 1e-5)) / 1.0;
    return clipped.map(g => {
      // Box-Muller transform
      const u1 = Math.random();
      const u2 = Math.random();
      const z = Math.sqrt(-2 * Math.log(u1)) * Math.cos(2 * Math.PI * u2);
      return g + sigma * z;
    });
  }

  /**
   * Force flush any remaining gradient (call on shutdown).
   */
  async shutdown(): Promise<void> {
    if (this.fleetEnabled && this.gradientEventCount > 0) {
      await this.flushGradient();
    }
  }
}
