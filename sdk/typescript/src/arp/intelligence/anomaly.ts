import * as fs from 'fs';
import * as path from 'path';
import type { ARPEvent, MonitorType } from '../types';

interface TimeSeriesPoint {
  timestamp: number;
  value: number;
}

interface BaselineStats {
  mean: number;
  stddev: number;
  count: number;
}

/**
 * L1: Statistical anomaly detection.
 * Tracks event frequency per monitor, computes z-scores for deviation detection.
 * Learns baseline over time — no training data needed.
 */
export class AnomalyDetector {
  /** Event counts per minute per monitor */
  private readonly timeSeries = new Map<MonitorType, TimeSeriesPoint[]>();
  /** Rolling baseline stats per monitor */
  private readonly baselines = new Map<MonitorType, BaselineStats>();
  /** Maximum history to keep (7 days at 1-minute resolution) */
  private readonly maxHistory = 10080;
  /** Minimum data points before anomaly detection activates */
  private readonly minDataPoints = 30;

  /** Score an event's anomaly level. Returns z-score (0 = normal, >2 = anomalous). */
  score(event: ARPEvent): number {
    const currentMinute = Math.floor(Date.now() / 60000);
    const series = this.timeSeries.get(event.source) ?? [];

    // Count events in current minute
    const currentCount = series.filter((p) => p.timestamp === currentMinute).length + 1;

    // Get baseline
    const baseline = this.baselines.get(event.source);
    if (!baseline || baseline.count < this.minDataPoints) {
      return 0; // Not enough data for anomaly detection
    }

    // Z-score: how many standard deviations from mean?
    if (baseline.stddev === 0) {
      return currentCount > baseline.mean ? 2.5 : 0;
    }

    return Math.abs(currentCount - baseline.mean) / baseline.stddev;
  }

  /** Record an event for baseline learning */
  record(event: ARPEvent): void {
    const currentMinute = Math.floor(Date.now() / 60000);

    if (!this.timeSeries.has(event.source)) {
      this.timeSeries.set(event.source, []);
    }

    const series = this.timeSeries.get(event.source)!;
    series.push({ timestamp: currentMinute, value: 1 });

    // Trim old data
    const cutoff = currentMinute - this.maxHistory;
    const trimIndex = series.findIndex((p) => p.timestamp >= cutoff);
    if (trimIndex > 0) {
      series.splice(0, trimIndex);
    }

    // Update baseline (incremental mean/stddev)
    this.updateBaseline(event.source, series);
  }

  /** Get the current baseline for a monitor */
  getBaseline(source: MonitorType): BaselineStats | null {
    return this.baselines.get(source) ?? null;
  }

  /** Reset all baselines */
  reset(): void {
    this.timeSeries.clear();
    this.baselines.clear();
  }

  /** Persist timeSeries and baselines to disk as JSON */
  save(dataDir: string): void {
    const filePath = path.join(dataDir, 'anomaly-baselines.json');
    const payload = {
      timeSeries: Object.fromEntries(
        Array.from(this.timeSeries.entries()).map(([k, v]) => [k, v]),
      ),
      baselines: Object.fromEntries(
        Array.from(this.baselines.entries()).map(([k, v]) => [k, v]),
      ),
    };
    fs.mkdirSync(dataDir, { recursive: true });
    fs.writeFileSync(filePath, JSON.stringify(payload, null, 2), 'utf-8');
  }

  /** Load persisted timeSeries and baselines from disk */
  static load(dataDir: string): AnomalyDetector {
    const filePath = path.join(dataDir, 'anomaly-baselines.json');
    const detector = new AnomalyDetector();

    if (!fs.existsSync(filePath)) {
      return detector;
    }

    try {
      const raw = fs.readFileSync(filePath, 'utf-8');
      const parsed = JSON.parse(raw) as {
        timeSeries: Record<string, TimeSeriesPoint[]>;
        baselines: Record<string, BaselineStats>;
      };

      if (parsed.timeSeries) {
        for (const [key, value] of Object.entries(parsed.timeSeries)) {
          detector.timeSeries.set(key as MonitorType, value);
        }
      }
      if (parsed.baselines) {
        for (const [key, value] of Object.entries(parsed.baselines)) {
          detector.baselines.set(key as MonitorType, value);
        }
      }
    } catch {
      // Corrupted file — start fresh
    }

    return detector;
  }

  private updateBaseline(source: MonitorType, series: TimeSeriesPoint[]): void {
    // Aggregate: count events per unique minute
    const minuteCounts = new Map<number, number>();
    for (const point of series) {
      minuteCounts.set(point.timestamp, (minuteCounts.get(point.timestamp) ?? 0) + 1);
    }

    const values = Array.from(minuteCounts.values());
    if (values.length === 0) return;

    const count = values.length;
    const mean = values.reduce((a, b) => a + b, 0) / count;
    const variance = values.reduce((a, b) => a + (b - mean) ** 2, 0) / count;
    const stddev = Math.sqrt(variance);

    this.baselines.set(source, { mean, stddev, count });
  }
}
