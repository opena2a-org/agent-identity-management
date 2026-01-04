/**
 * Tests for AIM SDK Type Definitions
 *
 * Tests cover:
 * - Enum values
 * - Type structure validation
 * - Interface contracts
 */

import { describe, it, expect } from 'vitest';
import {
  AgentType,
  RiskLevel,
  AgentStatus,
  WebhookEvent,
} from './index';

describe('AgentType Enum', () => {
  describe('LLM Provider Types', () => {
    it('should have CLAUDE type', () => {
      expect(AgentType.CLAUDE).toBe('claude');
    });

    it('should have GPT type', () => {
      expect(AgentType.GPT).toBe('gpt');
    });

    it('should have GEMINI type', () => {
      expect(AgentType.GEMINI).toBe('gemini');
    });

    it('should have LLAMA type', () => {
      expect(AgentType.LLAMA).toBe('llama');
    });

    it('should have MISTRAL type', () => {
      expect(AgentType.MISTRAL).toBe('mistral');
    });

    it('should have COHERE type', () => {
      expect(AgentType.COHERE).toBe('cohere');
    });
  });

  describe('Framework Types', () => {
    it('should have LANGCHAIN type', () => {
      expect(AgentType.LANGCHAIN).toBe('langchain');
    });

    it('should have LLAMAINDEX type', () => {
      expect(AgentType.LLAMAINDEX).toBe('llamaindex');
    });

    it('should have AUTOGEN type', () => {
      expect(AgentType.AUTOGEN).toBe('autogen');
    });

    it('should have CREWAI type', () => {
      expect(AgentType.CREWAI).toBe('crewai');
    });

    it('should have LANGGRAPH type', () => {
      expect(AgentType.LANGGRAPH).toBe('langgraph');
    });

    it('should have HAYSTACK type', () => {
      expect(AgentType.HAYSTACK).toBe('haystack');
    });

    it('should have SEMANTIC_KERNEL type', () => {
      expect(AgentType.SEMANTIC_KERNEL).toBe('semantic_kernel');
    });
  });

  describe('Copilot/Assistant Types', () => {
    it('should have COPILOT type', () => {
      expect(AgentType.COPILOT).toBe('copilot');
    });

    it('should have ASSISTANT type', () => {
      expect(AgentType.ASSISTANT).toBe('assistant');
    });

    it('should have CHATBOT type', () => {
      expect(AgentType.CHATBOT).toBe('chatbot');
    });
  });

  describe('Autonomous Agent Types', () => {
    it('should have AUTOGPT type', () => {
      expect(AgentType.AUTOGPT).toBe('autogpt');
    });

    it('should have BABYAGI type', () => {
      expect(AgentType.BABYAGI).toBe('babyagi');
    });
  });

  describe('Generic Types', () => {
    it('should have CUSTOM type', () => {
      expect(AgentType.CUSTOM).toBe('custom');
    });

    it('should have AI_AGENT type', () => {
      expect(AgentType.AI_AGENT).toBe('ai_agent');
    });
  });
});

describe('RiskLevel Enum', () => {
  it('should have LOW level', () => {
    expect(RiskLevel.LOW).toBe('low');
  });

  it('should have MEDIUM level', () => {
    expect(RiskLevel.MEDIUM).toBe('medium');
  });

  it('should have HIGH level', () => {
    expect(RiskLevel.HIGH).toBe('high');
  });

  it('should have CRITICAL level', () => {
    expect(RiskLevel.CRITICAL).toBe('critical');
  });

  it('should have exactly 4 levels', () => {
    const levels = Object.values(RiskLevel);
    expect(levels.length).toBe(4);
  });
});

describe('AgentStatus Enum', () => {
  it('should have PENDING status', () => {
    expect(AgentStatus.PENDING).toBe('pending');
  });

  it('should have VERIFIED status', () => {
    expect(AgentStatus.VERIFIED).toBe('verified');
  });

  it('should have SUSPENDED status', () => {
    expect(AgentStatus.SUSPENDED).toBe('suspended');
  });

  it('should have REVOKED status', () => {
    expect(AgentStatus.REVOKED).toBe('revoked');
  });

  it('should have exactly 4 statuses', () => {
    const statuses = Object.values(AgentStatus);
    expect(statuses.length).toBe(4);
  });
});

