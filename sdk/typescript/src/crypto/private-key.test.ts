/**
 * One decoder for every agent private key the SDK signs with.
 *
 * AIM issues an agent private key as standard padded base64 of 64 bytes: the
 * seed, then its public key. The signer takes the 32-byte seed alone. Each
 * consumer decoded the key on its own: `SecretsClient` with a lenient base64
 * decode that took an unpadded or whitespace-wrapped key and passed all 64
 * bytes of the issued form to the signer, and `sign` and `createDelegation`
 * passed a hex string straight to it. These tests pin the one rule: 32 bytes,
 * or the 64 bytes AIM issues whose last 32 are the public key of the first
 * 32, given as bytes or as standard padded base64 that re-encodes to itself.
 * Every other key is refused with one Fix that says how to convert it.
 */

import { randomBytes } from 'node:crypto';
import * as ed from '@noble/ed25519';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { normalizePrivateKey, PrivateKeyError, PRIVATE_KEY_FIX } from './private-key';
import { createAgentRequestHeaders } from './agent-request';
import { createDelegation, verifyDelegationSignature } from './delegation';
import { createRequestSignature, sign } from './ed25519';
import { AIMClient } from '../client/AIMClient';
import { SecretsClient, type SecretsClientHost } from '../secrets/SecretsClient';
import { ConfigurationError } from '../exceptions';

const b64 = (bytes: Uint8Array): string => Buffer.from(bytes).toString('base64');
const hex = (bytes: Uint8Array): string => Buffer.from(bytes).toString('hex');

interface TestKey {
  seed: Uint8Array;
  pub: Uint8Array;
  issued: Uint8Array;
  seedB64: string;
  issuedB64: string;
  pubB64: string;
}

async function makeKey(): Promise<TestKey> {
  const seed = new Uint8Array(randomBytes(32));
  const pub = await ed.getPublicKeyAsync(seed);
  const issued = new Uint8Array(64);
  issued.set(seed, 0);
  issued.set(pub, 32);
  return { seed, pub, issued, seedB64: b64(seed), issuedB64: b64(issued), pubB64: b64(pub) };
}

/** The refusal `fn` throws, checked to be the one class, carrying the Fix and no part of the key. */
function refusalOf(fn: () => unknown, key?: TestKey, input?: string): PrivateKeyError {
  let caught: unknown = null;
  try {
    fn();
  } catch (error) {
    caught = error;
  }
  return checkedRefusal(caught, key, input);
}

async function asyncRefusalOf(
  promise: Promise<unknown>,
  key?: TestKey,
  input?: string
): Promise<PrivateKeyError> {
  const caught = await promise.then(
    () => null,
    (error: unknown) => error
  );
  return checkedRefusal(caught, key, input);
}

function checkedRefusal(caught: unknown, key?: TestKey, input?: string): PrivateKeyError {
  expect(caught).toBeInstanceOf(PrivateKeyError);
  expect(caught).toBeInstanceOf(ConfigurationError);
  const error = caught as PrivateKeyError;
  expect(error.fix).toBe(PRIVATE_KEY_FIX);
  expect(error.message).toContain(`\nFix: ${PRIVATE_KEY_FIX}`);
  if (key) {
    for (const part of [key.seedB64, key.issuedB64, hex(key.seed), hex(key.issued)]) {
      expect(error.message).not.toContain(part);
    }
  }
  if (input && input.trim().length > 0) {
    expect(error.message).not.toContain(input.trim());
  }
  return error;
}

/** The last base64 character of `s` with its unused low bits changed: a non-canonical encoding of the same bytes. */
function nonCanonical(s: string): string {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
  const at = s.replace(/=+$/, '').length - 1;
  const index = alphabet.indexOf(s[at]);
  const unusedBits = s.endsWith('==') ? 0x0f : 0x03;
  const bumped = alphabet[(index & ~unusedBits) | ((index + 1) & unusedBits)];
  return s.slice(0, at) + bumped + s.slice(at + 1);
}

let key: TestKey;
let other: TestKey;

