import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import * as http from 'http';
import * as net from 'net';
import { ARPProxy, DEFAULT_PROXY_HOST } from '../../../src/arp/proxy/server';
import { EventEngine } from '../../../src/arp/engine/event-engine';
import type { ProxyConfig } from '../../../src/arp/types';

/**
 * The address ARPProxy.start() listens on (#853).
 *
 * With no `host` in its config the proxy listens on 127.0.0.1, and a host the
 * config names is passed to `listen` as given. The first group records the
 * arguments of `listen` with a spy that binds no socket. The second starts
 * real proxies, on loopback addresses only, and reads the address the server
 * reports.
 */

function newProxy(config: Partial<ProxyConfig> = {}): ARPProxy {
  const engine = new EventEngine({ agentName: 'test-agent', rules: [] });
  return new ARPProxy({ port: 0, upstreams: [], ...config }, { engine });
}

function listeningAddress(proxy: ARPProxy): net.AddressInfo {
  const server = (proxy as unknown as { server: http.Server | null }).server;
  const addr = server?.address();
  if (!addr || typeof addr !== 'object') {
    throw new Error('the proxy is not listening on a TCP address');
  }
  return addr;
}

function canBind(host: string): Promise<boolean> {
  return new Promise((resolve) => {
    const probe = net.createServer();
    probe.once('error', () => resolve(false));
    probe.listen(0, host, () => {
      probe.close(() => resolve(true));
    });
  });
}

describe('ARPProxy.start() listen arguments (spy, no socket bound)', () => {
  const proto = http.Server.prototype as unknown as Record<string, unknown>;
  const hadOwnListen = Object.prototype.hasOwnProperty.call(proto, 'listen');
  const originalListen = proto.listen;
  let calls: unknown[][];

  beforeEach(() => {
    calls = [];
    proto.listen = function (this: http.Server, ...args: unknown[]): http.Server {
      calls.push(args);
      const callback = args.find((a) => typeof a === 'function') as
        | (() => void)
        | undefined;
      callback?.();
      return this;
    };
  });

  afterEach(() => {
    if (hadOwnListen) {
      proto.listen = originalListen;
    } else {
      delete proto.listen;
    }
  });

  it('passes 127.0.0.1 as the host when the config sets no host', async () => {
    await newProxy().start();

    expect(calls).toHaveLength(1);
    expect(calls[0][0]).toBe(0);
    expect(calls[0][1]).toBe('127.0.0.1');
  });

  it('passes 127.0.0.1 as the host when the config sets an empty host', async () => {
    await newProxy({ host: '' }).start();

    expect(calls).toHaveLength(1);
    expect(calls[0][1]).toBe('127.0.0.1');
  });

  it('passes the host the config names', async () => {
    await newProxy({ host: '0.0.0.0' }).start();

    expect(calls).toHaveLength(1);
    expect(calls[0][1]).toBe('0.0.0.0');
  });
});

describe('ARPProxy listening address (loopback sockets)', () => {
  let proxy: ARPProxy | undefined;

  afterEach(async () => {
    if (proxy) {
      await proxy.stop();
      proxy = undefined;
    }
  });

  it('exports 127.0.0.1 as the default host', () => {
    expect(DEFAULT_PROXY_HOST).toBe('127.0.0.1');
  });

  it('listens on 127.0.0.1 when the config sets no host', async () => {
    proxy = newProxy();
    await proxy.start();

    const addr = listeningAddress(proxy);
    expect(addr.address).toBe('127.0.0.1');
    expect(addr.family).toBe('IPv4');
    expect(proxy.getPort()).toBe(addr.port);
  });

  it('listens on ::1 when the config sets host ::1', async (ctx) => {
    if (!(await canBind('::1'))) {
      // The machine has no IPv6 loopback address to bind.
      ctx.skip();
    }
    proxy = newProxy({ host: '::1' });
    await proxy.start();

    const addr = listeningAddress(proxy);
    expect(addr.address).toBe('::1');
    expect(addr.family).toBe('IPv6');
  });
});
