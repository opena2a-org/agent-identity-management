import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtempSync, rmSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import {
  isOptedOut,
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
});
afterEach(() => {
  delete process.env.OPENA2A_HOME;
  delete process.env.OPENA2A_TELEMETRY_OPTOUT;
  delete process.env.ARP_TELEMETRY_DISABLED;
  delete process.env.OPENA2A_REGISTRY_URL;
  rmSync(home, { recursive: true, force: true });
});

describe('master opt-out — default-on', () => {
  it('is ON by default (no opt-out source active)', () => {
    expect(isOptedOut()).toBe(false);
    expect(signatureTelemetryEnabled()).toBe(true);
  });

  it('config enabled:false opts out', () => {
    expect(isOptedOut({ enabled: false })).toBe(true);
    expect(signatureTelemetryEnabled({ enabled: false })).toBe(false);
  });

  it('OPENA2A_TELEMETRY_OPTOUT opts out', () => {
    process.env.OPENA2A_TELEMETRY_OPTOUT = '1';
    expect(isOptedOut()).toBe(true);
  });

  it('ARP_TELEMETRY_DISABLED opts out', () => {
    process.env.ARP_TELEMETRY_DISABLED = 'true';
    expect(isOptedOut()).toBe(true);
  });

  it('the marker file opts out, and clearing it re-enables', () => {
    expect(optOutMarkerExists()).toBe(false);
    writeOptOutMarker();
    expect(optOutMarkerExists()).toBe(true);
    expect(isOptedOut()).toBe(true);
    clearOptOutMarker();
    expect(optOutMarkerExists()).toBe(false);
    expect(isOptedOut()).toBe(false);
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
