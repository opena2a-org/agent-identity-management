/**
 * Guard anomaly detector (AIComply P1, distribution drift layer).
 *
 * This module complements the per-event behavioral risk channel
 * (`behavioral-risk.ts`) with a statistical layer that watches the
 * CLASSIFICATION DISTRIBUTION over a sliding window and flags drift
 * from a reference baseline. The two layers are additive and catch
 * different attacks:
 *
 *   - Behavioral risk scores a single event against the agent's
 *     behavioral baseline: "does THIS action look anomalous?" It
 *     catches individual outliers.
 *   - Guard anomaly scores the recent class mix against a reference
 *     distribution: "has the TYPE of events drifted from what the
 *     Guard was trained on?" It catches regime shifts.
 *
 * A slow attacker whose individual events stay below the per-event
 * risk threshold but whose actions collectively shift the class
 * distribution (for example a sustained 5x over-rate of a
 * sensitive-class label relative to the training baseline) is
 * invisible to the per-event layer and obvious to the drift
 * detector.
 *
 * ## Design decisions
 *
 * ### 1. Sliding ring buffer over reservoir sample
 *
 * A ring buffer of the most recent `windowSize` classification labels
 * gives exact recency with bounded memory. A reservoir sample would
 * preserve uniformly-drawn history and let stale events dilute the
 * current drift signal, which is the opposite of what drift detection
 * wants: the question is whether the CURRENT distribution matches
 * the baseline, not whether history mixed with the present does.
 *
 * ### 2. Chi-square goodness-of-fit over z-score or KL divergence
 *
 * Chi-square is the principled test for a multi-class observed
 * distribution against a reference. It operates on raw count vectors,
 * produces a single scalar statistic, and has a well-understood null
 * distribution (chi-squared with K-1 degrees of freedom) that gives
 * caller-interpretable thresholds.
 *
 * Z-score is wrong for a multi-class comparison because it measures
 * deviation of a single scalar from its own baseline; it cannot
 * capture joint drift across the class vector. KL divergence is a
 * reasonable alternative but lacks a principled threshold tied to a
 * null distribution, which matters when the caller needs to reason
 * about false-positive rates in a security context.
 *
 * ### 3. Injected baseline with Laplace smoothing
 *
 * The baseline class distribution is a dependency of this detector,
 * not a concern of this module. The caller is responsible for
 * providing it from the authoritative source. Two shipping paths are
 * supported by callers of this module:
 *
 *   - Registry-exported training distribution. The NanoMind-Guard
 *     training pipeline computes the class distribution over the
 *     training corpus and ships it out-of-band through the Registry.
 *     The runtime fetches it at startup and passes it here. This is
 *     the target deployment state.
 *
 *   - Observation-based bootstrap. In pre-Registry deployments a
 *     caller can collect observations over a "learning" period and
 *     build a baseline from them via `buildBaselineFromObservations`.
 *     This is intentionally a weaker source: the baseline is only as
 *     trustworthy as the period it was collected over.
 *
 * Laplace smoothing handles zero-count classes on both sides without
 * special casing. The smoothed baseline probability for class c with
 * K total classes is `(p(c) + alpha) / (1 + alpha * K)`, which
 * preserves sum-to-one and guarantees strictly positive expected
 * counts so the chi-square denominator never underflows. The default
 * alpha of 0.01 is intentionally small so that a window which mirrors
 * the baseline scores a chi-square near zero; larger alpha values
 * pull the smoothed baseline toward uniform and bias the null-case
 * statistic upward.
 *
 * ### 4. Bounded memory contract
 *
 * A ring buffer of at most `windowSize` label strings plus a count
 * map whose size is bounded by the number of distinct classes seen in
 * the window (in practice <20 for current AIComply label sets, hard
 * bounded by `windowSize`). A per-label length cap of
 * `MAX_CLASSIFICATION_LENGTH` characters guards against an adversary
 * passing pathologically long labels to exhaust memory; longer labels
 * are rejected as a record-time no-op so the window remains
 * O(windowSize * MAX_CLASSIFICATION_LENGTH) bytes in the worst case.
 * `windowSize` itself is clamped to `MAX_GUARD_ANOMALY_WINDOW` so a
 * misconfigured caller cannot blow up memory either.
 *
 * ### 5. Alarm surfacing
 *
 * Parallel to the session 28 behavioral risk channel, the coordinator
 * records the `GuardAnomalyStatus` on `event.data.guardAnomaly` for
 * every classified event. On the `drift` branch the coordinator
 * raises category to `anomaly` and severity to `medium`; on
 * `baseline-pending` and `normal` the record is written without
 * mutation. This is the record-and-lift policy: observation is
 * always recorded so downstream audit can see the drift state at the
 * time of each event, and enforcement kicks in only when the
 * chi-square statistic crosses the configured threshold.
 *
 * ## Parse-to-deny (CR-001)
 *
 * The detector never throws. `record()` rejects non-string or empty
 * classifications as a no-op that returns the current status without
 * mutating internal state. `installBaseline` silently drops invalid
 * entries; if the baseline is empty after installation, the detector
 * reports `baseline-pending` forever rather than inventing a default.
 * Callers must treat `drift` as authoritative and must not use the
 * statistic itself to make per-event decisions beyond the threshold
 * comparison already encoded here.
 */

