import { describe, it, expect, beforeEach } from 'vitest';
import { MCPProtocolInterceptor } from '../../../src/arp/interceptors/mcp-protocol';
import { EventEngine } from '../../../src/arp/engine/event-engine';
import { PATTERN_SETS } from '../../../src/arp/patterns/ai-threats';
import type { ARPEvent } from '../../../src/arp/types';

describe('MCPProtocolInterceptor', () => {
  let engine: EventEngine;
  let emittedEvents: ARPEvent[];

  async function createInterceptor(allowedTools?: string[]): Promise<MCPProtocolInterceptor> {
    const interceptor = new MCPProtocolInterceptor(engine, allowedTools);
    await interceptor.start();
    return interceptor;
  }

  beforeEach(() => {
    engine = new EventEngine({ agentName: 'test-agent' });
    emittedEvents = [];
    engine.onEvent((event) => { emittedEvents.push(event); });
  });

  describe('scanToolCall', () => {
    it('detects path traversal in tool args', async () => {
      const interceptor = await createInterceptor();
      const result = interceptor.scanToolCall('readFile', {
        path: '../../etc/passwd',
      });
      expect(result.detected).toBe(true);
      expect(emittedEvents[0].source).toBe('mcp-protocol');
      expect(emittedEvents[0].data.patternId).toBe('MCP-001');
    });

    it('detects command injection in tool args', async () => {
      const interceptor = await createInterceptor();
      const result = interceptor.scanToolCall('execute', {
        command: 'ls; cat /etc/passwd',
      });
      expect(result.detected).toBe(true);
      expect(emittedEvents[0].data.patternId).toBe('MCP-002');
    });

    it('detects SSRF in tool args', async () => {
      const interceptor = await createInterceptor();
      const result = interceptor.scanToolCall('fetch', {
        url: 'http://169.254.169.254/latest/meta-data/',
      });
      expect(result.detected).toBe(true);
      expect(emittedEvents[0].data.patternId).toBe('MCP-003');
    });

    it('detects nested path traversal', async () => {
      const interceptor = await createInterceptor();
      const result = interceptor.scanToolCall('readFile', {
        config: { basePath: '/app', file: '../../../etc/shadow' },
      });
      expect(result.detected).toBe(true);
    });

    it('passes clean tool calls', async () => {
      const interceptor = await createInterceptor();
      const result = interceptor.scanToolCall('readFile', {
        path: '/home/user/documents/report.txt',
      });
      expect(result.detected).toBe(false);
      expect(emittedEvents).toHaveLength(0);
    });

    it('enforces tool allowlist', async () => {
      const interceptor = await createInterceptor(['readFile', 'writeFile']);
      const result = interceptor.scanToolCall('executeShell', {
        command: 'whoami',
      });
      expect(result.detected).toBe(true);
      expect(emittedEvents[0].data.reason).toBe('not-in-allowlist');
    });

    it('allows tools on the allowlist', async () => {
      const interceptor = await createInterceptor(['readFile', 'writeFile']);
      const result = interceptor.scanToolCall('readFile', {
        path: '/app/data/config.json',
      });
      expect(result.detected).toBe(false);
    });

    it('skips allowlist check when no allowlist configured', async () => {
      const interceptor = await createInterceptor();
      const result = interceptor.scanToolCall('anyTool', {
        arg: 'safe value',
      });
      expect(result.detected).toBe(false);
    });
  });

  describe('result scope (which patterns ran)', () => {
    const mcpIds = PATTERN_SETS.mcpPatterns.map((p) => p.id);

    it('names the mcp set and lists its pattern ids', async () => {
      const interceptor = await createInterceptor();
      const result = interceptor.scanToolCall('readFile', { path: '/home/user/report.txt' });
      expect(result.scanned).toBe(true);
      expect(result.scope).toEqual({ set: 'mcp', patternIds: mcpIds });
    });

    it('lists the allowlist check before the set when an allowlist is configured', async () => {
      const interceptor = await createInterceptor(['readFile']);
      const result = interceptor.scanToolCall('readFile', { path: '/app/data/config.json' });
      expect(result.detected).toBe(false);
      expect(result.scope).toEqual({ set: 'mcp', patternIds: ['MCP-ALLOWLIST', ...mcpIds] });
    });

    it('an allowlist refusal lists only the allowlist check, because the arguments were not scanned', async () => {
      const interceptor = await createInterceptor(['readFile']);
      const result = interceptor.scanToolCall('executeShell', { command: 'ls; cat /etc/passwd' });
      expect(result.detected).toBe(true);
      expect(result.scanned).toBe(true);
      expect(result.scope).toEqual({ set: 'mcp', patternIds: ['MCP-ALLOWLIST'] });
    });

    it('an interceptor that is not started says it did not scan', () => {
      const idle = new MCPProtocolInterceptor(engine);
      const result = idle.scanToolCall('execute', { command: 'ls; cat /etc/passwd' });
      expect(result.detected).toBe(false);
      expect(result.scanned).toBe(false);
      expect(result.scope).toEqual({ set: 'mcp', patternIds: [] });
      expect(emittedEvents).toHaveLength(0);
    });
  });

  describe('Monitor interface', () => {
    it('starts and stops correctly', async () => {
      const interceptor = new MCPProtocolInterceptor(engine);
      expect(interceptor.isRunning()).toBe(false);
      await interceptor.start();
      expect(interceptor.isRunning()).toBe(true);
      await interceptor.stop();
      expect(interceptor.isRunning()).toBe(false);
    });

    it('reports correct type', () => {
      const interceptor = new MCPProtocolInterceptor(engine);
      expect(interceptor.type).toBe('mcp-protocol');
    });
  });
});
