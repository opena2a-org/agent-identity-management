/**
 * ARP License Module
 *
 * Checks for a valid ARP premium license. The open-source edition
 * includes L0 regex detection and alert-only proxy mode.
 *
 * Premium features (blocking mode, SDK wrappers, L2 AI-layer assessment,
 * custom patterns, dashboard export) require a valid license key.
 *
 * License validation logic is provided by the @opena2a/arp-premium package.
 * This stub exposes the check interface so the core can gate features
 * without depending on the premium package.
 */

export type LicenseTier = 'community' | 'pro' | 'team' | 'enterprise';

export interface LicenseInfo {
  /** License tier */
  tier: LicenseTier;
  /** Whether the license is currently valid */
  valid: boolean;
  /** Organization name (if licensed) */
  organization?: string;
  /** Expiry date (ISO string) */
  expiresAt?: string;
  /** Features enabled by this license */
  features: Set<string>;
}

/** Premium feature identifiers */
export const PREMIUM_FEATURES = {
  /** Block requests on threat detection (not just alert) */
  BLOCKING_MODE: 'blocking-mode',
  /** L2 LLM assessment for AI-layer threats */
  AI_LAYER_L2: 'ai-layer-l2',
  /** SDK wrappers (wrapOpenAI, wrapMCP) */
  SDK_WRAPPERS: 'sdk-wrappers',
  /** Custom pattern authoring and import */
  CUSTOM_PATTERNS: 'custom-patterns',
  /** SIEM/dashboard export */
  SIEM_EXPORT: 'siem-export',
  /** Compliance report generation */
  COMPLIANCE_REPORTS: 'compliance-reports',
} as const;

/** Validator function type -- provided by @opena2a/arp-premium */
type LicenseValidator = (key: string) => LicenseInfo | Promise<LicenseInfo>;

/** Registered external validator (set by premium package) */
let externalValidator: LicenseValidator | null = null;

/**
 * Register a license validator. Called by @opena2a/arp-premium
 * when it is imported alongside @opena2a/arp.
 */
export function registerLicenseValidator(validator: LicenseValidator): void {
  externalValidator = validator;
}

/**
 * Check the current license status.
 * Returns community tier if no license key or validator is present.
 */
export async function checkLicense(): Promise<LicenseInfo> {
  const key = process.env.ARP_LICENSE_KEY;

  if (!key || !externalValidator) {
    return communityLicense();
  }

  try {
    const info = await externalValidator(key);
    return info;
  } catch {
    return communityLicense();
  }
}

/**
 * Check if a specific premium feature is available.
 */
export async function hasFeature(feature: string): Promise<boolean> {
  const license = await checkLicense();
  return license.features.has(feature);
}

/**
 * Synchronous community license -- used as default.
 */
function communityLicense(): LicenseInfo {
  return {
    tier: 'community',
    valid: true,
    features: new Set<string>(),
  };
}