/**
 * Baseline class distribution. Keys are classification labels; values
 * are either probabilities that sum to ~1 or raw counts. The detector
 * normalizes internally, so either form works. Entries with invalid
 * keys or non-finite or negative values are silently dropped.
 */
export type ClassDistribution = Record<string, number>;

/**
 * One entry in the top-K deviation list on a drift status: a class
 * name and how far its observed count deviated from its smoothed
 * expected count. Positive deviation means the class was
 * over-represented in the window, negative means under-represented.
 */
export interface ClassDeviation {
  className: string;
  observed: number;
  expected: number;
  deviation: number;
}

/**
 * Discriminated union returned by `record()`. Callers route on
 * `status`:
 *
 *   - `baseline-pending` when the observation window contains fewer
 *     than `minObservations`, or when the installed baseline is
 *     empty. Record-only on the coordinator side: no severity raise.
 *
 *   - `normal` when the chi-square statistic is under threshold.
 *     Record-only.
 *
 *   - `drift` when the chi-square statistic is at or above threshold.
 *     Carries the top-K most-deviated classes so downstream audit can
 *     see which classes drove the alarm without re-running the chi-
 *     square computation.
 */
export type GuardAnomalyStatus =
  | {
      status: 'baseline-pending';
      /** How many valid observations are currently in the window. */
      observed: number;
      /** Minimum observations required before drift scoring activates. */
      required: number;
      /** Reason the detector is pending: not enough data, or empty baseline. */
      reason: 'insufficient-observations' | 'empty-baseline';
    }
  | {
      status: 'normal';
      statistic: number;
      threshold: number;
      windowSize: number;
      source: string;
    }
  | {
      status: 'drift';
      statistic: number;
      threshold: number;
      topDeviations: ClassDeviation[];
      windowSize: number;
      source: string;
    };

/**
 * Construction config. Only `baseline` is required; sensible defaults
 * are provided for every other knob.
 */
export interface GuardAnomalyConfig {
  /** Baseline class distribution (counts or probabilities). */
  baseline: ClassDistribution;
  /**
   * Maximum sliding window size. Default: 200. Clamped to
   * [1, MAX_GUARD_ANOMALY_WINDOW] by the constructor so a misconfigured
   * caller cannot allocate unbounded memory.
   */
  windowSize?: number;
  /**
   * Chi-square statistic threshold that triggers a drift alarm.
   * Default: 21.666 (~chi^2_{0.01} for df=9, a reasonable value for
   * ~10-class distributions at 99% confidence). Callers with larger
   * K should increase this; callers with smaller K should decrease it.
   */
  alarmThreshold?: number;
  /**
   * Laplace smoothing alpha. Default: 0.01. Larger alpha pulls the
   * smoothed baseline toward uniform, reducing sensitivity to rare
   * classes AND biasing a null-case (observed matches baseline)
   * statistic upward; smaller alpha reduces smoothing at the cost of
   * instability on zero-count classes. The default is tuned so that
   * a baseline-matching window over a ~100 event window scores under
   * 0.5 and never crosses the default alarm threshold.
   */
  smoothing?: number;
  /**
   * Minimum observations before drift scoring activates. Default:
   * max(30, floor(windowSize / 4)). Below this, `record()` returns
   * `baseline-pending`.
   */
  minObservations?: number;
  /** Audit source tag surfaced on the status object. Default: 'guard-anomaly'. */
  sourceName?: string;
  /**
   * How many top deviations to include in a drift status. Default: 3.
   * The full sorted list is truncated to this count.
   */
  topKDeviations?: number;
}

