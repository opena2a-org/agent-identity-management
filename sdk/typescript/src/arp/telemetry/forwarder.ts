/**
 * GTIN Event Forwarder
 *
 * Subscribes to ARP events and forwards anomalous ones to the
 * OpenA2A Registry for community threat intelligence. Events are
 * batched internally and submitted individually (the API accepts
 * one event at a time).
 *
 * Non-blocking: network failures are logged as warnings and never
 * affect ARP monitoring.
 */

import { ARPEvent } from '../types';
import {
  isAnomalousEvent,
  buildGTINPayload,
  submitGTINEvent,
  GTINPayload,
} from './gtin';

/** Configuration for the GTIN forwarder */
export interface GTINForwarderConfig {
  enabled: boolean;
  sensorToken: string;
  registryUrl?: string;
  packageName: string;
  packageVersion?: string;
}

/**
 * GTIN Event Forwarder
 *
 * Accumulates anomalous events and flushes them to the registry
 * every 30 seconds (or on explicit flush/shutdown).
 */
export class GTINForwarder {
  private readonly config: GTINForwarderConfig;
  private queue: GTINPayload[] = [];
  private flushTimer: ReturnType<typeof setInterval> | null = null;
  private stopped = false;

  /** Batch interval in milliseconds (30 seconds) */
  private readonly batchIntervalMs = 30_000;

  constructor(config: GTINForwarderConfig) {
    this.config = config;
  }

  /**
   * Start the forwarder's batch flush timer.
   * Called automatically when the first event is queued, or can be called explicitly.
   */
  start(): void {
    if (this.flushTimer || this.stopped) return;
    this.flushTimer = setInterval(() => {
      this.flush().catch(() => {
        // Non-blocking: swallow flush errors
      });
    }, this.batchIntervalMs);
    // Allow the timer to not prevent process exit
    if (this.flushTimer && typeof this.flushTimer === 'object' && 'unref' in this.flushTimer) {
      this.flushTimer.unref();
    }
  }

  /**
   * Handle an incoming ARP event.
   *
   * If GTIN is enabled and the event is anomalous, it is queued for
   * submission. Normal events are silently ignored.
   */
  onEvent(event: ARPEvent): void {
    if (!this.config.enabled || this.stopped) return;
    if (!isAnomalousEvent(event)) return;

    const payload = buildGTINPayload(
      event,
      this.config.packageName,
      this.config.packageVersion,
    );

    this.queue.push(payload);

    // Auto-start the flush timer on first queued event
    if (!this.flushTimer) {
      this.start();
    }
  }

  /**
   * Force-send all queued events immediately.
   * Each event is submitted individually (the API takes one event at a time).
   */
  async flush(): Promise<void> {
    if (this.queue.length === 0) return;

    // Drain the queue atomically
    const batch = this.queue.splice(0);

    const submissions = batch.map((payload) =>
      submitGTINEvent(payload, this.config.registryUrl).catch((err) => {
        // Log warning but never crash
        if (process.env.ARP_DEBUG) {
          console.warn(`[ARP] GTIN submission failed: ${err instanceof Error ? err.message : 'Unknown error'}`);
        }
        return { success: false, error: String(err) };
      }),
    );

    await Promise.allSettled(submissions);
  }

  /**
   * Flush all queued events and stop the forwarder.
   * After shutdown, no new events are accepted.
   */
  async shutdown(): Promise<void> {
    this.stopped = true;

    if (this.flushTimer) {
      clearInterval(this.flushTimer);
      this.flushTimer = null;
    }

    await this.flush();
  }

  /** Get the current queue length (for diagnostics) */
  getQueueLength(): number {
    return this.queue.length;
  }

  /** Check if the forwarder is running */
  isRunning(): boolean {
    return !this.stopped && this.config.enabled;
  }
}
