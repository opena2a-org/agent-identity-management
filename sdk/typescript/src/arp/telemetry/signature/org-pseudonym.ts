/**
 * Rotating salted org pseudonym (G4).
 *
 * The registry must count DISTINCT orgs reporting a hash (k-anonymity, G3) and
 * tell "first seen at A" from "blocked at B", WITHOUT a stable per-customer
 * identifier that would let an observer track one customer across hashes over
 * time. See opena2a-registry/docs/telemetry-behavioral-hash-spec.md.
 *
 *   orgSalt_epoch = HKDF(orgRootSecret, info="telemetry-org-pseudonym|"+epochId)
 *   orgPseudonym  = HMAC-SHA256(orgSalt_epoch, orgId)  [truncated to 32 hex]
 *
 * - orgRootSecret and orgId never leave the device.
 * - Stable WITHIN a UTC month (so distinct-org counting + first-seen/blocked
 *   correlation work); rotates ACROSS months (so no long-term tracking).
 */

import { createHmac, hkdfSync, randomBytes, randomUUID } from 'crypto';
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'fs';
import { opena2aHome, homePath, ORG_ROOT_SECRET_FILE, ORG_ID_FILE } from './paths';

const PSEUDONYM_INFO_PREFIX = 'telemetry-org-pseudonym|';
/** 16 bytes -> 32 hex chars, inside the registry's ^[a-f0-9]{16,64}$ floor. */
const PSEUDONYM_HEX_LEN = 32;

/** UTC year-month epoch id, e.g. "2026-06". The pseudonym rotation boundary. */
export function epochId(now: Date = new Date()): string {
  const y = now.getUTCFullYear();
  const m = String(now.getUTCMonth() + 1).padStart(2, '0');
  return `${y}-${m}`;
}

function ensureHome(): void {
  const home = opena2aHome();
  if (!existsSync(home)) mkdirSync(home, { recursive: true });
}

/**
 * Load (or first-time generate) the per-install org root secret. 32 random bytes,
 * persisted 0600. Never transmitted. A real multi-sensor org can share this file
 * (and the org id) across its sensors so they produce the SAME pseudonym.
 */
export function loadOrgRootSecret(): Buffer {
  const p = homePath(ORG_ROOT_SECRET_FILE);
  if (existsSync(p)) {
    const hex = readFileSync(p, 'utf8').trim();
    const buf = Buffer.from(hex, 'hex');
    if (buf.length === 32) return buf;
  }
  ensureHome();
  const secret = randomBytes(32);
  writeFileSync(p, secret.toString('hex'), { mode: 0o600 });
  return secret;
}

/**
 * Load (or first-time generate) the local org identity. Defaults to OPENA2A_ORG_ID
 * when set (so an operator can pin a shared org id across machines), else a stable
 * locally-generated uuid. This value is HMAC'd under a rotating secret salt and is
 * itself NEVER transmitted.
 */
export function loadOrgId(): string {
  const fromEnv = process.env.OPENA2A_ORG_ID;
  if (fromEnv && fromEnv.trim()) return fromEnv.trim();
  const p = homePath(ORG_ID_FILE);
  if (existsSync(p)) {
    const id = readFileSync(p, 'utf8').trim();
    if (id) return id;
  }
  ensureHome();
  const id = randomUUID();
  writeFileSync(p, id, { mode: 0o600 });
  return id;
}

/** Derive the per-epoch secret salt from the root secret (HKDF-SHA256). */
export function orgSaltForEpoch(orgRootSecret: Buffer, epoch: string): Buffer {
  const info = Buffer.from(PSEUDONYM_INFO_PREFIX + epoch, 'utf8');
  // Empty salt is fine: the IKM (orgRootSecret) is already high-entropy.
  const okm = hkdfSync('sha256', orgRootSecret, Buffer.alloc(0), info, 32);
  return Buffer.from(okm);
}

/**
 * Compute the rotating org pseudonym for the given epoch. Pure function of its
 * inputs (testable without disk/clock).
 */
export function computeOrgPseudonym(orgRootSecret: Buffer, orgId: string, epoch: string): string {
  const salt = orgSaltForEpoch(orgRootSecret, epoch);
  return createHmac('sha256', salt).update(orgId, 'utf8').digest('hex').slice(0, PSEUDONYM_HEX_LEN);
}

/** Resolve the current org pseudonym from local state and the current month. */
export function currentOrgPseudonym(now: Date = new Date()): string {
  return computeOrgPseudonym(loadOrgRootSecret(), loadOrgId(), epochId(now));
}