describe('WebhookEvent Enum', () => {
  describe('Alert Events', () => {
    it('should have ALERT_CREATED event', () => {
      expect(WebhookEvent.ALERT_CREATED).toBe('alert.created');
    });

    it('should have ALERT_ACKNOWLEDGED event', () => {
      expect(WebhookEvent.ALERT_ACKNOWLEDGED).toBe('alert.acknowledged');
    });
  });

  describe('Agent Events', () => {
    it('should have AGENT_CREATED event', () => {
      expect(WebhookEvent.AGENT_CREATED).toBe('agent.created');
    });

    it('should have AGENT_VERIFIED event', () => {
      expect(WebhookEvent.AGENT_VERIFIED).toBe('agent.verified');
    });

    it('should have AGENT_UPDATED event', () => {
      expect(WebhookEvent.AGENT_UPDATED).toBe('agent.updated');
    });

    it('should have AGENT_SUSPENDED event', () => {
      expect(WebhookEvent.AGENT_SUSPENDED).toBe('agent.suspended');
    });

    it('should have AGENT_DELETED event', () => {
      expect(WebhookEvent.AGENT_DELETED).toBe('agent.deleted');
    });
  });

  describe('Trust Score Events', () => {
    it('should have TRUST_SCORE_CHANGED event', () => {
      expect(WebhookEvent.TRUST_SCORE_CHANGED).toBe('trust_score.changed');
    });

    it('should have TRUST_SCORE_LOW event', () => {
      expect(WebhookEvent.TRUST_SCORE_LOW).toBe('trust_score.low');
    });
  });

  describe('Security Events', () => {
    it('should have SECURITY_VIOLATION event', () => {
      expect(WebhookEvent.SECURITY_VIOLATION).toBe('security.violation');
    });

    it('should have COMPLIANCE_VIOLATION event', () => {
      expect(WebhookEvent.COMPLIANCE_VIOLATION).toBe('compliance.violation');
    });

    it('should have CAPABILITY_DRIFT event', () => {
      expect(WebhookEvent.CAPABILITY_DRIFT).toBe('security.capability_drift');
    });

    it('should have DATA_EXFILTRATION event', () => {
      expect(WebhookEvent.DATA_EXFILTRATION).toBe('security.data_exfiltration');
    });
  });

  describe('API Key Events', () => {
    it('should have API_KEY_CREATED event', () => {
      expect(WebhookEvent.API_KEY_CREATED).toBe('api_key.created');
    });

    it('should have API_KEY_REVOKED event', () => {
      expect(WebhookEvent.API_KEY_REVOKED).toBe('api_key.revoked');
    });

    it('should have API_KEY_EXPIRED event', () => {
      expect(WebhookEvent.API_KEY_EXPIRED).toBe('api_key.expired');
    });
  });
});

