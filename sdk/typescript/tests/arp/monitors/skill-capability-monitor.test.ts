import { describe, it, expect, beforeEach } from 'vitest';
import {
  parseDeclaredCapabilities,
  SkillCapabilityMonitor,
  createCapabilityMonitor,
} from '../../../src/arp/monitors/skill-capability-monitor';
import type { DeclaredCapabilities } from '../../../src/arp/monitors/skill-capability-monitor';

describe('parseDeclaredCapabilities', () => {
  it('parses capabilities from SKILL.md frontmatter', () => {
    const skillMd = `---
name: test-skill
file_access:
  - /tmp/data
  - /home/user/project
network_access:
  - api.example.com
  - cdn.example.com
tools:
  - read_file
  - search
credentials:
  - openai-api
---

# Test Skill
`;

    const caps = parseDeclaredCapabilities(skillMd);

    expect(caps.name).toBe('test-skill');
    expect(caps.fileAccess).toEqual(['/tmp/data', '/home/user/project']);
    expect(caps.networkAccess).toEqual(['api.example.com', 'cdn.example.com']);
    expect(caps.tools).toEqual(['read_file', 'search']);
    expect(caps.credentials).toEqual(['openai-api']);
  });

  it('returns empty arrays when no capabilities declared', () => {
    const skillMd = `---
name: minimal-skill
---
`;

    const caps = parseDeclaredCapabilities(skillMd);

    expect(caps.name).toBe('minimal-skill');
    expect(caps.fileAccess).toEqual([]);
    expect(caps.networkAccess).toEqual([]);
    expect(caps.tools).toEqual([]);
    expect(caps.credentials).toEqual([]);
  });

  it('handles inline list format', () => {
    const skillMd = `---
name: inline-skill
tools: [read_file, write_file]
network_access: [api.example.com]
---
`;

    const caps = parseDeclaredCapabilities(skillMd);

    expect(caps.tools).toEqual(['read_file', 'write_file']);
    expect(caps.networkAccess).toEqual(['api.example.com']);
  });

  it('handles missing frontmatter', () => {
    const skillMd = `# No frontmatter skill`;

    const caps = parseDeclaredCapabilities(skillMd);

    expect(caps.name).toBe('');
    expect(caps.fileAccess).toEqual([]);
  });

  it('supports alternate field names (filesystem, credential_scopes)', () => {
    const skillMd = `---
name: alt-skill
filesystem:
  - /var/data
credential_scopes:
  - aws-s3
---
`;

    const caps = parseDeclaredCapabilities(skillMd);

    expect(caps.fileAccess).toEqual(['/var/data']);
    expect(caps.credentials).toEqual(['aws-s3']);
  });
});

