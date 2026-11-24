import { describe, it, expect, beforeEach, vi } from 'vitest';
import { MCPProtocolInterceptor } from '../../../src/arp/interceptors/mcp-protocol';
import { EventEngine } from '../../../src/arp/engine/event-engine';
import { PATTERN_SETS, MAX_SCAN_TOTAL, SCAN_WINDOW_LENGTH } from '../../../src/arp/patterns/ai-threats';
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

  describe('each argument scanned on its own', () => {
    it('scans an argument after one longer than 65,536 characters', async () => {
      const interceptor = await createInterceptor();
      const result = interceptor.scanToolCall('writeFile', {
        content: 'x'.repeat(SCAN_WINDOW_LENGTH + 1),
        path: '../../etc/passwd',
      });
      expect(result.detected).toBe(true);
      expect(result.truncated).toBe(false);
      expect(result.matches).toHaveLength(1);
      expect(result.matches[0].argument).toBe('path');
      expect(emittedEvents).toHaveLength(1);
      expect(emittedEvents[0].data.patternId).toBe('MCP-001');
      expect(emittedEvents[0].data.argument).toBe('path');
    });

    it('scans an argument after one longer than the scan total', async () => {
      const interceptor = await createInterceptor();
      const result = interceptor.scanToolCall('writeFile', {
        content: 'x'.repeat(MAX_SCAN_TOTAL + 1),
        path: '../../etc/passwd',
      });
      expect(result.detected).toBe(true);
      expect(result.truncated).toBe(true);
      expect(emittedEvents.map((e) => e.data.argument)).toEqual(['path']);
    });

    it('detects command injection past 65,536 characters within one argument', async () => {
      const interceptor = await createInterceptor();
      const result = interceptor.scanToolCall('execute', {
        command: 'echo ' + 'x'.repeat(SCAN_WINDOW_LENGTH * 2) + '; cat /etc/passwd',
      });
      expect(result.detected).toBe(true);
      expect(result.truncated).toBe(false);
      expect(emittedEvents[0].data.patternId).toBe('MCP-002');
      expect(emittedEvents[0].data.argument).toBe('command');
    });

    it('fires the event for a match within the total of an argument longer than the total', async () => {
      const interceptor = await createInterceptor();
      const result = interceptor.scanToolCall('execute', {
        command: 'x'.repeat(SCAN_WINDOW_LENGTH) + '; cat /etc/passwd ' + 'x'.repeat(MAX_SCAN_TOTAL),
      });
      expect(result.detected).toBe(true);
      expect(result.truncated).toBe(true);
      expect(emittedEvents).toHaveLength(1);
      expect(emittedEvents[0].data.patternId).toBe('MCP-002');
    });

    it('does not join two arguments into one match', async () => {
      const interceptor = await createInterceptor();
      // Joined, the text between the two backticks would match MCP-002 as a shell command substitution.
      const result = interceptor.scanToolCall('note', { title: 'Ann`s notes', body: 'Bob`s notes' });
      expect(result.detected).toBe(false);
      expect(emittedEvents).toHaveLength(0);
    });

    it('reports a pattern once for each argument it matches in', async () => {
      const interceptor = await createInterceptor();
      const result = interceptor.scanToolCall('copy', { from: '../a', to: '../b' });
      expect(result.matches.map((m) => [m.pattern.id, m.argument])).toEqual([
        ['MCP-001', 'from'],
        ['MCP-001', 'to'],
      ]);
      expect(emittedEvents).toHaveLength(2);
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

    it('a scan before start() prints one line naming start(), once per instance', () => {
      const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
      try {
        const idle = new MCPProtocolInterceptor(engine);
        expect(idle.scanToolCall('execute', { command: 'ls' }).scanned).toBe(false);
        expect(idle.scanToolCall('execute', { command: 'ls' }).scanned).toBe(false);
        expect(warn).toHaveBeenCalledTimes(1);
        expect(warn.mock.calls[0][0]).toBe(
          '[ARP] MCPProtocolInterceptor.scanToolCall() was called while it is not started, so nothing was scanned ' +
            '(scanned: false). Call await start() on the MCPProtocolInterceptor first.',
        );
      } finally {
        warn.mockRestore();
      }
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
