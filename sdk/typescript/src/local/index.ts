/**
 * Local, offline ATX credential verification (spec-compliant local-verify path).
 */
export {
  LocalVerifier,
  CrlCache,
  type LocalVerificationConfig,
  type LocalAuthorizationOptions,
  type LocalAuthorizationResult,
  type CrlData,
  type CrlStalePolicy,
  type CrlCacheConfig,
  type CrlCacheStatus,
  type Atx,
  type AtxPublicKey,
  type AtxTrustAnchors,
  type AtxVerificationResult,
  type RejectCategory,
  type ResolutionContext,
} from './LocalVerifier';