describe('SkillCapabilityMonitor', () => {
  let declared: DeclaredCapabilities;
  let monitor: SkillCapabilityMonitor;

  beforeEach(() => {
    declared = {
      name: 'test-skill',
      fileAccess: ['/tmp/allowed', '/home/user/project/*'],
      networkAccess: ['api.example.com', 'cdn.example.com'],
      tools: ['read_file', 'search'],
      credentials: ['openai-api'],
    };
    monitor = new SkillCapabilityMonitor(declared);
  });

  describe('file access tracking', () => {
    it('does not trigger violation for declared file paths', () => {
      monitor.recordFileAccess('/tmp/allowed');
      monitor.recordFileAccess('/home/user/project/src/index.ts');

      const violations = monitor.getViolations();
      expect(violations).toEqual([]);
    });

    it('triggers SKILL-013 for undeclared file access', () => {
      monitor.recordFileAccess('/etc/passwd', 'read');

      const violations = monitor.getViolations();
      expect(violations.length).toBe(1);
      expect(violations[0].id).toBe('SKILL-013');
      expect(violations[0].type).toBe('permission-overreach');
      expect(violations[0].description).toContain('/etc/passwd');
      expect(violations[0].description).toContain('test-skill');
    });

    it('handles wildcard file access patterns', () => {
      // /home/user/project/* should allow anything under that path
      monitor.recordFileAccess('/home/user/project/deep/nested/file.ts');
      expect(monitor.getViolations()).toEqual([]);
    });
  });

  describe('network access tracking', () => {
    it('does not trigger violation for declared network hosts', () => {
      monitor.recordNetworkCall('api.example.com', 443, 'https');
      monitor.recordNetworkCall('cdn.example.com');

      expect(monitor.getViolations()).toEqual([]);
    });

    it('triggers SKILL-014 for undeclared network access', () => {
      monitor.recordNetworkCall('evil.com', 443, 'https');

      const violations = monitor.getViolations();
      expect(violations.length).toBe(1);
      expect(violations[0].id).toBe('SKILL-014');
      expect(violations[0].type).toBe('undeclared-network');
      expect(violations[0].description).toContain('evil.com');
    });

    it('allows subdomain of declared host', () => {
      monitor.recordNetworkCall('sub.api.example.com');
      expect(monitor.getViolations()).toEqual([]);
    });
  });

  describe('tool use tracking', () => {
    it('does not trigger violation for declared tools', () => {
      monitor.recordToolUse('read_file');
      monitor.recordToolUse('search');

      expect(monitor.getViolations()).toEqual([]);
    });

    it('triggers SKILL-013 for undeclared tool use', () => {
      monitor.recordToolUse('shell_execute');

      const violations = monitor.getViolations();
      expect(violations.length).toBe(1);
      expect(violations[0].id).toBe('SKILL-013');
      expect(violations[0].description).toContain('shell_execute');
    });
  });

  describe('credential access tracking', () => {
    it('does not trigger violation for declared credentials', () => {
      monitor.recordCredentialAccess('openai-api');

      expect(monitor.getViolations()).toEqual([]);
    });

    it('triggers SKILL-013 for undeclared credential access', () => {
      monitor.recordCredentialAccess('aws-secret');

      const violations = monitor.getViolations();
      expect(violations.length).toBe(1);
      expect(violations[0].id).toBe('SKILL-013');
      expect(violations[0].description).toContain('aws-secret');
    });
  });

  describe('various capability combinations', () => {
    it('handles skill with no declared capabilities (all actions are violations)', () => {
      const emptyMonitor = new SkillCapabilityMonitor({
        name: 'empty-skill',
        fileAccess: [],
        networkAccess: [],
        tools: [],
        credentials: [],
      });

      emptyMonitor.recordFileAccess('/tmp/file');
      emptyMonitor.recordNetworkCall('example.com');
      emptyMonitor.recordToolUse('read_file');
      emptyMonitor.recordCredentialAccess('api-key');

      const violations = emptyMonitor.getViolations();
      expect(violations.length).toBe(4);
    });

    it('handles wildcard permissions', () => {
      const wildcardMonitor = new SkillCapabilityMonitor({
        name: 'admin-skill',
        fileAccess: ['*'],
        networkAccess: ['*'],
        tools: ['*'],
        credentials: ['*'],
      });

      wildcardMonitor.recordFileAccess('/any/path');
      wildcardMonitor.recordNetworkCall('any.host.com');
      wildcardMonitor.recordToolUse('any_tool');
      wildcardMonitor.recordCredentialAccess('any-cred');

      expect(wildcardMonitor.getViolations()).toEqual([]);
    });

    it('tracks observed behavior correctly', () => {
      monitor.recordFileAccess('/tmp/allowed', 'read');
      monitor.recordNetworkCall('api.example.com', 443);
      monitor.recordToolUse('read_file');
      monitor.recordCredentialAccess('openai-api');

      const observed = monitor.getObserved();

      expect(observed.fileAccesses.length).toBe(1);
      expect(observed.networkCalls.length).toBe(1);
      expect(observed.toolUses.length).toBe(1);
      expect(observed.credentialAccesses.length).toBe(1);
    });

    it('accumulates multiple violations', () => {
      monitor.recordNetworkCall('evil1.com');
      monitor.recordNetworkCall('evil2.com');
      monitor.recordFileAccess('/etc/shadow');

      const violations = monitor.getViolations();
      expect(violations.length).toBe(3);
      expect(violations.filter((v) => v.id === 'SKILL-014').length).toBe(2);
      expect(violations.filter((v) => v.id === 'SKILL-013').length).toBe(1);
    });
  });

  describe('createCapabilityMonitor helper', () => {
    it('creates a monitor from declared capabilities', () => {
      const mon = createCapabilityMonitor(declared);
      expect(mon).toBeInstanceOf(SkillCapabilityMonitor);

      mon.recordNetworkCall('undeclared.com');
      expect(mon.getViolations().length).toBe(1);
    });
  });

  describe('reset', () => {
    it('clears all observations and violations', () => {
      monitor.recordNetworkCall('evil.com');
      expect(monitor.getViolations().length).toBe(1);

      monitor.reset();

      expect(monitor.getViolations()).toEqual([]);
      expect(monitor.getObserved().networkCalls).toEqual([]);
    });
  });

  describe('Monitor interface', () => {
    it('implements start/stop/isRunning', async () => {
      expect(monitor.isRunning()).toBe(false);
      await monitor.start();
      expect(monitor.isRunning()).toBe(true);
      await monitor.stop();
      expect(monitor.isRunning()).toBe(false);
    });

    it('has type "skill"', () => {
      expect(monitor.type).toBe('skill');
    });
  });
});
