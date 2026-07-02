import type { ARPEvent, EventSeverity, MonitorType } from '../types';

const SEVERITY_ORDER: EventSeverity[] = ['info', 'low', 'medium', 'high', 'critical'];

/**
 * Cross-monitor event correlation engine.
 * Maintains a sliding window of recent events and detects when events
 * from 2+ different monitor sources occur within the correlation window,
 * emitting a synthetic 'correlation' event with threat category.
 */
export class CorrelationEngine {
  /** Sliding window of recent events */
  private readonly recentEvents: ARPEvent[] = [];
  /** Correlation window in ms (default: 60 seconds) */
  private readonly windowMs: number;
  /** Set of correlation IDs already emitted (to avoid duplicates within a window) */
  private readonly emittedCorrelations = new Set<string>();

  constructor(windowMs: number = 60000) {
    this.windowMs = windowMs;
  }

  /**
   * Process an incoming event and check for cross-monitor correlation.
   * Returns a synthetic correlation event if events from 2+ sources are
   * found within the window, or null if no correlation detected.
   */
  correlate(event: ARPEvent): ARPEvent | null {
    const now = Date.now();
    const cutoff = now - this.windowMs;

    // Prune old events and old correlation IDs
    while (this.recentEvents.length > 0) {
      const oldest = new Date(this.recentEvents[0].timestamp).getTime();
      if (oldest < cutoff) {
        this.recentEvents.shift();
      } else {
        break;
      }
    }

    // Also prune emitted correlations periodically (keep set small)
    if (this.emittedCorrelations.size > 1000) {
      this.emittedCorrelations.clear();
    }

    // Add the new event
    this.recentEvents.push(event);

    // Collect distinct sources in the window
    const sourceSet = new Set<MonitorType>();
    for (const e of this.recentEvents) {
      sourceSet.add(e.source);
    }

    if (sourceSet.size < 2) {
      return null; // Need events from at least 2 different sources
    }

    // Build a correlation key from sorted sources to avoid duplicate emissions
    const sortedSources = Array.from(sourceSet).sort();
    const correlationKey = sortedSources.join('+');

    if (this.emittedCorrelations.has(correlationKey)) {
      return null; // Already emitted this correlation within the window
    }

    this.emittedCorrelations.add(correlationKey);

    // Find the highest severity among correlated events
    let highestSeverity: EventSeverity = 'info';
    for (const e of this.recentEvents) {
      if (SEVERITY_ORDER.indexOf(e.severity) > SEVERITY_ORDER.indexOf(highestSeverity)) {
        highestSeverity = e.severity;
      }
    }

    // Escalate severity by one level for correlation (capped at critical)
    const escalatedIdx = Math.min(
      SEVERITY_ORDER.indexOf(highestSeverity) + 1,
      SEVERITY_ORDER.length - 1,
    );
    const escalatedSeverity = SEVERITY_ORDER[escalatedIdx];

    return {
      id: `corr-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      timestamp: new Date().toISOString(),
      source: this.recentEvents[this.recentEvents.length - 1].source,
      category: 'threat',
      severity: escalatedSeverity,
      description: `Cross-monitor correlation: events detected from ${sortedSources.join(', ')} within ${this.windowMs / 1000}s window`,
      data: {
        correlatedSources: sortedSources,
        eventCount: this.recentEvents.length,
        windowMs: this.windowMs,
        correlationKey,
      },
      classifiedBy: 'L1-statistical',
    };
  }

  /** Reset correlation state */
  reset(): void {
    this.recentEvents.length = 0;
    this.emittedCorrelations.clear();
  }

  /** Get the number of events currently in the window */
  getWindowSize(): number {
    return this.recentEvents.length;
  }
}
