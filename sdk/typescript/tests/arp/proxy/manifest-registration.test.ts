import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import * as http from 'http';
import * as path from 'path';
import { ARPProxy, type ARPProxyDeps } from '../../../src/arp/proxy/server';
import { EventEngine } from '../../../src/arp/engine/event-engine';
import type { ProxyConfig, ManifestRejectionLog } from '../../../src/arp/types';

/**
 * Integration test for the capability manifest gate on ARPProxy.start().
 *
 * A manifestPath that points at an invalid (or missing) YAML must transition
 * the proxy into a deny-all state, 403 every incoming request with the
 * generic client-facing reason, and surface a structured rejection log via
 * the onManifestRejection hook. The rejection log contains the discrete
 * loader error code; the HTTP response does not leak it.
 *
 * A manifestPath that points at a valid, signed, unexpired fixture must let
 * traffic through unchanged.
 */

const FIXTURE_DIR = path.join(
  __dirname,
  '..',
  '..',
  '..',
  'test',
  'fixtures',
  'capability-manifests',
);

describe('ARPProxy capability manifest gate', () => {
  let upstream: http.Server | undefined;
  let upstreamPort = 0;
  let proxy: ARPProxy | undefined;
  let engine: EventEngine;

  beforeEach(() => {
    engine = new EventEngine({ agentName: 'test-agent', rules: [] });
  });

  afterEach(async () => {
    if (proxy) {
      await proxy.stop().catch(() => {});
      proxy = undefined;
    }
    if (upstream) {
      await new Promise<void>((resolve) => upstream!.close(() => resolve()));
      upstream = undefined;
    }
  });

  function createUpstreamServer(responseBody: string): Promise<number> {
    return new Promise((resolve) => {
      upstream = http.createServer((_req, res) => {
        res.writeHead(200, { 'content-type': 'application/json' });
        res.end(responseBody);
      });
      upstream.listen(0, () => {
        const addr = upstream!.address();
        upstreamPort = typeof addr === 'object' && addr ? addr.port : 0;
        resolve(upstreamPort);
      });
    });
  }

  function buildProxy(
    manifestPath: string | undefined,
    port: number,
    onRejection?: (entry: ManifestRejectionLog) => void,
  ): ARPProxy {
    const config: ProxyConfig = {
      port: 0,
      upstreams: [
        {
          pathPrefix: '/',
          target: `http://127.0.0.1:${port}`,
          protocol: 'passthrough',
        },
      ],
      ...(manifestPath ? { manifestPath } : {}),
    };
    const deps: ARPProxyDeps = {
      engine,
      ...(onRejection ? { onManifestRejection: onRejection } : {}),
    };
    return new ARPProxy(config, deps);
  }

  function makeRequest(
    proxyPort: number,
    reqPath: string,
  ): Promise<{ statusCode: number; body: string }> {
    return new Promise((resolve, reject) => {
      const req = http.get(
        `http://127.0.0.1:${proxyPort}${reqPath}`,
        (res) => {
          const chunks: Buffer[] = [];
          res.on('data', (chunk) => chunks.push(chunk));
          res.on('end', () => {
            resolve({
              statusCode: res.statusCode ?? 0,
              body: Buffer.concat(chunks).toString('utf-8'),
            });
          });
        },
      );
      req.on('error', reject);
    });
  }

  it('accepts traffic when the configured manifest verifies', async () => {
    const upstreamBody = JSON.stringify({ ok: true });
    const port = await createUpstreamServer(upstreamBody);

    const rejections: ManifestRejectionLog[] = [];
    proxy = buildProxy(
      path.join(FIXTURE_DIR, 'happy.yaml'),
      port,
      (entry) => rejections.push(entry),
    );
    await proxy.start();

    expect(proxy.isManifestRejected()).toBe(false);
    expect(proxy.getManifest()?.agentId).toBe('fixture-agent');
    expect(proxy.getManifest()?.tier).toBe('execute');
    expect(rejections).toHaveLength(0);

    const result = await makeRequest(proxy.getPort(), '/api/data');
    expect(result.statusCode).toBe(200);
    expect(result.body).toBe(upstreamBody);
  });

  it('fails closed to 403 and logs rejection for tampered payload', async () => {
    const port = await createUpstreamServer(JSON.stringify({ ok: true }));

    const rejections: ManifestRejectionLog[] = [];
    proxy = buildProxy(
      path.join(FIXTURE_DIR, 'tampered-payload.yaml'),
      port,
      (entry) => rejections.push(entry),
    );
    await proxy.start();

    expect(proxy.isManifestRejected()).toBe(true);
    expect(proxy.getManifest()).toBeNull();

    const result = await makeRequest(proxy.getPort(), '/api/data');
    expect(result.statusCode).toBe(403);
    const parsed = JSON.parse(result.body);
    expect(parsed.error).toBe(
      'Request blocked by ARP: agent registration denied',
    );
    // Client response must not leak the discrete loader error code.
    expect(result.body).not.toMatch(/SIGNATURE_INVALID/);
    expect(result.body).not.toMatch(/CapabilityManifestError/);

    expect(rejections).toHaveLength(1);
    expect(rejections[0].code).toBe('SIGNATURE_INVALID');
    expect(rejections[0].manifestPath).toContain('tampered-payload.yaml');
    expect(rejections[0].timestamp).toMatch(
      /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/,
    );
  });

  it('fails closed to 403 for a missing signature block', async () => {
    const port = await createUpstreamServer(JSON.stringify({ ok: true }));

    const rejections: ManifestRejectionLog[] = [];
    proxy = buildProxy(
      path.join(FIXTURE_DIR, 'missing-signature.yaml'),
      port,
      (entry) => rejections.push(entry),
    );
    await proxy.start();

    const result = await makeRequest(proxy.getPort(), '/api/data');
    expect(result.statusCode).toBe(403);
    expect(rejections[0].code).toBe('SIGNATURE_MISSING');
  });

  it('fails closed to 403 for an expired manifest', async () => {
    const port = await createUpstreamServer(JSON.stringify({ ok: true }));

    const rejections: ManifestRejectionLog[] = [];
    proxy = buildProxy(
      path.join(FIXTURE_DIR, 'expired.yaml'),
      port,
      (entry) => rejections.push(entry),
    );
    await proxy.start();

    const result = await makeRequest(proxy.getPort(), '/api/data');
    expect(result.statusCode).toBe(403);
    expect(rejections[0].code).toBe('EXPIRED');
  });

  it('fails closed to 403 for a manifest path that does not exist', async () => {
    const port = await createUpstreamServer(JSON.stringify({ ok: true }));

    const rejections: ManifestRejectionLog[] = [];
    proxy = buildProxy(
      path.join(FIXTURE_DIR, 'does-not-exist.yaml'),
      port,
      (entry) => rejections.push(entry),
    );
    await proxy.start();

    const result = await makeRequest(proxy.getPort(), '/api/data');
    expect(result.statusCode).toBe(403);
    expect(rejections[0].code).toBe('IO_ERROR');
    // The rejection log may include a loader reason string; it must not be
    // echoed back to the client response.
    expect(result.body).not.toContain('ENOENT');
  });

  it('denies every subsequent request once the proxy is in deny-all state', async () => {
    const port = await createUpstreamServer(JSON.stringify({ ok: true }));

    proxy = buildProxy(
      path.join(FIXTURE_DIR, 'wrong-algorithm.yaml'),
      port,
      () => {},
    );
    await proxy.start();

    // Exercise multiple distinct paths to confirm the short-circuit runs
    // before routing, not after a failed upstream lookup.
    for (const p of ['/a', '/b/c', '/completely/unmapped/path']) {
      const result = await makeRequest(proxy.getPort(), p);
      expect(result.statusCode).toBe(403);
    }
  });

  it('does not touch the upstream when denied', async () => {
    let upstreamHits = 0;
    upstream = http.createServer((_req, res) => {
      upstreamHits += 1;
      res.writeHead(200);
      res.end('ok');
    });
    await new Promise<void>((resolve) => {
      upstream!.listen(0, () => {
        const addr = upstream!.address();
        upstreamPort = typeof addr === 'object' && addr ? addr.port : 0;
        resolve();
      });
    });

    proxy = buildProxy(
      path.join(FIXTURE_DIR, 'tampered-payload.yaml'),
      upstreamPort,
      () => {},
    );
    await proxy.start();

    const result = await makeRequest(proxy.getPort(), '/api/data');
    expect(result.statusCode).toBe(403);
    expect(upstreamHits).toBe(0);
  });

  it('passes traffic through when no manifestPath is configured', async () => {
    const upstreamBody = JSON.stringify({ ok: true, legacy: true });
    const port = await createUpstreamServer(upstreamBody);

    proxy = buildProxy(undefined, port, () => {});
    await proxy.start();

    expect(proxy.isManifestRejected()).toBe(false);
    expect(proxy.getManifest()).toBeNull();

    const result = await makeRequest(proxy.getPort(), '/api/data');
    expect(result.statusCode).toBe(200);
    expect(result.body).toBe(upstreamBody);
  });
});
