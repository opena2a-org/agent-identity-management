/**
 * Structural signature telemetry producer (G2/G4/G5/G7) — public barrel.
 *
 * Default-on / opt-out channel that emits ONLY the structural shape of anomalous
 * runtime behaviors to the OpenA2A Registry, with a local audit log of every byte
 * sent and a single master opt-out. Distinct from the legacy GTIN runtime channel.
 */

export {
  ACTION_CLASSES,
  TARGET_CLASSES,
  TACTIC_IDS,
  OUTCOME_CLASSES,
  SEVERITIES,
  TECHNIQUE_ALLOWLIST,
  isWireTechniqueId,
  redactEvent,
} from './redaction';
export type {
  ActionClass,
  TargetClass,
  TacticId,
  OutcomeClass,
  Severity,
  RedactedSignal,
} from './redaction';

export { HASH_SHAPE_VERSION, canonicalShape, behavioralHash } from './behavioral-hash';

export {
  epochId,
  loadOrgRootSecret,
  loadOrgId,
  orgSaltForEpoch,
  computeOrgPseudonym,
  currentOrgPseudonym,
} from './org-pseudonym';

export {
  loadSensorPrivateKey,
  loadSensorId,
  publicKeyHex,
  signCanonicalHex,
} from './sensor-identity';

export {
  SCHEMA_VERSION,
  SIGNATURE_INGEST_PATH,
  buildCanonical,
  generateNonce,
  buildSignedSubmission,
} from './wire';
export type { TelemetrySignatureRequest, BuiltSubmission } from './wire';

export {
  appendAuditRecord,
  queuedRecord,
  readAuditRecords,
  auditLogPath,
} from './audit-log';
export type { AuditRecord, AuditPhase } from './audit-log';

export {
  isOptedOut,
  optOutMarkerExists,
  signatureTelemetryEnabled,
  resolveRegistryUrl,
  writeOptOutMarker,
  clearOptOutMarker,
} from './config';
export type { SignatureTelemetryConfig } from './config';

export { SignatureEmitter, deriveOutcome } from './emitter';
export type { SignatureEmitterConfig } from './emitter';

export {
  disclosureText,
  hasShownDisclosure,
  markDisclosureShown,
  maybeShowDisclosure,
} from './disclosure';

export {
  PURGE_SCHEMA_VERSION,
  SIGNATURE_PURGE_PATH,
  buildPurgeCanonical,
  buildPurgeProof,
  purgeRemoteSignatures,
  manualPurgeCurl,
} from './purge';
export type { PurgeProofBody, PurgeResult } from './purge';

export {
  ENROLL_SCHEMA_VERSION,
  SENSOR_ENROLL_PATH,
  buildEnrollCanonical,
  buildEnrollProof,
  enrollSensor,
  manualEnrollCurl,
  readEnrollmentRecord,
  writeEnrollmentRecord,
} from './enroll';
export type { EnrollRequestBody, EnrollResult, EnrollmentRecord, EnrollmentState } from './enroll';
