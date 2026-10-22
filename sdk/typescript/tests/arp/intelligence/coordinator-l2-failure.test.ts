import { describe, it, expect, beforeEach, afterEach, vi, type MockInstance } from 'vitest';
import { mkdtempSync, rmSync } from 'fs';
import * as http from 'http';
import type { AddressInfo } from 'net';
import { tmpdir } from 'os';
import { join } from 'path';
import { AgentRuntimeProtection } from '../../../src/arp';
import { IntelligenceCoordinator } from '../../../src/arp/intelligence/coordinator';
import type { ARPConfig, ARPEvent, IntelligenceConfig, LLMAdapter } from '../../../src/arp/types';

/**
 * An L2 call that throws is reported once and counted in the budget status.
 *
 * With `intelligence: { enabled: true, adapter: 'ollama' }` and nothing listening at the
 * host, every L2 call was refused and the refusal was dropped: nothing was printed,
 * `getBudgetStatus()` read `totalCalls: 0`, which is also what it reads when L2 never ran,
 * and the adapter's `healthCheck()` had no caller.
 *
 * These tests use the real Ollama adapter against 127.0.0.1: a port that was bound and
 * closed for the failures, and a server that answers for the positive controls, so a
 * harness that stops reaching the adapter fails here instead of reading as a pass.
 */

const CANARY = 'CANARY' + 'EVENTCONTENT' + '5d81c2';

interface Reply {
  status: number;
  body: string;
}

interface Stub {
  port: number;
  /** `<method> <path>` of each request, in the order they arrived. */
  requests: string[];
  close(): Promise<void>;
}

