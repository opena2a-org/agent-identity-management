import type { EventEngine } from '../engine/event-engine';
import type { MonitorType } from '../types';

/**
 * Why part of the input was not matched against any pattern:
 * - 'truncated': the analyzer scanned only the first `MAX_SCAN_TOTAL`
 *   characters of a longer text (or, for `scanToolCall`, of a longer argument).
 * - 'over-size-limit': the input was over a size limit and was not scanned at all.
 */
export type TruncationReason = 'truncated' | 'over-size-limit';

export interface TruncationDetails {
  reason: TruncationReason;
  /** How much of the input was matched against the patterns. */
  matchedLength: number;
  /** How much input the analyzer received. */
  receivedLength: number;
  /** Unit of `matchedLength` and `receivedLength`. */
  unit: 'characters' | 'bytes';
}

/**
 * Emit the one event for an analyzer call that left input unmatched.
 *
 * The event is a high-severity `violation`, so the default `high-violation`
 * rule alerts on it. It carries the reason and the two lengths, and never any
 * of the input text.
 */
export function emitTruncation(
  engine: EventEngine,
  source: MonitorType,
  analyzer: string,
  details: TruncationDetails,
): void {
  const { reason, matchedLength, receivedLength, unit } = details;
  const what = reason === 'over-size-limit' ? 'was over the size limit and' : 'was longer than the scan total, so';
  void engine.emit({
    source,
    category: 'violation',
    severity: 'high',
    description:
      `${analyzer}: input ${what} only ${matchedLength} of ${receivedLength} ${unit} were matched against patterns`,
    data: {
      truncation: true,
      analyzer,
      reason,
      matchedLength,
      receivedLength,
      unit,
    },
  });
}
