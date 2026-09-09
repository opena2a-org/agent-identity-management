/**
 * AIM-12 item 9: the client accepted any `timeout`/`enforcementTimeout`
 * (-5, NaN, 0 — each producing an immediately-aborting or never-aborting
 * request budget) and `registerAgent` POSTed whatever it was handed, an empty
 * name included. Misconfiguration should fail loudly at the call site with a
 * ConfigurationError, not surface later as a baffling network timeout or a
 * server-side rejection of a payload the SDK knew was empty.
 */
import { describe, it, expect, vi, afterEach } from 'vitest';
import { AIMClient } from './AIMClient';
import { ConfigurationError } from '../exceptions';

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('constructor timeout validation', () => {
  it('AIM-12.AC3 item 9: timeout: -5 throws ConfigurationError', () => {
    expect(() => new AIMClient({ timeout: -5 })).toThrow(ConfigurationError);
  });

  it('AIM-12.AC3 item 9: timeout: NaN throws ConfigurationError', () => {
    expect(() => new AIMClient({ timeout: Number.NaN })).toThrow(ConfigurationError);
  });

  it('AIM-12.AC3 item 9: timeout: 0 throws ConfigurationError', () => {
    expect(() => new AIMClient({ timeout: 0 })).toThrow(ConfigurationError);
  });

  it('AIM-12.AC3 item 9: enforcementTimeout: -1 throws ConfigurationError', () => {
    expect(() => new AIMClient({ enforcementTimeout: -1 })).toThrow(ConfigurationError);
  });

  it('AIM-12.AC3 item 9: enforcementTimeout: Infinity throws ConfigurationError', () => {
    expect(() => new AIMClient({ enforcementTimeout: Number.POSITIVE_INFINITY })).toThrow(
      ConfigurationError,
    );
  });

  it('valid timeouts still construct (the defaults and every existing positive value)', () => {
    expect(() => new AIMClient()).not.toThrow();
    expect(() => new AIMClient({ timeout: 500, enforcementTimeout: 500 })).not.toThrow();
  });
});

describe('registerAgent input validation', () => {
  it('AIM-12.AC3 item 9: an empty name is refused before any request is made', async () => {
    const fetchSpy = vi.fn(async () => {
      throw new Error('network must not be touched');
    });
    vi.stubGlobal('fetch', fetchSpy);
    const client = new AIMClient({ apiKey: 'k', baseUrl: 'http://localhost:9' });

    await expect(client.registerAgent({ name: '' })).rejects.toThrow(ConfigurationError);
    await expect(client.registerAgent({ name: '   ' })).rejects.toThrow(ConfigurationError);
    await expect(
      client.registerAgent({ name: 42 as unknown as string }),
    ).rejects.toThrow(ConfigurationError);
    expect(fetchSpy).not.toHaveBeenCalled();
  });
});