/** An HTTP server on 127.0.0.1 that answers each request with `answer`, or never answers when it returns null. */
async function serve(answer: (method: string, url: string) => Reply | null): Promise<Stub> {
  const requests: string[] = [];
  const server = http.createServer((req, res) => {
    requests.push(`${req.method} ${req.url}`);
    req.resume();
    req.on('end', () => {
      const reply = answer(req.method ?? '', req.url ?? '');
      if (reply === null) return;
      res.writeHead(reply.status, { 'Content-Type': 'application/json' });
      res.end(reply.body);
    });
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const { port } = server.address() as AddressInfo;
  return {
    port,
    requests,
    close: () =>
      new Promise<void>((resolve) => {
        server.closeAllConnections();
        server.close(() => resolve());
      }),
  };
}

/**
 * A port on 127.0.0.1 with nothing listening on it: bound to port 0, then closed.
 *
 * There is a small race: another process could bind the same port between the close and
 * the calls a test makes. A port closed a moment ago is not normally handed out again that
 * soon, and if it were, the calls would reach that listener and the assertions on
 * ECONNREFUSED below would fail rather than pass.
 */
async function closedPort(): Promise<number> {
  const stub = await serve(() => ({ status: 404, body: '{}' }));
  await stub.close();
  return stub.port;
}

const ALLOW: Reply = { status: 200, body: JSON.stringify({ response: 'ACTION: ALLOW' }) };
const NOT_FOUND: Reply = { status: 404, body: '{}' };

function config(dataDir: string, intelligence: IntelligenceConfig): ARPConfig {
  return {
    agentName: 'l2-failure-test',
    dataDir,
    monitors: { process: { enabled: false }, network: { enabled: false }, filesystem: { enabled: false } },
    // The runtime twin writes under the home directory; nothing here needs it.
    intelligence: { enabled: true, adapter: 'ollama', runtimeTwin: { enabled: false }, ...intelligence },
  };
}

let sequence = 0;

/** A process violation of severity high, which L2 assesses. */
function violation(): ARPEvent {
  sequence += 1;
  return {
    id: `l2-failure-${sequence}`,
    timestamp: new Date().toISOString(),
    source: 'process',
    category: 'violation',
    severity: 'high',
    description: `Unexpected child process ${CANARY}`,
    data: { pid: 4242, command: CANARY },
    classifiedBy: 'L0-rules',
  };
}

describe('a failed L2 call', () => {
  let dataDir: string;
  let warn: MockInstance<Parameters<typeof console.warn>, void>;
  const stubs: Stub[] = [];

  /** The lines printed with console.warn, in order. */
  function warnings(): string[] {
    return warn.mock.calls.map((call) => String(call[0]));
  }

  beforeEach(() => {
    dataDir = mkdtempSync(join(tmpdir(), 'arp-l2-unreachable-'));
    warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined);
  });

  afterEach(async () => {
    warn.mockRestore();
    await Promise.all(stubs.splice(0).map((stub) => stub.close()));
    rmSync(dataDir, { recursive: true, force: true });
  });

  describe('IntelligenceCoordinator', () => {
    it('reports a refused call once, naming the destination and the error, and counts each failed call', async () => {
      const port = await closedPort();
      const coord = new IntelligenceCoordinator(
        config(dataDir, { adapterConfig: { host: `http://127.0.0.1:${port}` }, enableBatching: false }),
        dataDir,
      );
      const events = [violation(), violation()];

      for (const event of events) {
        expect(await coord.analyze(event)).toBeNull();
      }

      expect(warn).toHaveBeenCalledTimes(1);
      const [line] = warnings();
      expect(line).toContain('L2 call failed');
      expect(line).toContain(`127.0.0.1:${port}`);
      expect(line).toBe(
        `[ARP] L2 call failed (ollama, http://127.0.0.1:${port}): ECONNREFUSED. ` +
          'L0 and L1 are unaffected and still gate. ' +
          `Check that an Ollama server is listening there: curl -sS http://127.0.0.1:${port}/api/tags`,
      );

      const status = coord.getBudgetStatus();
      expect(status.failedCalls).toBe(2);
      expect(status.totalCalls).toBe(0);
      expect(status.lastFailure?.adapter).toBe('ollama');
      expect(status.lastFailure?.message).toContain('ECONNREFUSED');
      expect(Number.isNaN(Date.parse(status.lastFailure?.at ?? ''))).toBe(false);
      expect(JSON.stringify(status)).not.toContain('CANARY');

      for (const event of events) {
        expect(event.classifiedBy).toBe('L0-rules');
        expect(event.severity).toBe('high');
        expect(event.category).toBe('violation');
        expect(event.llmAssessment).toBeUndefined();
      }
    });

    it('positive control: an answered call is counted in totalCalls and prints nothing', async () => {
      const stub = await serve((_method, url) => (url === '/api/generate' ? ALLOW : NOT_FOUND));
      stubs.push(stub);
      const coord = new IntelligenceCoordinator(
        config(dataDir, { adapterConfig: { host: `http://127.0.0.1:${stub.port}` }, enableBatching: false }),
        dataDir,
      );
      const event = violation();

      const assessment = await coord.analyze(event);

      expect(stub.requests).toEqual(['POST /api/generate']);
      expect(assessment?.recommendation).toBe('allow');
      expect(event.classifiedBy).toBe('L2-llm');
      expect(warn).not.toHaveBeenCalled();
      const status = coord.getBudgetStatus();
      expect(status.failedCalls).toBe(0);
      expect(status.totalCalls).toBe(1);
      expect(status.lastFailure).toBeNull();
    });

    it('counts a batched call that fails when stop() sends it, and reports it once', async () => {
      const port = await closedPort();
      const coord = new IntelligenceCoordinator(
        config(dataDir, { adapterConfig: { host: `http://127.0.0.1:${port}` }, enableBatching: true }),
        dataDir,
      );
      const event = violation();

      expect(await coord.analyze(event)).toBeNull();
      // Queued for the batch, not sent yet.
      expect(coord.getBudgetStatus().failedCalls).toBe(0);
      expect(warn).not.toHaveBeenCalled();

      await coord.stop();

      expect(warnings()).toEqual([
        `[ARP] L2 call failed (ollama, http://127.0.0.1:${port}): ECONNREFUSED. ` +
          'L0 and L1 are unaffected and still gate. ' +
          `Check that an Ollama server is listening there: curl -sS http://127.0.0.1:${port}/api/tags`,
      ]);
      const status = coord.getBudgetStatus();
      expect(status.failedCalls).toBe(1);
      expect(status.totalCalls).toBe(0);
      expect(status.lastFailure?.message).toBe('ECONNREFUSED');
      expect(event.classifiedBy).toBe('L0-rules');
    });

    it('a call that answers after a failure re-arms the line, so the next failure is reported again', async () => {
      let failing = true;
      const stub = await serve((_method, url) => {
        if (url !== '/api/generate') return NOT_FOUND;
        return failing ? { status: 500, body: '{"error":"model runner stopped"}' } : ALLOW;
      });
      stubs.push(stub);
      const coord = new IntelligenceCoordinator(
        config(dataDir, { adapterConfig: { host: `http://127.0.0.1:${stub.port}` }, enableBatching: false }),
        dataDir,
      );

      await coord.analyze(violation());
      await coord.analyze(violation());
      expect(warn).toHaveBeenCalledTimes(1);

      failing = false;
      await coord.analyze(violation());
      expect(warn).toHaveBeenCalledTimes(1);

      failing = true;
      await coord.analyze(violation());

      expect(stub.requests).toHaveLength(4);
      expect(warnings()).toHaveLength(2);
      for (const line of warnings()) {
        expect(line).toContain(`[ARP] L2 call failed (ollama, http://127.0.0.1:${stub.port}): HTTP 500. `);
      }
      const status = coord.getBudgetStatus();
      expect(status.failedCalls).toBe(3);
      expect(status.totalCalls).toBe(1);
      expect(status.lastFailure?.message).toBe('HTTP 500');
    });

    it.each([
      {
        form: 'an HTTP error whose body quotes the event',
        reply: { status: 500, body: JSON.stringify({ error: `${CANARY} is not allowed` }) },
        message: 'HTTP 500',
      },
      {
        form: 'an answer that is not JSON and quotes the event',
        reply: { status: 200, body: `${CANARY} was the request` },
        message: 'the answer was not valid JSON',
      },
    ])('prints and records none of what the server answered: $form', async ({ reply, message }) => {
      const stub = await serve((_method, url) => (url === '/api/generate' ? reply : NOT_FOUND));
      stubs.push(stub);
      const coord = new IntelligenceCoordinator(
        config(dataDir, { adapterConfig: { host: `http://127.0.0.1:${stub.port}` }, enableBatching: false }),
        dataDir,
      );

      await coord.analyze(violation());

      expect(stub.requests).toEqual(['POST /api/generate']);
      expect(warnings()).toEqual([
        `[ARP] L2 call failed (ollama, http://127.0.0.1:${stub.port}): ${message}. ` +
          'L0 and L1 are unaffected and still gate. ' +
          `Check that an Ollama server is listening there: curl -sS http://127.0.0.1:${stub.port}/api/tags`,
      ]);
      const status = coord.getBudgetStatus();
      expect(status.lastFailure?.message).toBe(message);
      expect(JSON.stringify(status)).not.toContain('CANARY');
    });

    it('names a configured host that is not a URL instead of printing a command', async () => {
      const coord = new IntelligenceCoordinator(
        config(dataDir, { adapterConfig: { host: 'not a url' }, enableBatching: false }),
        dataDir,
      );

      await coord.analyze(violation());

      expect(warnings()).toEqual([
        '[ARP] L2 call failed (ollama, endpoint not reported by the adapter): ERR_INVALID_URL. ' +
          'L0 and L1 are unaffected and still gate. ' +
          'Check that intelligence.adapterConfig.host is the URL of the Ollama server.',
      ]);
      expect(coord.getBudgetStatus().failedCalls).toBe(1);
    });

    it('prints no command for a host that a shell would not hand to curl as written', async () => {
      const coord = new IntelligenceCoordinator(
        config(dataDir, { adapterConfig: { host: 'http://ollama$(id).internal:11434' }, enableBatching: false }),
        dataDir,
      );
      // The adapter is private. Its health check is stubbed so that nothing is looked up or dialed.
      const adapter = (coord as unknown as { adapter: LLMAdapter | null }).adapter;
      if (!adapter) throw new Error('IntelligenceCoordinator no longer keeps its adapter in adapter');
      vi.spyOn(adapter, 'healthCheck').mockResolvedValue(false);

      await coord.checkL2Health();

      expect(warnings()).toEqual([
        '[ARP] L2 call failed (ollama, http://ollama$(id).internal:11434): health check failed. ' +
          'L0 and L1 are unaffected and still gate. ' +
          'Check that intelligence.adapterConfig.host is the URL of the Ollama server.',
      ]);
    });

    it('checkL2Health names the key and the endpoint for a vendor adapter without a key', async () => {
      // The vendor adapters' health check reads the key and sends nothing; no call is made here.
      const coord = new IntelligenceCoordinator(
        config(dataDir, { adapter: 'anthropic', adapterConfig: { apiKey: '' }, enableBatching: false }),
        dataDir,
      );

      await coord.checkL2Health();

      expect(warnings()).toEqual([
        '[ARP] L2 call failed (anthropic, https://api.anthropic.com): health check failed. ' +
          'L0 and L1 are unaffected and still gate. ' +
          'Check that ANTHROPIC_API_KEY or intelligence.adapterConfig.apiKey holds a valid key, ' +
          'and that this host reaches the endpoint: curl -sSI https://api.anthropic.com',
      ]);
      expect(coord.getBudgetStatus().failedCalls).toBe(0);
    });

    it('checkL2Health reports a destination that is not listening with the same line, and counts no call', async () => {
      const port = await closedPort();
      const coord = new IntelligenceCoordinator(
        config(dataDir, { adapterConfig: { host: `http://127.0.0.1:${port}` }, enableBatching: false }),
        dataDir,
      );

      await coord.checkL2Health();

      expect(warnings()).toEqual([
        `[ARP] L2 call failed (ollama, http://127.0.0.1:${port}): health check failed. ` +
          'L0 and L1 are unaffected and still gate. ' +
          `Check that an Ollama server is listening there: curl -sS http://127.0.0.1:${port}/api/tags`,
      ]);
      expect(coord.getBudgetStatus().failedCalls).toBe(0);

      // The call that fails next is counted, and the failure is not printed a second time.
      await coord.analyze(violation());
      expect(warn).toHaveBeenCalledTimes(1);
      expect(coord.getBudgetStatus().failedCalls).toBe(1);
    });

    it('checkL2Health gives up on a destination that does not answer within its timeout', async () => {
      const stub = await serve(() => null);
      stubs.push(stub);
      const coord = new IntelligenceCoordinator(
        config(dataDir, { adapterConfig: { host: `http://127.0.0.1:${stub.port}` }, enableBatching: false }),
        dataDir,
      );

      const started = Date.now();
      await coord.checkL2Health(100);

      expect(Date.now() - started).toBeLessThan(2000);
      expect(stub.requests).toEqual(['GET /api/tags']);
      expect(warnings()).toHaveLength(1);
      expect(warnings()[0]).toContain(
        `[ARP] L2 call failed (ollama, http://127.0.0.1:${stub.port}): health check did not answer within 100 ms. `,
      );
    });

    it('checkL2Health prints nothing for a destination that answers, and asks it once', async () => {
      const stub = await serve((_method, url) => (url === '/api/tags' ? { status: 200, body: '{"models":[]}' } : NOT_FOUND));
      stubs.push(stub);
      const coord = new IntelligenceCoordinator(
        config(dataDir, { adapterConfig: { host: `http://127.0.0.1:${stub.port}` }, enableBatching: false }),
        dataDir,
      );

      await coord.checkL2Health();
      await coord.checkL2Health();

      expect(stub.requests).toEqual(['GET /api/tags']);
      expect(warn).not.toHaveBeenCalled();
    });

    it('checkL2Health asks nothing and prints nothing when L2 is off', async () => {
      const stub = await serve(() => NOT_FOUND);
      stubs.push(stub);
      const coord = new IntelligenceCoordinator(
        config(dataDir, { enabled: false, adapterConfig: { host: `http://127.0.0.1:${stub.port}` } }),
        dataDir,
      );

      await coord.checkL2Health();
      await coord.analyze(violation());

      expect(stub.requests).toEqual([]);
      expect(warn).not.toHaveBeenCalled();
      expect(coord.getBudgetStatus().failedCalls).toBe(0);
    });
  });

  describe('AgentRuntimeProtection', () => {
    it('start() reports an L2 destination that is not listening before the first event', async () => {
      const port = await closedPort();
      const arp = new AgentRuntimeProtection(
        config(dataDir, { adapterConfig: { host: `http://127.0.0.1:${port}` }, enableBatching: false }),
      );

      await arp.start();

      expect(warnings()).toEqual([
        `[ARP] L2 call failed (ollama, http://127.0.0.1:${port}): health check failed. ` +
          'L0 and L1 are unaffected and still gate. ' +
          `Check that an Ollama server is listening there: curl -sS http://127.0.0.1:${port}/api/tags`,
      ]);
      expect(arp.getStatus().budget.failedCalls).toBe(0);
      await arp.stop();
    });

    it('getStatus().budget carries failedCalls and lastFailure as the coordinator reports them', async () => {
      const port = await closedPort();
      const arp = new AgentRuntimeProtection(
        config(dataDir, { adapterConfig: { host: `http://127.0.0.1:${port}` }, enableBatching: false }),
      );
      await arp.start();

      const event = await arp.getEngine().emit({
        source: 'process',
        category: 'violation',
        severity: 'high',
        description: 'Unexpected child process',
        data: { pid: 4242 },
      });

      const budget = arp.getStatus().budget;
      expect(budget.failedCalls).toBe(1);
      expect(budget.totalCalls).toBe(0);
      expect(budget.lastFailure?.message).toBe('ECONNREFUSED');
      expect(budget).toEqual(arp.getIntelligence().getBudgetStatus());
      expect(event.classifiedBy).toBe('L0-rules');
      // The health check at start() printed the line; the failed call does not print it again.
      expect(warn).toHaveBeenCalledTimes(1);
      await arp.stop();
    });

    it('start() asks no destination and prints nothing when L2 is off', async () => {
      const stub = await serve(() => NOT_FOUND);
      stubs.push(stub);
      const arp = new AgentRuntimeProtection(
        config(dataDir, { enabled: false, adapterConfig: { host: `http://127.0.0.1:${stub.port}` } }),
      );

      await arp.start();
      await arp.stop();

      expect(stub.requests).toEqual([]);
      expect(warn).not.toHaveBeenCalled();
      expect(arp.getStatus().budget.failedCalls).toBe(0);
    });
  });
});