/**
 * Minimal interface the coordinator holds. The concrete detector
 * implements this; tests that want to inject bespoke behavior can
 * stub it directly without instantiating a full detector.
 */
export interface GuardAnomalySource {
  /**
   * Record a classification observation and return the current drift
   * status. Must never throw. An empty, non-string, or over-long
   * classification is a no-op that returns the current status.
   */
  record(classification: string): GuardAnomalyStatus;
}

/** Default sliding-window size if the caller does not override. */
export const DEFAULT_GUARD_ANOMALY_WINDOW = 200;

/**
 * Hard cap on the sliding-window size. Bounds worst-case memory to
 * `MAX_GUARD_ANOMALY_WINDOW * MAX_CLASSIFICATION_LENGTH` bytes of
 * label storage.
 */
export const MAX_GUARD_ANOMALY_WINDOW = 10_000;

/**
 * Default chi-square alarm threshold. Approximately the critical
 * value for chi-squared with 9 degrees of freedom at p=0.01, which
 * is a reasonable default for distributions with about 10 classes
 * and a 1% tolerable false-positive rate.
 */
export const DEFAULT_GUARD_ANOMALY_THRESHOLD = 21.666;

/**
 * Default Laplace smoothing constant. Tuned small so a null-case
 * window (observed matches baseline) scores near zero. See the
 * module header for the rationale on why 0.01 was chosen over the
 * textbook 0.5.
 */
export const DEFAULT_GUARD_ANOMALY_SMOOTHING = 0.01;

/** Default top-K deviation count surfaced on a drift status. */
export const DEFAULT_GUARD_ANOMALY_TOP_K = 3;

/** Absolute floor on minObservations regardless of windowSize. */
export const DEFAULT_GUARD_ANOMALY_MIN_OBS = 30;

/**
 * Maximum accepted classification label length. Labels longer than
 * this are rejected at `record()` as a no-op so a malicious producer
 * cannot inflate memory by writing pathologically long labels into
 * the window.
 */
export const MAX_CLASSIFICATION_LENGTH = 256;

/**
 * Statistical drift detector over a sliding window of classification
 * labels. Thread-unsafe by construction; callers are expected to
 * serialize calls through the coordinator's analyze() loop.
 */
export class GuardAnomalyDetector implements GuardAnomalySource {
  private readonly windowSize: number;
  private readonly alarmThreshold: number;
  private readonly smoothing: number;
  private readonly minObservations: number;
  private readonly sourceName: string;
  private readonly topKDeviations: number;

  /** Normalized baseline probabilities (sum to 1 when non-empty). */
  private baselineNorm: Map<string, number>;

  /** Ring buffer of label strings; indices in [0, windowSize). */
  private readonly window: Array<string | undefined>;
  /** Next write index. Wraps at windowSize. */
  private head: number;
  /** True once the ring has wrapped at least once. */
  private filled: boolean;
  /** Running count of each class currently in the window. */
  private readonly counts: Map<string, number>;

