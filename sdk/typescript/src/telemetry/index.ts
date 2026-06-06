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

export type {
  EnforcementInput,
  IntentInput,
  DetectionInput,
} from './correlated-record';

export {
  defaultDataDir,
  writeCorrelatedRecord,
  readCorrelatedRecords,
} from './local-writer';

export type { ReadOptions } from './local-writer';

export { CorrelationJoiner, isHighConfidence } from './joiner';

export type {
  JoinerOptions,
  EnforcementEvent,
  IntentEvent,
  DetectionEvent,
} from './joiner';

export {
  MATRIX_SNAPSHOT_VERSION,
  TECHNIQUE_ID_RE,
  KNOWN_TECHNIQUE_IDS,
  INTERIM_ATTACK_CLASS_MAP,
  isValidTechniqueId,
  isTechniqueIdFormat,
  mapAttackClass,
  interimTechniqueFields,
} from './technique-mapping';

export type { InterimMappingEntry } from './technique-mapping';

export {
  CorrelatedRelay,
  DEFAULT_REGISTRY_URL,
  RUNTIME_TELEMETRY_PATH,
  RELAY_EVENT_TYPE,
  SENTINEL_PACKAGE_NAME,
  openA2AHome,
  deriveSensorToken,
  detectRuntimeEnv,
  daysSinceInstall,
} from './relay';

export type { RelayConfig, FlushResult } from './relay';
