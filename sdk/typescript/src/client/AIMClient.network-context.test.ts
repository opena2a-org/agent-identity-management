/**
 * Tests for connection-failure diagnostics with an EXPLICIT baseUrl
 * (AIM-11 item 6): the NetworkError message must name the request URL and the
 * underlying errno (unwrapped from the fetch error's cause chain), matching
 * the diagnostic quality of the defaulted-baseUrl hint.
 *
 * Uses the real global fetch against a loopback port that is known to refuse
 * connections — never a non-loopback host.
 */

import { describe, it, expect, beforeEach } from 'vitest';
import { createServer } from 'net';
import { AIMClient } from './AIMClient';
import { NetworkError } from '../exceptions';

/** Bind an ephemeral loopback port, then release it so connections are refused. */
async function refusedLoopbackPort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const server = createServer();
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => {
      const address = server.address();
      const port = typeof address === 'object' && address ? address.port : 0;
      server.close(() => resolve(port));
    });
  });
}

beforeEach(() => {
  delete process.env.AIM_BASE_URL;
  delete process.env.AIM_API_KEY;
  delete process.env.AIM_AGENT_ID;
  delete process.env.AIM_PRIVATE_KEY;
  delete process.env.AIM_PUBLIC_KEY;
  delete process.env.AIM_ORGANIZATION_ID;
});

describe('network-error context (item 6)', () => {
  it('AIM-11.AC6 a refused connection with an explicit baseUrl names the URL and the errno', async () => {
    const port = await refusedLoopbackPort();
    const baseUrl = `http://127.0.0.1:${port}`;
    const client = new AIMClient({ baseUrl });

    let caught: unknown;
    try {
      await client.registerAgent({ name: 'net-context-probe' });
    } catch (err) {
      caught = err;
    }

    expect(caught).toBeInstanceOf(NetworkError);
    const message = (caught as NetworkError).message;
    expect(message).toContain(baseUrl);
    expect(message).toContain('ECONNREFUSED');
  });

  it('AIM-11.AC6 the defaulted-baseUrl hint is preserved alongside the new context', async () => {
    // No baseUrl option and no AIM_BASE_URL: the hint about the localhost
    // default must still appear (the pre-existing diagnostic must not regress).
    const client = new AIMClient();

    let caught: unknown;
    try {
      await client.registerAgent({ name: 'net-context-default-probe' });
    } catch (err) {
      caught = err;
    }

    // Either the default port answers (unlikely in CI) or we get the hinted
    // NetworkError; only assert when the connection actually failed.
    if (caught instanceof NetworkError) {
      expect(caught.message).toContain('baseUrl defaulted to');
    }
  });
});