  constructor(config: GuardAnomalyConfig) {
    this.windowSize = clampWindowSize(config.windowSize ?? DEFAULT_GUARD_ANOMALY_WINDOW);
    this.alarmThreshold = Number.isFinite(config.alarmThreshold)
      ? Math.max(0, config.alarmThreshold as number)
      : DEFAULT_GUARD_ANOMALY_THRESHOLD;
    this.smoothing = Number.isFinite(config.smoothing)
      ? Math.max(0, config.smoothing as number)
      : DEFAULT_GUARD_ANOMALY_SMOOTHING;
    this.minObservations = Math.max(
      1,
      config.minObservations ??
        Math.max(DEFAULT_GUARD_ANOMALY_MIN_OBS, Math.floor(this.windowSize / 4)),
    );
    this.sourceName = config.sourceName ?? 'guard-anomaly';
    this.topKDeviations = Math.max(1, config.topKDeviations ?? DEFAULT_GUARD_ANOMALY_TOP_K);

    this.baselineNorm = new Map();
    this.installBaseline(config.baseline);

    this.window = new Array(this.windowSize).fill(undefined);
    this.head = 0;
    this.filled = false;
    this.counts = new Map();
  }

  /**
   * Record a classification observation and return the current drift
   * status. Never throws. Returns the current status (without
   * recording) when the classification is empty, not a string, or
   * exceeds `MAX_CLASSIFICATION_LENGTH`.
   */
  record(classification: string): GuardAnomalyStatus {
    if (
      typeof classification !== 'string' ||
      classification.length === 0 ||
      classification.length > MAX_CLASSIFICATION_LENGTH
    ) {
      return this.computeStatus();
    }

    // Evict the oldest entry when the ring has wrapped. `head` points
    // to the next write slot, which is also the oldest entry once
    // the buffer is filled.
    if (this.filled) {
      const evicted = this.window[this.head];
      if (evicted !== undefined) {
        const prev = this.counts.get(evicted) ?? 0;
        if (prev <= 1) {
          this.counts.delete(evicted);
        } else {
          this.counts.set(evicted, prev - 1);
        }
      }
    }

    this.window[this.head] = classification;
    this.counts.set(classification, (this.counts.get(classification) ?? 0) + 1);

    this.head++;
    if (this.head >= this.windowSize) {
      this.head = 0;
      this.filled = true;
    }

    return this.computeStatus();
  }

  /** Current number of observations in the sliding window. */
  getWindowObservations(): number {
    return this.filled ? this.windowSize : this.head;
  }

  /** Configured window capacity. */
  getWindowCapacity(): number {
    return this.windowSize;
  }

  /**
   * Snapshot of the current observed class counts. Returns a new
   * object; safe to mutate without affecting the detector.
   */
  getObserved(): ClassDistribution {
    const out: ClassDistribution = {};
    for (const [k, v] of this.counts) {
      out[k] = v;
    }
    return out;
  }

  /**
   * Snapshot of the installed (normalized) baseline. Returns a new
   * object.
   */
  getBaseline(): ClassDistribution {
    const out: ClassDistribution = {};
    for (const [k, v] of this.baselineNorm) {
      out[k] = v;
    }
    return out;
  }

  /**
   * Replace the baseline without tearing down the detector. Used by
   * a caller that rotates the Guard's training distribution in
   * place. The observation window is preserved so drift detection
   * continues without a re-learning period.
   */
  setBaseline(baseline: ClassDistribution): void {
    this.installBaseline(baseline);
  }

  /**
   * Clear the observation window. Baseline is untouched. Intended
   * for callers that want to reset state after a confirmed drift
   * resolution or a configuration change.
   */
  reset(): void {
    for (let i = 0; i < this.window.length; i++) {
      this.window[i] = undefined;
    }
    this.head = 0;
    this.filled = false;
    this.counts.clear();
  }

  private installBaseline(baseline: ClassDistribution): void {
    const next = new Map<string, number>();
    let total = 0;
    if (baseline !== null && typeof baseline === 'object') {
      for (const [k, v] of Object.entries(baseline)) {
        if (typeof k !== 'string' || k.length === 0 || k.length > MAX_CLASSIFICATION_LENGTH) {
          continue;
        }
        if (typeof v !== 'number' || !Number.isFinite(v) || v < 0) {
          continue;
        }
        next.set(k, v);
        total += v;
      }
    }
    this.baselineNorm = new Map();
    if (total > 0) {
      for (const [k, v] of next) {
        this.baselineNorm.set(k, v / total);
      }
    }
  }

