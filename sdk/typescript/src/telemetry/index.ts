/**
 * Causal-denial telemetry.
 *
 * Captures why a blocked agent action happened by joining the injection cause
 * (detection inference), the classified intent, and the authorization outcome
 * (enforcement fact) into one correlated record. The full record is
 * authoritative and stays local; only an anonymized indicator may be shared.
 *
 * Everything here is off the enforcement critical path and best-effort.
 */
export {
  CORRELATION_ID_PREFIX,
  CORRELATION_HEADER,
  CORRELATION_BAGGAGE_KEY,
  mintCorrelationId,
  isCorrelationId,
  correlationHeaders,
  extractCorrelationId,
} from './correlation';

export {
  TELEMETRY_SCHEMA_VERSION,
  buildCorrelatedRecord,
  assertObservedInvariant,
  toSharedIndicator,
} from './correlated-record';

export type {
  EnforcementDecision,
  EnforcementOutcome,
  TechniqueSource,
  Completeness,
  EnforcementFact,
  IntentInference,
  DetectionInference,
  AssemblyMeta,
  CorrelatedRecord,
  SharedIndicator,
  BuildCorrelatedRecordInput,
  SharedIndicatorContext,
} from './correlated-record';

export {
  defaultDataDir,
  writeCorrelatedRecord,
  readCorrelatedRecords,
} from './local-writer';

export type { ReadOptions } from './local-writer';
