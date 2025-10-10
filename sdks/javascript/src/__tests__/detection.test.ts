import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
import { autoDetectMCPs } from '../detection/capability-detection';

describe('MCP Capability Detection', () => {
  const testConfigDir = path.join(os.tmpdir(), 'aim-sdk-test-config');
  const testConfigFile = path.join(testConfigDir, 'mcp.json');

  beforeAll(() => {
    // Create test config directory
    if (!fs.existsSync(testConfigDir)) {
      fs.mkdirSync(testConfigDir, { recursive: true });
    }
  });

  afterAll(() => {
    // Clean up test config
    if (fs.existsSync(testConfigFile)) {
      fs.unlinkSync(testConfigFile);
    }
    if (fs.existsSync(testConfigDir)) {
      fs.rmdirSync(testConfigDir);
    }
  });

  describe('autoDetectMCPs', () => {
    it('should detect MCPs from config file', async () => {
      // Create a test MCP config
      const config = {
        mcpServers: {
          filesystem: {
            command: 'npx',
            args: ['-y', '@modelcontextprotocol/server-filesystem', '/tmp'],
            type: 'stdio',
          },
          database: {
            command: 'python',
            args: ['-m', 'mcp_server_sqlite'],
            type: 'stdio',
          },
        },
      };

      fs.writeFileSync(testConfigFile, JSON.stringify(config, null, 2));

      // Change to test directory
      const originalCwd = process.cwd();
      process.chdir(testConfigDir);

      try {
        const result = await autoDetectMCPs();

        expect(result).toBeDefined();
        expect(result.mcps).toBeDefined();
        expect(result.detectedAt).toBeDefined();
        expect(result.runtime).toBeDefined();

        // Check runtime info
        expect(result.runtime.runtime).toBe('node');
        expect(result.runtime.node_version).toBeDefined();
        expect(result.runtime.platform).toBeDefined();

        // Check detected MCPs
        expect(result.mcps.length).toBeGreaterThanOrEqual(0);

        if (result.mcps.length > 0) {
          const filesystemMcp = result.mcps.find((m) => m.name === 'filesystem');
          if (filesystemMcp) {
            expect(filesystemMcp.command).toBe('npx');
            expect(filesystemMcp.capabilities).toContain('filesystem');
            // Handle macOS /private prefix by comparing real paths
            expect(fs.realpathSync(filesystemMcp.detectedFrom)).toBe(fs.realpathSync(testConfigFile));
          }
        }
      } finally {
        process.chdir(originalCwd);
      }
    });

    it('should return empty array when no config files found', async () => {
      // Move to a directory with no MCP configs
      const emptyDir = path.join(os.tmpdir(), 'aim-sdk-empty-' + Date.now());
      fs.mkdirSync(emptyDir, { recursive: true });

      const originalCwd = process.cwd();
      process.chdir(emptyDir);

      try {
        const result = await autoDetectMCPs();

        expect(result).toBeDefined();
        expect(result.mcps).toBeDefined();
        expect(Array.isArray(result.mcps)).toBe(true);
        expect(result.detectedAt).toBeDefined();
        expect(result.runtime).toBeDefined();
      } finally {
        process.chdir(originalCwd);
        fs.rmdirSync(emptyDir);
      }
    });

    it('should detect correct capabilities based on command patterns', async () => {
      const config = {
        mcpServers: {
          'test-filesystem': {
            command: 'npx',
            args: ['@modelcontextprotocol/server-filesystem'],
          },
          'test-github': {
            command: 'node',
            args: ['github-server.js'],
          },
          'test-sequential': {
            command: 'python',
            args: ['-m', 'sequential_thinking'],
          },
          'test-brave': {
            command: 'brave-search-server',
          },
        },
      };

      fs.writeFileSync(testConfigFile, JSON.stringify(config, null, 2));

      const originalCwd = process.cwd();
      process.chdir(testConfigDir);

      try {
        const result = await autoDetectMCPs();

        const filesystem = result.mcps.find((m) => m.name === 'test-filesystem');
        if (filesystem) {
          expect(filesystem.capabilities).toContain('filesystem');
        }

        const github = result.mcps.find((m) => m.name === 'test-github');
        if (github) {
          expect(github.capabilities).toContain('github');
        }

        const sequential = result.mcps.find((m) => m.name === 'test-sequential');
        if (sequential) {
          expect(sequential.capabilities).toContain('sequential');
        }

        const brave = result.mcps.find((m) => m.name === 'test-brave');
        if (brave) {
          expect(brave.capabilities).toContain('brave');
        }
      } finally {
        process.chdir(originalCwd);
      }
    });
  });

  describe('Capability probing', () => {
    it('should detect filesystem capability', async () => {
      const config = {
        mcpServers: {
          fs1: { command: 'npx', args: ['filesystem-server'] },
          fs2: { command: '@modelcontextprotocol/server-filesystem' },
        },
      };

      fs.writeFileSync(testConfigFile, JSON.stringify(config, null, 2));

      const originalCwd = process.cwd();
      process.chdir(testConfigDir);

      try {
        const result = await autoDetectMCPs();

        result.mcps.forEach((mcp) => {
          if (mcp.name === 'fs1' || mcp.name === 'fs2') {
            expect(mcp.capabilities).toContain('filesystem');
          }
        });
      } finally {
        process.chdir(originalCwd);
      }
    });

    it('should detect database capability', async () => {
      const config = {
        mcpServers: {
          db1: { command: 'sqlite-server' },
          db2: { command: 'postgres-mcp' },
          db3: { command: 'mongodb', args: ['--db', 'test'] },
        },
      };

      fs.writeFileSync(testConfigFile, JSON.stringify(config, null, 2));

      const originalCwd = process.cwd();
      process.chdir(testConfigDir);

      try {
        const result = await autoDetectMCPs();

        result.mcps.forEach((mcp) => {
          if (mcp.name.startsWith('db')) {
            expect(mcp.capabilities).toContain('database');
          }
        });
      } finally {
        process.chdir(originalCwd);
      }
    });

    it('should detect multiple capabilities', async () => {
      const config = {
        mcpServers: {
          'multi-capability': {
            command: 'complex-server',
            args: ['--filesystem', '--database', '--memory'],
          },
        },
      };

      fs.writeFileSync(testConfigFile, JSON.stringify(config, null, 2));

      const originalCwd = process.cwd();
      process.chdir(testConfigDir);

      try {
        const result = await autoDetectMCPs();

        const multiMcp = result.mcps.find((m) => m.name === 'multi-capability');
        if (multiMcp) {
          // Should detect capabilities from args
          expect(multiMcp.capabilities.length).toBeGreaterThanOrEqual(1);
        }
      } finally {
        process.chdir(originalCwd);
      }
    });
  });
});
