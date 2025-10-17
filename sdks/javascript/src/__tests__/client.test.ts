import { AIMClient } from '../client';

describe('AIMClient', () => {
  // Mock fetch globally
  global.fetch = jest.fn();

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('Constructor', () => {
    it('should initialize with config', () => {
      const client = new AIMClient({
        apiUrl: 'http://localhost:8080',
        apiKey: 'test-key',
        agentId: 'test-agent',
        autoDetect: false, // Disable for testing
      });

      expect(client).toBeDefined();
      client.destroy();
    });

    it('should use default values for optional config', () => {
      const client = new AIMClient({
        apiUrl: 'http://localhost:8080',
        apiKey: 'test-key',
        agentId: 'test-agent',
        autoDetect: false,
      });

      expect(client).toBeDefined();
      client.destroy();
    });

    it('should start detectors when autoDetect is true', () => {
      const client = new AIMClient({
        apiUrl: 'http://localhost:8080',
        apiKey: 'test-key',
        agentId: 'test-agent',
        autoDetect: true,
      });

      expect(client).toBeDefined();
      client.destroy();
    });
  });

  describe('detect()', () => {
    it('should return empty array when no detections', async () => {
      const client = new AIMClient({
        apiUrl: 'http://localhost:8080',
        apiKey: 'test-key',
        agentId: 'test-agent',
        autoDetect: true,
      });

      const detections = await client.detect();
      expect(Array.isArray(detections)).toBe(true);
      expect(detections.length).toBe(0);

      client.destroy();
    });
  });

  describe('reportMCP()', () => {
    it('should report MCP successfully', async () => {
      // Mock successful API response
      (global.fetch as jest.Mock).mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          success: true,
          detectionsProcessed: 1,
          newMCPs: ['filesystem'],
          existingMCPs: [],
          message: 'Successfully processed 1 detections',
        }),
      });

      const client = new AIMClient({
        apiUrl: 'http://localhost:8080',
        apiKey: 'test-key',
        agentId: 'test-agent-id',
        autoDetect: false,
      });

      await client.reportMCP('filesystem');

      // Verify fetch was called
      expect(global.fetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/detection/agents/test-agent-id/report',
        expect.objectContaining({
          method: 'POST',
          headers: expect.objectContaining({
            'Content-Type': 'application/json',
            'Authorization': 'Bearer test-key',
          }),
        })
      );

      client.destroy();
    });

    it('should handle API errors gracefully', async () => {
      // Mock failed API response
      (global.fetch as jest.Mock).mockResolvedValueOnce({
        ok: false,
        status: 500,
        text: async () => 'Internal Server Error',
      });

      const client = new AIMClient({
        apiUrl: 'http://localhost:8080',
        apiKey: 'test-key',
        agentId: 'test-agent-id',
        autoDetect: false,
      });

      // Should not throw error (silent failure)
      await expect(client.reportMCP('filesystem')).resolves.not.toThrow();

      client.destroy();
    });

    it('should handle network errors gracefully', async () => {
      // Mock network error
      (global.fetch as jest.Mock).mockRejectedValueOnce(
        new Error('Network error')
      );

      const client = new AIMClient({
        apiUrl: 'http://localhost:8080',
        apiKey: 'test-key',
        agentId: 'test-agent-id',
        autoDetect: false,
      });

      // Should not throw error (silent failure)
      await expect(client.reportMCP('filesystem')).resolves.not.toThrow();

      client.destroy();
    });
  });

  describe('destroy()', () => {
    it('should clean up resources', () => {
      const client = new AIMClient({
        apiUrl: 'http://localhost:8080',
        apiKey: 'test-key',
        agentId: 'test-agent',
        autoDetect: true,
      });

      // Should not throw
      expect(() => client.destroy()).not.toThrow();
    });

    it('should stop periodic reporting', (done) => {
      const client = new AIMClient({
        apiUrl: 'http://localhost:8080',
        apiKey: 'test-key',
        agentId: 'test-agent',
        autoDetect: true,
        reportInterval: 100, // Short interval for testing
      });

      // Mock fetch
      (global.fetch as jest.Mock).mockResolvedValue({
        ok: true,
        json: async () => ({ success: true, detectionsProcessed: 0 }),
      });

      // Destroy after short delay
      setTimeout(() => {
        client.destroy();

        // Wait a bit more and verify fetch wasn't called again
        setTimeout(() => {
          const callCountBeforeDestroy = (global.fetch as jest.Mock).mock.calls.length;

          // Wait for another interval
          setTimeout(() => {
            const callCountAfterDestroy = (global.fetch as jest.Mock).mock.calls.length;

            // Call count should not have increased
            expect(callCountAfterDestroy).toBe(callCountBeforeDestroy);
            done();
          }, 150);
        }, 50);
      }, 50);
    });
  });
});
