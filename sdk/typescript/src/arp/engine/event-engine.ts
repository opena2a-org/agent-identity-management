import * as crypto from 'crypto';
import type {
  ARPEvent,
  ARPConfig,
  AlertRule,
  EventSeverity,
  EventCategory,
  EnforcementAction,
  EnforcementExecutor,
  EnforcementResult,
} from '../types';
import { CorrelationEngine } from './correlation';

type EventHandler = (event: ARPEvent) => void | Promise<void>;
type EnforcementHandler = (result: EnforcementResult) => void | Promise<void>;

const SEVERITY_ORDER: EventSeverity[] = ['info', 'low', 'medium', 'high', 'critical'];

function severityGte(a: EventSeverity, b: EventSeverity): boolean {
  return SEVERITY_ORDER.indexOf(a) >= SEVERITY_ORDER.indexOf(b);
}

/**
 * Central event bus — receives events from monitors, evaluates rules,
 * decides enforcement actions. The "brain" between monitors and enforcement.
 */
export class EventEngine {
  private handlers: EventHandler[] = [];
  private enforcementHandlers: EnforcementHandler[] = [];
  private rules: AlertRule[];
  private eventBuffer: ARPEvent[] = [];
  private readonly maxBufferSize = 10000;
  private readonly correlation: CorrelationEngine;
  private readonly executor?: EnforcementExecutor;

  /**
   * @param executor Carries out each matched rule's action before the
   *   enforcement handlers are called. Without one, a matched action is not
   *   carried out and handlers receive `success: false`, `outcome: 'not-executed'`.
   */
  constructor(config: ARPConfig, executor?: EnforcementExecutor) {
    this.rules = config.rules ?? defaultRules();
    this.correlation = new CorrelationEngine();
    this.executor = executor;
  }

  /** Register a handler for all events (for logging, reporting, etc.) */
  onEvent(handler: EventHandler): void {
    this.handlers.push(handler);
  }

  /**
   * Register a handler for enforcement results: the outcome of each matched
   * rule's action, after it ran. Every handler receives the same result object.
   */
  onEnforcement(handler: EnforcementHandler): void {
    this.enforcementHandlers.push(handler);
  }

  /** Emit an event from a monitor — evaluates rules and triggers actions */
  async emit(event: Omit<ARPEvent, 'id' | 'timestamp' | 'classifiedBy'>): Promise<ARPEvent> {
    // Shallow validation for untyped (JS) callers. Without this, an event that
    // omits `data` crashes deep in the pipeline with an opaque
    // "Cannot read properties of undefined (reading 'correlationKey')".
    if (event === null || typeof event !== 'object') {
      throw new TypeError(
        'EventEngine.emit(event): event must be an object with { source, category, severity, description, data }',
      );
    }
    if (event.data === null || typeof event.data !== 'object') {
      throw new TypeError(
        `EventEngine.emit(event): event.data must be an object (received ${
          event.data === undefined ? 'undefined' : typeof event.data
        })`,
      );
    }

    const fullEvent: ARPEvent = {
      ...event,
      id: crypto.randomUUID(),
      timestamp: new Date().toISOString(),
      classifiedBy: 'L0-rules',
    };

    // Buffer for threshold rules
    this.eventBuffer.push(fullEvent);
    if (this.eventBuffer.length > this.maxBufferSize) {
      this.eventBuffer = this.eventBuffer.slice(-this.maxBufferSize / 2);
    }

    // Notify all event handlers
    for (const handler of this.handlers) {
      try {
        await handler(fullEvent);
      } catch {
        // Handler errors don't block the pipeline
      }
    }

    // Evaluate rules
    const matchedRules = this.evaluateRules(fullEvent);
    for (const rule of matchedRules) {
      // CR-002: LLM confirmation is no longer a deferral gate.
      // If a rule matches, enforcement executes immediately.
      // L2 assessment runs asynchronously and can UPGRADE severity,
      // but the initial matched action always fires (fail-closed).
      if (rule.requireLlmConfirmation) {
        // Tag event for L2 follow-up assessment, but enforce NOW
        fullEvent.data._llmReviewRequested = true;
        fullEvent.data._initialAction = rule.action;
        fullEvent.data._matchedRule = rule.name;
      }

      const result = await this.enforce(rule, fullEvent);

      for (const handler of this.enforcementHandlers) {
        try {
          await handler(result);
        } catch {
          // Enforcement handler errors don't block
        }
      }
    }

    // Cross-monitor correlation check (skip for synthetic correlation events to prevent loops)
    if (!fullEvent.data.correlationKey) {
      const correlationEvent = this.correlation.correlate(fullEvent);
      if (correlationEvent) {
        // Emit the synthetic correlation event through the same pipeline
        await this.emit({
          source: correlationEvent.source,
          category: correlationEvent.category,
          severity: correlationEvent.severity,
          description: correlationEvent.description,
          data: correlationEvent.data,
        });
      }
    }

    return fullEvent;
  }

