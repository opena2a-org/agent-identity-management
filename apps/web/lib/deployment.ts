/**
 * Deployment flavor. false in the open-source tree; the hosted product's copy
 * of this file (sync-protected in aim-cloud) sets it to true. Surfaces that
 * exist in only one flavor (e.g. the OSS registration-approval queue, which
 * hosted auto-approval can never fill) consult this instead of guessing from
 * the hostname.
 */
export const HOSTED_DEPLOYMENT = false;
