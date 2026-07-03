/**
 * GTIN Telemetry — public API barrel exports
 */

export {
  generateSensorToken,
  buildGTINPayload,
  submitGTINEvent,
  isAnomalousEvent,
  mapEventType,
  detectRuntimeEnv,
  getDaySinceInstall,
  type GTINEventType,
  type GTINRuntimeEnv,
  type GTINPayload,
  type GTINSubmitResult,
} from './gtin';

export {
  GTINForwarder,
  type GTINForwarderConfig,
} from './forwarder';