  private computeStatus(): GuardAnomalyStatus {
    const n = this.getWindowObservations();
    if (this.baselineNorm.size === 0) {
      return {
        status: 'baseline-pending',
        observed: n,
        required: this.minObservations,
        reason: 'empty-baseline',
      };
    }
    if (n < this.minObservations) {
      return {
        status: 'baseline-pending',
        observed: n,
        required: this.minObservations,
        reason: 'insufficient-observations',
      };
    }

    const { statistic, topDeviations } = this.computeChiSquare(n);
    if (statistic >= this.alarmThreshold) {
      return {
        status: 'drift',
        statistic,
        threshold: this.alarmThreshold,
        topDeviations: topDeviations.slice(0, this.topKDeviations),
        windowSize: n,
        source: this.sourceName,
      };
    }
    return {
      status: 'normal',
      statistic,
      threshold: this.alarmThreshold,
      windowSize: n,
      source: this.sourceName,
    };
  }

  /**
   * Compute the Laplace-smoothed chi-square goodness-of-fit statistic
   * of the current window against the installed baseline, and the
   * full per-class deviation list sorted by absolute deviation.
   *
   * Smoothed baseline:
   *   smoothed_p(c) = (p(c) + alpha) / (1 + alpha * K)
   * where K = |universe|, the union of baseline classes and observed
   * classes. This preserves sum-to-one and yields strictly positive
   * expected counts for every c in the universe, so the chi-square
   * denominator never underflows.
   */
  private computeChiSquare(
    n: number,
  ): { statistic: number; topDeviations: ClassDeviation[] } {
    const universe = new Set<string>();
    for (const k of this.baselineNorm.keys()) universe.add(k);
    for (const k of this.counts.keys()) universe.add(k);

    const K = universe.size;
    if (K === 0 || n === 0) {
      return { statistic: 0, topDeviations: [] };
    }

    const alpha = this.smoothing;
    const denom = 1 + alpha * K;

    let statistic = 0;
    const deviations: ClassDeviation[] = [];

    for (const className of universe) {
      const p = this.baselineNorm.get(className) ?? 0;
      const smoothedP = (p + alpha) / denom;
      const expected = smoothedP * n;
      const observed = this.counts.get(className) ?? 0;
      if (expected <= 0) continue; // defensive; impossible with alpha > 0 or p > 0
      const diff = observed - expected;
      statistic += (diff * diff) / expected;
      deviations.push({
        className,
        observed,
        expected,
        deviation: diff,
      });
    }

    deviations.sort((a, b) => Math.abs(b.deviation) - Math.abs(a.deviation));
    return { statistic, topDeviations: deviations };
  }
}

function clampWindowSize(n: number): number {
  if (!Number.isFinite(n) || n <= 0) return DEFAULT_GUARD_ANOMALY_WINDOW;
  return Math.min(Math.floor(n), MAX_GUARD_ANOMALY_WINDOW);
}

/**
 * Build a baseline `ClassDistribution` from an array of observation
 * labels. Intended for the observation-based bootstrap path where no
 * Registry-exported baseline is yet available. The returned object
 * is a count map; the detector normalizes it internally.
 *
 * Callers should collect at least several hundred observations over
 * a trusted "learning" period before calling this. A baseline built
 * from fewer observations is noise, and a baseline built from a
 * compromised period bakes the attack into the reference.
 *
 * Entries that are not non-empty strings are silently skipped.
 */
export function buildBaselineFromObservations(
  samples: ReadonlyArray<string>,
): ClassDistribution {
  const counts: ClassDistribution = {};
  for (const s of samples) {
    if (typeof s !== 'string' || s.length === 0 || s.length > MAX_CLASSIFICATION_LENGTH) {
      continue;
    }
    counts[s] = (counts[s] ?? 0) + 1;
  }
  return counts;
}
