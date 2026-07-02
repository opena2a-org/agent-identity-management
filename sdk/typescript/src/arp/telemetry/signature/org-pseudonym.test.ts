import { describe, it, expect } from 'vitest';
import { epochId, computeOrgPseudonym, orgSaltForEpoch } from './org-pseudonym';
import { randomBytes } from 'crypto';

const secret = randomBytes(32);
const orgId = 'org-12345-real-account-id';

describe('epochId', () => {
  it('is the UTC year-month', () => {
    expect(epochId(new Date('2026-06-25T23:59:59Z'))).toBe('2026-06');
    expect(epochId(new Date('2026-01-01T00:00:00Z'))).toBe('2026-01');
    expect(epochId(new Date('2026-12-31T23:59:59Z'))).toBe('2026-12');
  });
});

describe('computeOrgPseudonym', () => {
  it('matches the registry pseudonym floor ^[a-f0-9]{16,64}$', () => {
    expect(computeOrgPseudonym(secret, orgId, '2026-06')).toMatch(/^[a-f0-9]{16,64}$/);
  });

  it('is stable WITHIN an epoch (distinct-org counting + correlation work)', () => {
    expect(computeOrgPseudonym(secret, orgId, '2026-06')).toBe(computeOrgPseudonym(secret, orgId, '2026-06'));
  });

  it('ROTATES across epochs (no long-term tracking)', () => {
    expect(computeOrgPseudonym(secret, orgId, '2026-06')).not.toBe(computeOrgPseudonym(secret, orgId, '2026-07'));
  });

  it('never equals the real org id (one-way)', () => {
    const p = computeOrgPseudonym(secret, orgId, '2026-06');
    expect(p).not.toBe(orgId);
    expect(p).not.toContain(orgId);
  });

  it('differs across orgs under the same secret/epoch', () => {
    expect(computeOrgPseudonym(secret, 'org-A', '2026-06')).not.toBe(computeOrgPseudonym(secret, 'org-B', '2026-06'));
  });

  it('differs across root secrets (the salt is secret-derived)', () => {
    expect(computeOrgPseudonym(randomBytes(32), orgId, '2026-06')).not.toBe(computeOrgPseudonym(randomBytes(32), orgId, '2026-06'));
  });
});

describe('orgSaltForEpoch', () => {
  it('is 32 bytes and rotates per epoch', () => {
    const a = orgSaltForEpoch(secret, '2026-06');
    const b = orgSaltForEpoch(secret, '2026-07');
    expect(a.length).toBe(32);
    expect(a.equals(b)).toBe(false);
  });
});