describe('Type Validation', () => {
  describe('AIMClientConfig interface', () => {
    it('should accept valid config object', () => {
      // Type-level test - if this compiles, the interface is valid
      const config: import('./index').AIMClientConfig = {
        baseUrl: 'https://aim.example.com',
        organizationId: 'org-123',
        apiKey: 'api-key',
        autoRegister: true,
        timeout: 30000,
        debug: false,
        headers: { 'X-Custom': 'value' },
      };
      expect(config).toBeDefined();
    });

    it('should accept minimal config', () => {
      const config: import('./index').AIMClientConfig = {};
      expect(config).toBeDefined();
    });
  });

  describe('RegisterAgentOptions interface', () => {
    it('should accept valid options', () => {
      const options: import('./index').RegisterAgentOptions = {
        name: 'my-agent',
        displayName: 'My Agent',
        description: 'Test agent',
        agentType: AgentType.CLAUDE,
        version: '1.0.0',
        capabilities: ['db:read', 'api:call'],
        talksTo: ['mcp-server-1'],
        metadata: { key: 'value' },
      };
      expect(options.name).toBe('my-agent');
    });

    it('should require name field', () => {
      const options: import('./index').RegisterAgentOptions = {
        name: 'required-name',
      };
      expect(options.name).toBeDefined();
    });
  });

  describe('Agent interface', () => {
    it('should have required fields', () => {
      const agent: import('./index').Agent = {
        id: 'agent-123',
        name: 'test-agent',
        displayName: 'Test Agent',
        agentType: 'claude',
        version: '1.0.0',
        status: AgentStatus.VERIFIED,
        trustScore: 0.85,
        organizationId: 'org-456',
        capabilities: ['db:read'],
        talksTo: [],
        createdAt: '2024-01-01T00:00:00Z',
        updatedAt: '2024-01-01T00:00:00Z',
      };
      expect(agent.id).toBe('agent-123');
      expect(agent.trustScore).toBe(0.85);
    });
  });

  describe('VerificationResult interface', () => {
    it('should have required fields', () => {
      const result: import('./index').VerificationResult = {
        verified: true,
        agentId: 'agent-123',
        agentName: 'test-agent',
        trustScore: 0.9,
        riskLevel: RiskLevel.LOW,
        actionAllowed: true,
        timestamp: '2024-01-01T00:00:00Z',
      };
      expect(result.verified).toBe(true);
      expect(result.actionAllowed).toBe(true);
    });

    it('should support denial reason', () => {
      const result: import('./index').VerificationResult = {
        verified: false,
        agentId: 'agent-123',
        agentName: 'test-agent',
        trustScore: 0.3,
        riskLevel: RiskLevel.HIGH,
        actionAllowed: false,
        denialReason: 'Trust score too low',
        timestamp: '2024-01-01T00:00:00Z',
      };
      expect(result.actionAllowed).toBe(false);
      expect(result.denialReason).toBe('Trust score too low');
    });
  });

  describe('AgentCredentials interface', () => {
    it('should have required fields', () => {
      const creds: import('./index').AgentCredentials = {
        agentId: 'agent-123',
        privateKey: 'base64-private-key',
        publicKey: 'base64-public-key',
        organizationId: 'org-456',
        createdAt: '2024-01-01T00:00:00Z',
      };
      expect(creds.agentId).toBe('agent-123');
      expect(creds.privateKey).toBeDefined();
    });
  });

  describe('TokenResponse interface', () => {
    it('should have required fields', () => {
      const token: import('./index').TokenResponse = {
        accessToken: 'access-token-xyz',
        tokenType: 'Bearer',
        expiresIn: 3600,
      };
      expect(token.accessToken).toBe('access-token-xyz');
      expect(token.tokenType).toBe('Bearer');
    });

    it('should support optional scope', () => {
      const token: import('./index').TokenResponse = {
        accessToken: 'token',
        tokenType: 'Bearer',
        expiresIn: 3600,
        scope: 'read write',
      };
      expect(token.scope).toBe('read write');
    });
  });

  describe('WebhookPayload interface', () => {
    it('should have required fields', () => {
      const payload: import('./index').WebhookPayload = {
        id: 'webhook-123',
        event: WebhookEvent.AGENT_CREATED,
        timestamp: '2024-01-01T00:00:00Z',
        organizationId: 'org-456',
        data: { agentId: 'agent-789' },
      };
      expect(payload.event).toBe(WebhookEvent.AGENT_CREATED);
    });
  });

  describe('MCPServer interface', () => {
    it('should have required fields', () => {
      const server: import('./index').MCPServer = {
        id: 'mcp-123',
        name: 'test-server',
        displayName: 'Test Server',
        serverType: 'custom',
        capabilities: ['tool:execute'],
        trustScore: 0.9,
        status: 'active',
      };
      expect(server.name).toBe('test-server');
      expect(server.trustScore).toBe(0.9);
    });
  });
});
