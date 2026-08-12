import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtempSync, rmSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import {
  isOptedOut,
  isOptedIn,
  signatureTelemetryEnabled,
  resolveRegistryUrl,
  writeOptOutMarker,
  clearOptOutMarker,
  optOutMarkerExists,
} from './config';

let home: string;
beforeEach(() => {
  home = mkdtempSync(join(tmpdir(), 'arp-cfg-'));
  process.env.OPENA2A_HOME = home;
  delete process.env.OPENA2A_TELEMETRY_OPTOUT;
  delete process.env.ARP_TELEMETRY_DISABLED;
  delete process.env.OPENA2A_REGISTRY_URL;
  delete process.env.AIM_TELEMETRY;
});
afterEach(() => {
  delete process.env.OPENA2A_HOME;
  delete process.env.OPENA2A_TELEMETRY_OPTOUT;
  delete process.env.ARP_TELEMETRY_DISABLED;
  delete process.env.OPENA2A_REGISTRY_URL;
  delete process.env.AIM_TELEMETRY;
  rmSync(home, { recursive: true, force: true });
});

describe('the channel is OFF unless explicitly turned on', () => {
  it('does NOT run when nothing is configured — taking no action shares nothing', () => {
    expect(signatureTelemetryEnabled()).toBe(false);
  });

  it('doing nothing is not the same as refusing: nobody opted in, nobody opted out', () => {
    // Guards the invariant that broke the old design, where these were each
    // other's negation and "no action" therefore read as consent.
    expect(isOptedIn()).toBe(false);
    expect(isOptedOut()).toBe(false);
  });

  it('AIM_TELEMETRY turns it on', () => {
    process.env.AIM_TELEMETRY = '1';
    expect(isOptedIn()).toBe(true);
    expect(signatureTelemetryEnabled()).toBe(true);
  });

  it.each(['1', 'true', 'yes', 'on', 'TRUE', ' On '])(
    'AIM_TELEMETRY=%j is a recognised truthy opt-in',
    (v) => {
      process.env.AIM_TELEMETRY = v;
      expect(signatureTelemetryEnabled()).toBe(true);
    },
  );

  it.each(['0', 'false', 'no', 'off', '', 'maybe'])(
    'AIM_TELEMETRY=%j does NOT turn it on',
    (v) => {
      process.env.AIM_TELEMETRY = v;
      expect(signatureTelemetryEnabled()).toBe(false);
    },
  );

  it('config enabled:true turns it on', () => {
    expect(signatureTelemetryEnabled({ enabled: true })).toBe(true);
  });
});

describe('the shipped default config does not turn the channel on', () => {
  // This is the hazard that made the opt-in flip vacuous once already:
  // defaultConfig() carried `signatureTelemetry: { enabled: true }`, which the
  // opt-in check reads as an explicit choice, so every ARP built from defaults
  // ran the channel. Asserted against the REAL loader, not a copy of its shape.
  it('defaultConfig() does not opt in', async () => {
    const { defaultConfig } = await import('../../config/loader');
    expect(signatureTelemetryEnabled(defaultConfig().signatureTelemetry)).toBe(false);
  });

  it('defaultConfig() does not opt OUT either — AIM_TELEMETRY must still work', async () => {
    // A `{ enabled: false }` default would also make this test's sibling pass
    // while permanently disabling the env var, so pin both directions.
    const { defaultConfig } = await import('../../config/loader');
    process.env.AIM_TELEMETRY = '1';
    expect(signatureTelemetryEnabled(defaultConfig().signatureTelemetry)).toBe(true);
  });
});

describe('an opt-out always beats an opt-in', () => {
  it('config enabled:false wins over AIM_TELEMETRY=1', () => {
    process.env.AIM_TELEMETRY = '1';
    expect(signatureTelemetryEnabled({ enabled: false })).toBe(false);
  });

  it('OPENA2A_TELEMETRY_OPTOUT wins over config enabled:true', () => {
    process.env.OPENA2A_TELEMETRY_OPTOUT = '1';
    expect(signatureTelemetryEnabled({ enabled: true })).toBe(false);
  });

  it('ARP_TELEMETRY_DISABLED wins over config enabled:true', () => {
    process.env.ARP_TELEMETRY_DISABLED = 'true';
    expect(signatureTelemetryEnabled({ enabled: true })).toBe(false);
  });

  it('the marker file wins over AIM_TELEMETRY=1', () => {
    process.env.AIM_TELEMETRY = '1';
    writeOptOutMarker();
    expect(signatureTelemetryEnabled()).toBe(false);
  });
});

describe('master opt-out — unchanged semantics (still gates GTIN and the pre-send re-check)', () => {
  it('config enabled:false opts out', () => {
    expect(isOptedOut({ enabled: false })).toBe(true);
  });

  it('OPENA2A_TELEMETRY_OPTOUT opts out', () => {
    process.env.OPENA2A_TELEMETRY_OPTOUT = '1';
    expect(isOptedOut()).toBe(true);
  });

  it('ARP_TELEMETRY_DISABLED opts out', () => {
    process.env.ARP_TELEMETRY_DISABLED = 'true';
    expect(isOptedOut()).toBe(true);
  });

  it('the marker file opts out, and clearing it lifts the refusal', () => {
    expect(optOutMarkerExists()).toBe(false);
    writeOptOutMarker();
    expect(optOutMarkerExists()).toBe(true);
    expect(isOptedOut()).toBe(true);
    clearOptOutMarker();
    expect(optOutMarkerExists()).toBe(false);
    expect(isOptedOut()).toBe(false);
    // Clearing a refusal must NOT itself start the channel.
    expect(signatureTelemetryEnabled()).toBe(false);
  });
});

describe('resolveRegistryUrl', () => {
  it('prefers config, then env, then the default', () => {
    expect(resolveRegistryUrl({ registryUrl: 'https://cfg.test' })).toBe('https://cfg.test');
    process.env.OPENA2A_REGISTRY_URL = 'https://env.test';
    expect(resolveRegistryUrl()).toBe('https://env.test');
    delete process.env.OPENA2A_REGISTRY_URL;
    expect(resolveRegistryUrl()).toBe('https://api.oa2a.org');
  });
});
