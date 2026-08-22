/**
 * The manual-retry curl commands are copy-pasted into a user's shell, so they
 * are a shell-injection surface and a correctness contract: the command must
 * actually deliver the signed body to the URL. Executed here for real — the
 * generated string runs through `sh -c` against a local HTTP server and the
 * received request is asserted — rather than eyeballed (release-test Binding
 * Decision 8: run the documented command, don't read it).
 *
 * The child runs ASYNC (execFile, not execFileSync): the server lives on this
 * process's event loop, and a sync child would block that loop while curl
 * waits for the server — a self-deadlock.
 */

import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { execFile, execFileSync } from 'child_process';
import { promisify } from 'util';
import { existsSync, rmSync } from 'fs';
import { createServer, type Server } from 'http';
import type { AddressInfo } from 'net';
import { manualPurgeCurl, type PurgeResult, type PurgeProofBody } from './purge';
import { manualEnrollCurl, type EnrollResult } from './enroll';

const execFileAsync = promisify(execFile);

let server: Server;
let baseUrl: string;
let received: Array<{ method: string; url: string; body: string }> = [];

function hasCurl(): boolean {
  try {
    execFileSync('curl', ['--version'], { stdio: 'ignore' });
    return true;
  } catch {
    return false;
  }
}

async function runShell(command: string): Promise<void> {
  try {
    await execFileAsync('sh', ['-c', command], { timeout: 10_000 });
  } catch {
    // curl's exit code is not the assertion; what the server received is.
  }
}

beforeAll(async () => {
  server = createServer((req, res) => {
    let body = '';
    req.on('data', (c) => (body += c));
    req.on('end', () => {
      received.push({ method: req.method ?? '', url: req.url ?? '', body });
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end('{"deleted": 0}');
    });
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const addr = server.address() as AddressInfo;
  baseUrl = `http://127.0.0.1:${addr.port}`;
});

afterAll(async () => {
  await new Promise<void>((resolve, reject) => server.close((e) => (e ? reject(e) : resolve())));
});

const PROOF: PurgeProofBody = {
  sensorId: 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee',
  signature: 'ab'.repeat(64),
  publicKey: 'cd'.repeat(32),
  signedAt: 1_755_000_000,
  nonce: 'ef'.repeat(16),
};

describe.skipIf(!hasCurl())('the manual retry curls deliver what they promise', () => {
  it('manualPurgeCurl executes and the server receives the signed body', async () => {
    received = [];
    const result: PurgeResult = {
      ok: false,
      error: 'HTTP 503',
      url: `${baseUrl}/api/v1/telemetry/signature/sensor/${PROOF.sensorId}`,
      body: PROOF,
    };
    await runShell(manualPurgeCurl(result));
    expect(received).toHaveLength(1);
    expect(received[0].method).toBe('DELETE');
    expect(received[0].url).toBe(`/api/v1/telemetry/signature/sensor/${PROOF.sensorId}`);
    expect(JSON.parse(received[0].body)).toEqual(PROOF);
  });

  it('manualEnrollCurl executes and the server receives the signed body', async () => {
    received = [];
    const result: EnrollResult = {
      ok: false,
      error: 'HTTP 503',
      url: `${baseUrl}/api/v1/telemetry/sensors/enroll`,
      body: {
        publicKey: PROOF.publicKey,
        nonce: PROOF.nonce,
        signedAt: PROOF.signedAt,
        signature: PROOF.signature,
      },
    };
    await runShell(manualEnrollCurl(result));
    expect(received).toHaveLength(1);
    expect(received[0].method).toBe('POST');
    expect(JSON.parse(received[0].body).publicKey).toBe(PROOF.publicKey);
  });

  it('a single quote in the URL cannot break out of the command', async () => {
    received = [];
    const marker = '/tmp/aim-arp-breakout';
    rmSync(marker, { force: true });
    const result: PurgeResult = {
      ok: false,
      error: 'x',
      // A hostile registry URL: quote-breakout followed by a command. With
      // naive interpolation this EXECUTES `touch`; with correct escaping it is
      // just a (failing) URL fetch.
      url: `${baseUrl}/x'; touch ${marker}; echo '`,
      body: PROOF,
    };
    await runShell(manualPurgeCurl(result));
    const escaped = !existsSync(marker);
    rmSync(marker, { force: true });
    expect(escaped).toBe(true);
  });
});
