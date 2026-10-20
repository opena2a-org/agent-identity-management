/**
 * `SecretsClient.resolve` signs the UTF-8 of `namespace|operation|nonce`, which
 * starts with the caller's namespace. These tests hold what it refuses before
 * anything is signed or sent, and what it still signs and sends.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import * as ed from '@noble/ed25519';
import { sign } from '../crypto/ed25519';
import { ConfigurationError } from '../exceptions';
import { SecretsClient } from './SecretsClient';
import type { SecretsClientHost } from './SecretsClient';

// The signer, wrapped so a test can tell whether anything was signed.
vi.mock('../crypto/ed25519', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../crypto/ed25519')>();
  return { ...actual, sign: vi.fn(actual.sign) };
});

const signSpy = vi.mocked(sign);

/** A host holding a new agent key. Its one request is recorded and refused with `sent`. */
async function signingHost(): Promise<{
  host: SecretsClientHost & { request: ReturnType<typeof vi.fn> };
  publicKey: Uint8Array;
}> {
  const privateKey = ed.utils.randomPrivateKey();
  const publicKey = await ed.getPublicKeyAsync(privateKey);
  const request = vi.fn().mockRejectedValue(new Error('sent'));
  return {
    host: {
      request,
      getPrivateKey: () => Buffer.from(privateKey).toString('base64'),
      getPublicKey: () => Buffer.from(publicKey).toString('base64'),
    },
    publicKey,
  };
}

const FIELD_FIX =
  'Fix: Pass a namespace and an operation that contain no line feed, carriage return or vertical bar (|).';

function fieldRefusal(field: 'namespace' | 'operation'): string {
  return `the ${field} contains a line feed, a carriage return or a vertical bar (|); the request was not sent\n${FIELD_FIX}`;
}

const DSSE_PREFIX_REFUSAL =
  'the namespace begins with "DSSEv1 ", the first bytes of a DSSE statement an agent key signs; ' +
  'the request was not sent\nFix: Pass a namespace that does not begin with "DSSEv1 ".';

beforeEach(() => {
  signSpy.mockClear();
});

async function expectRefusedBeforeSigning(namespace: string, operation: string, message: string): Promise<void> {
  const { host } = await signingHost();

  const resolving = new SecretsClient(host).resolve(namespace, operation);

  await expect(resolving).rejects.toBeInstanceOf(ConfigurationError);
  await expect(resolving).rejects.toHaveProperty('message', message);
  expect(signSpy).not.toHaveBeenCalled();
  expect(host.request).not.toHaveBeenCalled();
}

describe('SecretsClient.resolve: refusals before anything is signed or sent', () => {
  const characters = [
    ['a line feed', 'a\nb'],
    ['a carriage return', 'a\rb'],
    ['a vertical bar', 'a|b'],
  ] as const;

  for (const field of ['namespace', 'operation'] as const) {
    for (const [name, value] of characters) {
      it(`refuses ${name} in the ${field}`, async () => {
        const fields = { namespace: 'my-ns', operation: 'read', [field]: value };
        await expectRefusedBeforeSigning(fields.namespace, fields.operation, fieldRefusal(field));
      });
    }
  }

  // The DSSE v1 pre-authentication encoding of a statement starts with
  // "DSSEv1", a space, the payload type's length, the type and a space.
  const dssePrefixed = [
    'DSSEv1 45 application/vnd.opena2a.agent-request.v2+json 2 {}',
    'DSSEv1 46 application/vnd.opena2a.action-request.v1+json 2 {}',
    'DSSEv1 53 application/vnd.opena2a.delegation-revocation.v1+json 2 {}',
    'DSSEv1 ',
  ];

  for (const namespace of dssePrefixed) {
    it(`refuses the namespace ${JSON.stringify(namespace)}, which begins with "DSSEv1 "`, async () => {
      await expectRefusedBeforeSigning(namespace, 'read', DSSE_PREFIX_REFUSAL);
    });
  }

  // The text of an agent-request-v1 signature starts with the method in upper
  // case and a line feed.
  for (const method of ['GET', 'POST', 'PUT', 'PATCH', 'DELETE']) {
    it(`refuses a namespace that begins with ${method} and a line feed`, async () => {
      await expectRefusedBeforeSigning(`${method}\n/api/v1/agents`, 'read', fieldRefusal('namespace'));
    });
  }
});

describe('SecretsClient.resolve: namespaces it signs and sends', () => {
  // None of these begins with the seven characters "DSSEv1 ".
  const namespaces = ['my-ns', 'DSSEv1', 'dssev1 creds', ' DSSEv1 creds', 'creds DSSEv1 x'];

  for (const namespace of namespaces) {
    it(`signs ${JSON.stringify(namespace)}|read|nonce once and sends it once`, async () => {
      const { host, publicKey } = await signingHost();

      await expect(new SecretsClient(host).resolve(namespace, 'read')).rejects.toThrow('sent');

      expect(host.request).toHaveBeenCalledTimes(1);
      const [method, path, payload] = host.request.mock.calls[0] as [string, string, Record<string, string>];
      expect(method).toBe('POST');
      expect(path).toBe('/api/v1/secrets/resolve');
      expect(payload.namespace).toBe(namespace);

      const signed = new TextEncoder().encode(`${namespace}|read|${payload.nonce}`);
      expect(signSpy).toHaveBeenCalledTimes(1);
      expect(signSpy.mock.calls[0][0]).toEqual(signed);
      expect(await ed.verifyAsync(Buffer.from(payload.signature, 'base64'), signed, publicKey)).toBe(true);
    });
  }
});