beforeEach(async () => {
  key = await makeKey();
  other = await makeKey();
});

describe('normalizePrivateKey: accepted forms', () => {
  const message = new TextEncoder().encode('agent-request-v1 test message');

  it('returns the seed of the 64-byte form AIM issues, and that seed signs and verifies under its public key', async () => {
    const seed = normalizePrivateKey(key.issuedB64);
    expect(seed).toHaveLength(32);
    expect(b64(seed)).toBe(key.seedB64);

    const signature = await ed.signAsync(message, seed);
    expect(await ed.verifyAsync(signature, message, key.pub)).toBe(true);
  });

  it('signs identically with the 32-byte form and the 64-byte form', async () => {
    const fromSeed = await ed.signAsync(message, normalizePrivateKey(key.seedB64));
    const fromIssued = await ed.signAsync(message, normalizePrivateKey(key.issuedB64));
    expect(b64(fromIssued)).toBe(b64(fromSeed));
  });

  it('accepts the key as 32 or 64 bytes, and returns a copy of the seed', () => {
    const fromSeedBytes = normalizePrivateKey(key.seed);
    const fromIssuedBytes = normalizePrivateKey(key.issued);
    expect(b64(fromSeedBytes)).toBe(key.seedB64);
    expect(b64(fromIssuedBytes)).toBe(key.seedB64);

    fromSeedBytes[0] ^= 0xff;
    expect(b64(key.seed)).toBe(key.seedB64);
  });
});

describe('normalizePrivateKey: refusals', () => {
  it('refuses a 64-byte key whose second half is not the public key derived from its first half, naming the mismatch', () => {
    const swapped = new Uint8Array(64);
    swapped.set(key.seed, 0);
    swapped.set(other.pub, 32);
    const flipped = key.issued.slice();
    flipped[63] ^= 0x01;

    for (const input of [b64(swapped), b64(flipped)]) {
      const error = refusalOf(() => normalizePrivateKey(input), key, input);
      expect(error.message).toMatch(
        /^the agent private key is 64 bytes long, and its last 32 bytes are not its public key; /
      );
    }
    const asBytes = refusalOf(() => normalizePrivateKey(swapped), key);
    expect(asBytes.message).toContain('its last 32 bytes are not its public key');
  });

  const formRefusals: Array<[string, () => string, RegExp]> = [
    ['an unpadded 32-byte key', () => key.seedB64.replace(/=+$/, ''), /^the agent private key is not standard base64; /],
    ['an unpadded 64-byte key', () => key.issuedB64.replace(/=+$/, ''), /^the agent private key is not standard base64; /],
    ['a key with a leading space', () => ` ${key.seedB64}`, /^the agent private key is not standard base64; /],
    ['a key with a trailing newline', () => `${key.issuedB64}\n`, /^the agent private key is not standard base64; /],
    [
      'a key wrapped onto two lines',
      () => `${key.issuedB64.slice(0, 44)}\n${key.issuedB64.slice(44)}`,
      /^the agent private key is not standard base64; /,
    ],
    [
      'a URL-safe base64 key',
      () => Buffer.from(new Uint8Array(32).fill(0xfb)).toString('base64url') + '=',
      /^the agent private key is not standard base64; /,
    ],
    ['a 32-byte hex key', () => hex(key.seed), /^the agent private key is 48 bytes long, not 32 or 64; /],
    ['a 64-byte hex key', () => hex(key.issued), /^the agent private key is 96 bytes long, not 32 or 64; /],
  ];

  for (const [name, input, expected] of formRefusals) {
    it(`refuses ${name} with the Fix`, () => {
      const value = input();
      const error = refusalOf(() => normalizePrivateKey(value), key, value);
      expect(error.message).toMatch(expected);
    });
  }

  it('refuses a key a lenient decoder reads, but which does not re-encode to itself', () => {
    for (const canonical of [key.seedB64, key.issuedB64]) {
      const lenient = nonCanonical(canonical);
      expect(lenient).not.toBe(canonical);
      // A lenient decoder reads the same bytes, so only the re-encoding tells them apart.
      expect(Buffer.from(lenient, 'base64').toString('base64')).toBe(canonical);

      const error = refusalOf(() => normalizePrivateKey(lenient), key, lenient);
      expect(error.message).toMatch(/^the agent private key is not standard base64; /);
    }
  });

  it('refuses a key that is neither 32 nor 64 bytes long, as base64 or as bytes', () => {
    for (const length of [0, 31, 33, 48, 65]) {
      const bytes = new Uint8Array(randomBytes(length));
      const asBytes = refusalOf(() => normalizePrivateKey(bytes));
      expect(asBytes.message).toMatch(new RegExp(`^the agent private key is ${length} bytes long, not 32 or 64; `));
      if (length > 0) {
        const asBase64 = refusalOf(() => normalizePrivateKey(b64(bytes)));
        expect(asBase64.message).toMatch(new RegExp(`^the agent private key is ${length} bytes long, not 32 or 64; `));
      }
    }
  });

  it('refuses an absent key', () => {
    for (const absent of ['', undefined, null, 42]) {
      const error = refusalOf(() => normalizePrivateKey(absent as unknown as string));
      expect(error.message).toMatch(/^no agent private key is configured; nothing was signed\nFix: /);
    }
  });

  it('the Fix names each conversion, and each conversion it names yields a key that is accepted', () => {
    const toHexFix = "Buffer.from(key, 'hex').toString('base64')";
    const toBase64Fix = "Buffer.from(key, 'base64').toString('base64')";
    expect(PRIVATE_KEY_FIX).toContain(toHexFix);
    expect(PRIVATE_KEY_FIX).toContain(toBase64Fix);

    for (const hexKey of [hex(key.seed), hex(key.issued)]) {
      expect(b64(normalizePrivateKey(Buffer.from(hexKey, 'hex').toString('base64')))).toBe(key.seedB64);
    }
    const base64Forms = [
      key.seedB64.replace(/=+$/, ''),
      key.issuedB64.replace(/=+$/, ''),
      ` ${key.seedB64}`,
      `${key.issuedB64.slice(0, 44)}\n${key.issuedB64.slice(44)}`,
      Buffer.from(key.issued).toString('base64url'),
    ];
    for (const form of base64Forms) {
      expect(b64(normalizePrivateKey(Buffer.from(form, 'base64').toString('base64')))).toBe(key.seedB64);
    }
  });
});

