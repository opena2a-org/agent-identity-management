import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import * as http from 'http';
import { ARPProxy, type ARPProxyDeps } from '../../../src/arp/proxy/server';
import { EventEngine } from '../../../src/arp/engine/event-engine';
import { PromptInterceptor } from '../../../src/arp/interceptors/prompt';
import type { ARPEvent, ProxyConfig } from '../../../src/arp/types';

/**
 * Test that passthrough-mode HTTP responses are scanned through
 * the PromptInterceptor for output leak patterns (Gap 4 fix).
 */

describe('ARPProxy response body scanning', () => {
  let upstream: http.Server;
  let upstreamPort: number;
  let proxy: ARPProxy;
  let engine: EventEngine;
  let promptInterceptor: PromptInterceptor;
  let emittedEvents: ARPEvent[];

  beforeEach(async () => {
    engine = new EventEngine({ agentName: 'test-agent', rules: [] });
    promptInterceptor = new PromptInterceptor(engine);
    await promptInterceptor.start();

    emittedEvents = [];
    engine.onEvent((e) => { emittedEvents.push(e); });
  });

  afterEach(async () => {
    if (proxy) await proxy.stop().catch(() => {});
    if (upstream) await new Promise<void>((resolve) => upstream.close(() => resolve()));
  });

  function createUpstreamServer(responseBody: string): Promise<number> {
    return new Promise((resolve) => {
      upstream = http.createServer((_req, res) => {
        res.writeHead(200, { 'content-type': 'application/json' });
        res.end(responseBody);
      });
      upstream.listen(0, () => {
        const addr = upstream.address();
        upstreamPort = typeof addr === 'object' && addr ? addr.port : 0;
        resolve(upstreamPort);
      });
    });
  }

  async function createProxy(port: number): Promise<ARPProxy> {
    const config: ProxyConfig = {
      port: 0, // Let OS assign
      upstreams: [
        {
          pathPrefix: '/',
          target: `http://127.0.0.1:${port}`,
          protocol: 'passthrough',
        },
      ],
    };
    const deps: ARPProxyDeps = {
      engine,
      promptInterceptor,
    };
    proxy = new ARPProxy(config, deps);
    await proxy.start();
    return proxy;
  }

  function makeRequest(proxyPort: number, path: string): Promise<{ statusCode: number; body: string }> {
    return new Promise((resolve, reject) => {
      const req = http.get(`http://127.0.0.1:${proxyPort}${path}`, (res) => {
        const chunks: Buffer[] = [];
        res.on('data', (chunk) => chunks.push(chunk));
        res.on('end', () => {
          resolve({
            statusCode: res.statusCode ?? 0,
            body: Buffer.concat(chunks).toString('utf-8'),
          });
        });
      });
      req.on('error', reject);
    });
  }

  it('detects API key leak in passthrough response body', async () => {
    const leakyResponse = JSON.stringify({
      data: 'Here is the key: sk-proj-abc123def456ghi789jkl012mno',
    });

    const port = await createUpstreamServer(leakyResponse);
    await createProxy(port);

    const result = await makeRequest(proxy.getPort(), '/api/data');

    expect(result.statusCode).toBe(200);

    // The prompt interceptor should have detected the API key in the response
    const outputLeakEvents = emittedEvents.filter(
      (e) => e.source === 'prompt' && e.data.direction === 'output'
    );
    expect(outputLeakEvents.length).toBeGreaterThan(0);
    expect(outputLeakEvents[0].data.patternId).toBe('OL-001');
  });

  it('passes clean passthrough responses without detection', async () => {
    const cleanResponse = JSON.stringify({
      message: 'Hello, world!',
      status: 'ok',
    });

    const port = await createUpstreamServer(cleanResponse);
    await createProxy(port);

    const result = await makeRequest(proxy.getPort(), '/api/data');

    expect(result.statusCode).toBe(200);

    const outputLeakEvents = emittedEvents.filter(
      (e) => e.source === 'prompt' && e.data.direction === 'output'
    );
    expect(outputLeakEvents).toHaveLength(0);
  });

  it('skips scanning when response body exceeds 100KB', async () => {
    // Create a response that contains an API key but is >100KB
    const padding = 'x'.repeat(110 * 1024);
    const largeResponse = `${padding} sk-proj-abc123def456ghi789jkl012mno`;

    const port = await createUpstreamServer(largeResponse);
    await createProxy(port);

    const result = await makeRequest(proxy.getPort(), '/api/data');

    expect(result.statusCode).toBe(200);

    // Should NOT detect anything because body exceeds 100KB
    const outputLeakEvents = emittedEvents.filter(
      (e) => e.source === 'prompt' && e.data.direction === 'output'
    );
    expect(outputLeakEvents).toHaveLength(0);
  });

  it('detects PII leak in passthrough response body', async () => {
    const piiResponse = JSON.stringify({
      result: 'User SSN: 123-45-6789',
    });

    const port = await createUpstreamServer(piiResponse);
    await createProxy(port);

    await makeRequest(proxy.getPort(), '/api/user');

    const outputLeakEvents = emittedEvents.filter(
      (e) => e.source === 'prompt' && e.data.direction === 'output'
    );
    expect(outputLeakEvents.length).toBeGreaterThan(0);
    expect(outputLeakEvents[0].data.patternId).toBe('OL-002');
  });
});