  /** Update an event's classification (called by L1/L2 after re-analysis) */
  reclassify(event: ARPEvent, newCategory: EventCategory, newSeverity: EventSeverity, classifiedBy: ARPEvent['classifiedBy']): ARPEvent {
    event.category = newCategory;
    event.severity = newSeverity;
    event.classifiedBy = classifiedBy;
    return event;
  }

  /** Get recent events matching criteria (for threshold evaluation) */
  getRecentEvents(windowMs: number, source?: string): ARPEvent[] {
    const cutoff = Date.now() - windowMs;
    return this.eventBuffer.filter((e) => {
      const eventTime = new Date(e.timestamp).getTime();
      if (eventTime < cutoff) return false;
      if (source && e.source !== source) return false;
      return true;
    });
  }

  /** Carry out a matched rule's action once and return what happened. */
  private async enforce(rule: AlertRule, event: ARPEvent): Promise<EnforcementResult> {
    const action = rule.action as EnforcementAction;
    if (!this.executor) {
      return {
        action,
        success: false,
        outcome: 'not-executed',
        reason: `Rule "${rule.name}" matched: ${event.description} (not carried out: no enforcement executor is attached)`,
        event,
      };
    }
    let result: EnforcementResult;
    try {
      result = await this.executor.execute(action, event);
    } catch (err) {
      return {
        action,
        success: false,
        outcome: 'failed',
        reason: `Rule "${rule.name}" matched; the ${action} action threw: ${err instanceof Error ? err.message : String(err)}`,
        event,
      };
    }
    // A custom executor may leave `outcome` unset, or return a `success` that
    // disagrees with it. Derive `outcome` from `success` when it is unset, and
    // otherwise derive `success` from `outcome`, so handlers see the two agree.
    if (result.outcome === undefined) {
      return { ...result, outcome: result.success ? 'executed' : 'failed' };
    }
    const success = result.outcome === 'executed';
    return result.success === success ? result : { ...result, success };
  }

  private evaluateRules(event: ARPEvent): AlertRule[] {
    const matched: AlertRule[] = [];

    for (const rule of this.rules) {
      if (this.ruleMatches(rule, event)) {
        matched.push(rule);
      }
    }

    return matched;
  }

  private ruleMatches(rule: AlertRule, event: ARPEvent): boolean {
    const c = rule.condition;

    if (c.source && c.source !== event.source) return false;
    if (c.category && c.category !== event.category) return false;
    if (c.minSeverity && !severityGte(event.severity, c.minSeverity)) return false;

    // Field matching
    if (c.fieldMatch) {
      for (const [key, pattern] of Object.entries(c.fieldMatch)) {
        const value = getNestedValue(event as unknown as Record<string, unknown>, key);
        if (value === undefined) return false;
        if (!matchPattern(String(value), pattern)) return false;
      }
    }

    // Threshold
    if (c.threshold) {
      const recent = this.getRecentEvents(c.threshold.windowMs, c.source);
      if (recent.length < c.threshold.count) return false;
    }

    return true;
  }
}

function getNestedValue(obj: Record<string, unknown>, path: string): unknown {
  const parts = path.split('.');
  let current: unknown = obj;
  for (const part of parts) {
    if (current === null || current === undefined || typeof current !== 'object') return undefined;
    current = (current as Record<string, unknown>)[part];
  }
  return current;
}

function matchPattern(value: string, pattern: string): boolean {
  // Simple glob matching: * matches any sequence
  const regex = new RegExp(
    '^' + pattern.replace(/[.*+?^${}()|[\]\\]/g, '\\$&').replace(/\\\*/g, '.*') + '$'
  );
  return regex.test(value);
}

/** Default rules — sensible security defaults */
function defaultRules(): AlertRule[] {
  return [
    {
      name: 'critical-threat',
      condition: { category: 'threat', minSeverity: 'critical' },
      action: 'kill',
      requireLlmConfirmation: true,
    },
    {
      name: 'high-violation',
      condition: { category: 'violation', minSeverity: 'high' },
      action: 'alert',
    },
    {
      name: 'anomaly-burst',
      condition: {
        category: 'anomaly',
        threshold: { count: 10, windowMs: 60000 },
      },
      action: 'alert',
      requireLlmConfirmation: true,
    },
    // AI-layer default rules
    {
      name: 'prompt-injection-detected',
      condition: { source: 'prompt', category: 'threat', minSeverity: 'high' },
      action: 'alert',
    },
    {
      name: 'credential-leak-detected',
      condition: {
        source: 'prompt',
        category: 'threat',
        fieldMatch: { 'data.patternId': 'OL-001' },
      },
      action: 'alert',
    },
    {
      name: 'mcp-exploitation-detected',
      condition: { source: 'mcp-protocol', category: 'threat', minSeverity: 'high' },
      action: 'alert',
    },
    {
      name: 'a2a-spoofing-detected',
      condition: { source: 'a2a-protocol', category: 'threat', minSeverity: 'high' },
      action: 'alert',
    },
  ];
}