describe('every consumer of an agent private key goes through the one decoder', () => {
  const BASE_URL = 'https://aim.example.com';
  const AGENT_ID = '550e8400-e29b-41d4-a716-446655440000';
  const mockFetch = vi.fn();

  beforeEach(() => {
    mockFetch.mockReset();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.unstubAllEnvs();
  });

  function secretsHost(privateKey: string, publicKey: string) {
    const request = vi.fn(async () => ({}));
    const host: SecretsClientHost = {
      request: request as unknown as SecretsClientHost['request'],
      getPrivateKey: () => privateKey,
      getPublicKey: () => publicKey,
    };
    return { host, request };
  }

  it('createAgentRequestHeaders refuses a hex key with the Fix', async () => {
    const value = hex(key.seed);
    await asyncRefusalOf(
      createAgentRequestHeaders(
        { method: 'GET', url: `${BASE_URL}/p` },
        { agentId: AGENT_ID, privateKey: value, publicKey: key.pubB64 }
      ),
      key,
      value
    );
  });

  it('AIMClient refuses a hex key set with setCredentials with the Fix, and sends nothing', async () => {
    const value = hex(key.seed);
    const client = new AIMClient({ baseUrl: BASE_URL });
    client.setCredentials({
      agentId: AGENT_ID,
      privateKey: value,
      publicKey: key.pubB64,
      organizationId: 'org-1',
      createdAt: new Date().toISOString(),
    });
    await asyncRefusalOf(client.verifyAction({ action: 'db:read' }), key, value);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it('AIMClient refuses a hex AIM_PRIVATE_KEY from the environment with the Fix, and sends nothing', async () => {
    const value = hex(key.issued);
    vi.stubEnv('AIM_AGENT_ID', AGENT_ID);
    vi.stubEnv('AIM_PRIVATE_KEY', value);
    vi.stubEnv('AIM_PUBLIC_KEY', key.pubB64);
    vi.stubEnv('AIM_ORGANIZATION_ID', 'org-1');
    const client = new AIMClient({ baseUrl: BASE_URL });
    expect(client.getCredentials()?.privateKey).toBe(value);
    await asyncRefusalOf(client.verifyAction({ action: 'db:read' }), key, value);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it('SecretsClient refuses a hex key with the Fix, and sends nothing', async () => {
    const value = hex(key.seed);
    const { host, request } = secretsHost(value, key.pubB64);
    await asyncRefusalOf(new SecretsClient(host).resolve('github-creds', 'read'), key, value);
    expect(request).not.toHaveBeenCalled();
  });

  it('SecretsClient refuses an unpadded key with the Fix, and sends nothing', async () => {
    const value = key.seedB64.replace(/=+$/, '');
    const { host, request } = secretsHost(value, key.pubB64);
    await asyncRefusalOf(new SecretsClient(host).resolve('github-creds', 'read'), key, value);
    expect(request).not.toHaveBeenCalled();
  });

  it('SecretsClient signs a resolution with the 64-byte form AIM issues, under its public key', async () => {
    const { host, request } = secretsHost(key.issuedB64, key.pubB64);
    // The empty answer ends the call after the signed request was made.
    await expect(new SecretsClient(host).resolve('github-creds', 'read')).rejects.toThrow(
      'Server returned incomplete resolution response'
    );
    expect(request).toHaveBeenCalledTimes(1);
    const payload = (request.mock.calls[0] as unknown[])[2] as Record<string, string>;
    const signed = new TextEncoder().encode(`github-creds|read|${payload.nonce}`);
    expect(await ed.verifyAsync(Buffer.from(payload.signature, 'base64'), signed, key.pub)).toBe(true);
  });

  it('createDelegation refuses a hex KeyPair.privateKey with the Fix', async () => {
    const value = hex(key.seed);
    await asyncRefusalOf(
      createDelegation({
        delegatorKeyPair: { privateKey: value as unknown as Uint8Array, publicKey: key.pub },
        delegatePublicKey: other.pub,
        scopes: ['db:read'],
      }),
      key,
      value
    );
  });

  it('createDelegation signs with a KeyPair.privateKey in the 64-byte form AIM issues', async () => {
    const delegation = await createDelegation({
      delegatorKeyPair: { privateKey: key.issued, publicKey: key.pub },
      delegatePublicKey: other.pub,
      scopes: ['db:read'],
    });
    expect(await verifyDelegationSignature(delegation)).toBe(true);
  });

  it('createDelegation refuses a 64-byte KeyPair.privateKey whose second half is another public key', async () => {
    const swapped = new Uint8Array(64);
    swapped.set(key.seed, 0);
    swapped.set(other.pub, 32);
    const error = await asyncRefusalOf(
      createDelegation({
        delegatorKeyPair: { privateKey: swapped, publicKey: key.pub },
        delegatePublicKey: other.pub,
        scopes: ['db:read'],
      }),
      key
    );
    expect(error.message).toContain('its last 32 bytes are not its public key');
  });

  it('sign and createRequestSignature refuse a hex KeyPair.privateKey with the Fix', async () => {
    const value = hex(key.seed);
    const message = new TextEncoder().encode('m');
    await asyncRefusalOf(sign(message, value as unknown as Uint8Array), key, value);
    await asyncRefusalOf(
      createRequestSignature(value as unknown as Uint8Array, 'GET', '/api/v1/agents'),
      key,
      value
    );
  });

  it('sign signs with a KeyPair.privateKey in the 64-byte form AIM issues', async () => {
    const message = new TextEncoder().encode('m');
    const signature = await sign(message, key.issued);
    expect(await ed.verifyAsync(signature, message, key.pub)).toBe(true);
  });
});
