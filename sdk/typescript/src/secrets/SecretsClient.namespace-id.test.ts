/**
 * The namespace routes read the namespace ID from the request path. These
 * tests hold that `SecretsClient` refuses an ID that is not a UUID before a
 * request is made, and sends a UUID as one path segment.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import * as http from 'http';
import type { AddressInfo } from 'net';
import { AIMClient } from '../client/AIMClient';
import { ConfigurationError } from '../exceptions';
import { SecretsClient } from './SecretsClient';
import type { SecretsClientHost } from './SecretsClient';

const NAMESPACE_ID = '0f8fad5b-d9cb-469f-a165-70867728950e';

const REFUSAL =
  'the namespace ID is not a UUID; the request was not sent\n' +
  'Fix: Pass the id the server answered for the namespace, a UUID such as the id of an entry ' +
  'listNamespaces returns.';

/** Each method that puts a namespace ID in the path, with the request it makes for `id`. */
const CALLS: Array<{
  name: string;
  call: (client: SecretsClient, id: string) => Promise<unknown>;
  method: string;
  path: (id: string) => string;
}> = [
  {
    name: 'getNamespace',
    call: (client, id) => client.getNamespace(id),
    method: 'GET',
    path: (id) => `/api/v1/secrets/namespaces/${id}`,
  },
  {
    name: 'deleteNamespace',
    call: (client, id) => client.deleteNamespace(id),
    method: 'DELETE',
    path: (id) => `/api/v1/secrets/namespaces/${id}`,
  },
  {
    name: 'storeCredential',
    call: (client, id) => client.storeCredential({ namespaceId: id, encryptedBlob: new Uint8Array([1]) }),
    method: 'POST',
    path: (id) => `/api/v1/secrets/namespaces/${id}/credentials`,
  },
  {
    name: 'rotateCredential',
    call: (client, id) => client.rotateCredential({ namespaceId: id, encryptedBlob: new Uint8Array([1]) }),
    method: 'POST',
    path: (id) => `/api/v1/secrets/namespaces/${id}/rotate`,
  },
];

// Not a UUID in the form the server answers. Sent as given, `..` climbs out
// of the namespaces route, `/` adds a segment, and `?` and `#` end the path.
const NOT_UUIDS: unknown[] = [
  `../../agents/${NAMESPACE_ID}`,
  `${NAMESPACE_ID}/../../../agents`,
  'a/b',
  `${NAMESPACE_ID}/`,
  `${NAMESPACE_ID}?agentId=${NAMESPACE_ID}`,
  `${NAMESPACE_ID}#x`,
  `${NAMESPACE_ID}\n`,
  ` ${NAMESPACE_ID}`,
  `{${NAMESPACE_ID}}`,
  `urn:uuid:${NAMESPACE_ID}`,
  NAMESPACE_ID.replace(/-/g, ''),
  `${NAMESPACE_ID.slice(0, -1)}g`,
  NAMESPACE_ID.replace(/0/g, '０'),
  '%2e%2e',
  'ns-123',
  '',
  undefined,
  null,
  42,
];

function mockHost(): SecretsClientHost & { request: ReturnType<typeof vi.fn> } {
  return {
    request: vi.fn().mockResolvedValue({}),
    getPrivateKey: () => null,
    getPublicKey: () => null,
  };
}

beforeEach(() => {
  delete process.env.AIM_BASE_URL;
  delete process.env.AIM_API_KEY;
  delete process.env.AIM_AGENT_ID;
  delete process.env.AIM_PRIVATE_KEY;
  delete process.env.AIM_PUBLIC_KEY;
  delete process.env.AIM_ORGANIZATION_ID;
});

describe('SecretsClient: a namespace ID that is not a UUID', () => {
  for (const { name, call } of CALLS) {
    for (const id of NOT_UUIDS) {
      it(`${name} refuses ${JSON.stringify(id) ?? String(id)} and makes no request`, async () => {
        const host = mockHost();

        const sending = call(new SecretsClient(host), id as string);

        await expect(sending).rejects.toBeInstanceOf(ConfigurationError);
        await expect(sending).rejects.toHaveProperty('message', REFUSAL);
        expect(host.request).not.toHaveBeenCalled();
      });
    }
  }
});

describe('SecretsClient: a namespace ID that is a UUID', () => {
  for (const { name, call, method, path } of CALLS) {
    for (const id of [NAMESPACE_ID, NAMESPACE_ID.toUpperCase()]) {
      it(`${name} sends ${id} as the namespace segment`, async () => {
        const host = mockHost();

        await call(new SecretsClient(host), id);

        expect(host.request).toHaveBeenCalledTimes(1);
        expect(host.request.mock.calls[0].slice(0, 2)).toEqual([method, path(id)]);
      });
    }
  }
});

describe('SecretsClient namespace ID against a server on 127.0.0.1', () => {
  it('the server receives no request for an ID that climbs out of the route, and the namespace path for a UUID', async () => {
    const received: string[] = [];
    const server = http.createServer((req, res) => {
      received.push(`${req.method} ${req.url}`);
      req.resume();
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end('{}');
    });
    await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
    try {
      const { port } = server.address() as AddressInfo;
      const aim = new AIMClient({ baseUrl: `http://127.0.0.1:${port}`, apiKey: '', autoRegister: false });
      // The host AIMClient.secrets gives SecretsClient: AIMClient's own request.
      const request = (aim as unknown as { request: SecretsClientHost['request'] }).request.bind(aim);
      const client = new SecretsClient({ request, getPrivateKey: () => null, getPublicKey: () => null });

      for (const { name, call } of CALLS) {
        await expect(call(client, `../../agents/${NAMESPACE_ID}`), name).rejects.toBeInstanceOf(
          ConfigurationError,
        );
      }
      expect(received).toEqual([]);

      for (const { call } of CALLS) {
        await call(client, NAMESPACE_ID);
      }
      expect(received).toEqual(CALLS.map(({ method, path }) => `${method} ${path(NAMESPACE_ID)}`));
    } finally {
      server.closeAllConnections();
      await new Promise<void>((resolve) => server.close(() => resolve()));
    }
  });
});
