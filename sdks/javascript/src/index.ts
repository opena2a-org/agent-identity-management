// Main client
export { AIMClient, AIMClientConfig } from './client';

// Types
export { DetectedMCP, DetectionEvent, DetectionReportRequest, DetectionReportResponse } from './types';

// Detection (Legacy)
export { autoDetectCapabilities } from './detection/capability-detector';
export { autoDetectMCPs as autoDetectMCPsLegacy } from './detection/mcp-detector';
export { autoDetectMCPs } from './detection/capability-detection';

// Intelligent Detection (Tier 1 + Tier 2 + Tier 3)
export {
  intelligentAutoDetectMCPs,
  invalidateDetectionCache,
  IntelligentDetectionConfig,
  PerformanceMetrics,
  IntelligentDetectionResult,
} from './detection/intelligent-detection';

// Signing
export * from './signing';

// OAuth
export * from './oauth';

// Credentials
export * from './credentials';

// Registration
export * from './registration';
