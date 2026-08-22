/**
 * `aim-arp telemetry status` is a READ command and must not mint identity
 * state. Before this guarantee existed, running status on a clean machine
 * created three files — including the org root secret — as a side effect of
 * looking (found in the CPO release review, 2026-08-22). A consent-surface
 * read command that creates a persistent identity fails the
 * verified-as-a-user bar.
 */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { existsSync, mkdtempSync, readdirSync, rmSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { telemetryStatus } from './telemetry';
import { peekSensorId } from '../telemetry/signature/sensor-identity';
import { peekOrgPseudonym, currentOrgPseudonym, loadOrgRootSecret, loadOrgId } from '../telemetry/signature/org-pseudonym';

let home: string;
let savedHome: string | undefined;

beforeEach(() => {
  home = join(mkdtempSync(join(tmpdir(), 'aim-arp-status-')), 'opena2a-home');
  savedHome = process.env.OPENA2A_HOME;
  process.env.OPENA2A_HOME = home;
  vi.spyOn(console, 'log').mockImplementation(() => {});
});

afterEach(() => {
  if (savedHome === undefined) delete process.env.OPENA2A_HOME;
  else process.env.OPENA2A_HOME = savedHome;
  vi.restoreAllMocks();
  rmSync(join(home, '..'), { recursive: true, force: true });
});

describe('telemetry status is side-effect-free', () => {
  it('creates NO files under a clean home', async () => {
    expect(existsSync(home)).toBe(false);
    await telemetryStatus(undefined);
    // The strongest possible assertion: the home directory itself was never
    // created, so no identity file can have been minted.
    expect(existsSync(home)).toBe(false);
  });

  it('reports identity as not-yet-created instead of minting one', async () => {
    const lines: string[] = [];
    vi.mocked(console.log).mockImplementation((...a: unknown[]) => {
      lines.push(a.join(' '));
    });
    await telemetryStatus(undefined);
    const out = lines.join('\n');
    expect(out).toContain('none yet');
    expect(out).not.toContain('undefined');
  });
});

describe('peek functions', () => {
  it('peekSensorId returns null on a clean home and creates nothing', () => {
    expect(peekSensorId()).toBeNull();
    expect(existsSync(home)).toBe(false);
  });

  it('peekOrgPseudonym returns null on a clean home and creates nothing', () => {
    expect(peekOrgPseudonym()).toBeNull();
    expect(existsSync(home)).toBe(false);
  });

  it('peekOrgPseudonym equals currentOrgPseudonym once state exists', () => {
    // Mint identity through the write path, then the peek must agree with it.
    loadOrgRootSecret();
    loadOrgId();
    const now = new Date('2026-08-22T00:00:00Z');
    expect(peekOrgPseudonym(now)).toBe(currentOrgPseudonym(now));
    expect(readdirSync(home).length).toBeGreaterThan(0);
  });

  it('peekSensorId honors the env override without touching disk', () => {
    process.env.OPENA2A_SENSOR_ID = ' pinned-id ';
    try {
      expect(peekSensorId()).toBe('pinned-id');
      expect(existsSync(home)).toBe(false);
    } finally {
      delete process.env.OPENA2A_SENSOR_ID;
    }
  });
});

describe('opt-out and purge do not mint identity on a machine that never sent', () => {
  it('purge on a clean home creates nothing and says there is nothing to purge', async () => {
    const lines: string[] = [];
    vi.mocked(console.log).mockImplementation((...a: unknown[]) => {
      lines.push(a.join(' '));
    });
    const { telemetryPurge } = await import('./telemetry');
    await telemetryPurge(undefined);
    expect(existsSync(home)).toBe(false);
    expect(lines.join('\n')).toContain('nothing to purge');
  });
});
