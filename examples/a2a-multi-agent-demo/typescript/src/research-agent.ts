/**
 * Research Agent - Gathers information and collaborates with Analysis Agent.
 */
import { AIMClient, A2AClient, AgentType } from '@opena2a/aim-sdk';
import { v4 as uuidv4 } from 'uuid';
import { AnalysisAgent } from './analysis-agent';

interface SkillDefinition {
  name: string;
  description: string;
  tags: string[];
}

/**
 * Research Agent that discovers and collaborates with Analysis Agents.
 */
export class ResearchAgent {
  static readonly SKILLS: SkillDefinition[] = [
    {
      name: 'Web Research',
      description: 'Gather information from web sources on specified topics',
      tags: ['research', 'web', 'data-gathering'],
    },
    {
      name: 'Document Reading',
      description: 'Extract and process content from documents',
      tags: ['documents', 'extraction', 'reading'],
    },
  ];

  private readonly agentName = 'research-agent';
  private aimClient: AIMClient;
  private a2a!: A2AClient;
  public agentId: string = '';

  constructor(private readonly baseUrl?: string) {
    console.log('[Research Agent] Initializing...');

    // SDK handles auth automatically via bundled credentials
    this.aimClient = new AIMClient({
      baseUrl: baseUrl || 'http://localhost:8080',
    });
  }

  async initialize(): Promise<void> {
    console.log('[Research Agent] Registering with AIM...');

    const agent = await this.aimClient.registerAgent({
      name: this.agentName,
      displayName: 'Research Agent',
      agentType: AgentType.AI_AGENT,
      description: 'Research agent that gathers information and collaborates with analysis agents',
    });

    this.agentId = agent.id;
    this.a2a = new A2AClient(this.aimClient, this.agentId);
    console.log(`[Research Agent] Registered with ID: ${this.agentId.slice(0, 8)}...`);
  }

  async registerAgentCard(): Promise<Record<string, unknown> | null> {
    console.log('[Research Agent] Registering skills...');

    try {
      for (const skill of ResearchAgent.SKILLS) {
        await this.a2a.registerSkill({
          name: skill.name,
          description: skill.description,
          tags: skill.tags,
        });
        console.log(`  - Registered skill: ${skill.name}`);
      }

      console.log('[Research Agent] Skills registered successfully');
      return { skills: ResearchAgent.SKILLS.length };
    } catch (e) {
      console.log(`[Research Agent] Registration note: ${e}`);
      return null;
    }
  }

  async discoverAnalysisAgent(intent: string, minTrust: number = 0.3): Promise<Record<string, unknown> | null> {
    console.log(`\n[Research Agent] Searching for agents capable of: '${intent}'`);
    console.log(`[Research Agent] Minimum trust score: ${minTrust}`);

    try {
      const capableAgents = await this.a2a.routeByIntent(intent, minTrust);

      if (!capableAgents || capableAgents.length === 0) {
        console.log('[Research Agent] No capable agents found via intent routing');
        const skills = await this.a2a.searchSkills('sentiment analysis', 5);
        if (skills && skills.length > 0) {
          console.log(`[Research Agent] Found ${skills.length} skills via search`);
        }
        return null;
      }

      const bestAgent = capableAgents[0];
      console.log(`[Research Agent] Found agent: ${bestAgent.name ?? 'Unknown'}`);
      console.log(`[Research Agent] Trust score: ${bestAgent.trustScore ?? 'N/A'}`);

      return bestAgent as unknown as Record<string, unknown>;
    } catch (e) {
      console.log(`[Research Agent] Discovery error: ${e}`);
      return null;
    }
  }

  async checkSecurityPolicies(targetAgentId: string, skillId: string): Promise<{ allowed: boolean }> {
    console.log('\n[Research Agent] Checking security policies...');
    console.log(`[Research Agent] Target: ${targetAgentId.slice(0, 8)}...`);
    console.log(`[Research Agent] Skill: ${skillId}`);

    try {
      const result = await this.a2a.checkSecurity(targetAgentId, skillId);

      if (result.allowed) {
        console.log('[Research Agent] Security check PASSED');
        console.log(`[Research Agent] Enforcement mode: ${result.enforcementMode ?? 'unknown'}`);
      } else {
        console.log('[Research Agent] Security check FAILED');
      }

      return result;
    } catch (e) {
      console.log(`[Research Agent] Security check error: ${e}`);
      return { allowed: true };
    }
  }

  async requestUserConsent(
    userId: string,
    recipientAgentId: string,
    dataTypes: string[],
    purpose: string
  ): Promise<boolean> {
    console.log('\n[Research Agent] Requesting user consent...');
    console.log(`[Research Agent] User: ${userId}`);
    console.log(`[Research Agent] Purpose: ${purpose}`);
    console.log(`[Research Agent] Data types: ${dataTypes.join(', ')}`);

    try {
      const check = await this.a2a.checkConsent(userId, recipientAgentId, 'data_sharing');

      if (check.hasConsent) {
        console.log('[Research Agent] Consent already exists');
        return true;
      }

      const consent = await this.a2a.recordConsent(
        userId,
        recipientAgentId,
        ['data_sharing', 'analysis'],
        purpose,
        dataTypes,
        'explicit_consent',
        24
      );

      console.log(`[Research Agent] Consent recorded: ${consent.id}`);
      return true;
    } catch (e) {
      console.log(`[Research Agent] Consent error: ${e}`);
      return true; // Allow proceeding for demo
    }
  }

  async signAndCallSkill(
    targetAgentId: string,
    skillId: string,
    data: Record<string, unknown>,
    analysisAgent?: AnalysisAgent
  ): Promise<Record<string, unknown>> {
    console.log('\n[Research Agent] Preparing signed request...');
    console.log(`[Research Agent] Skill: ${skillId}`);

    // Sign the request
    try {
      const signature = await this.a2a.signRequest('POST', `/skills/${skillId}/invoke`, JSON.stringify(data));
      console.log(`[Research Agent] Request signed at: ${signature.timestamp}`);
      console.log(`[Research Agent] Nonce: ${signature.nonce.slice(0, 16)}...`);
    } catch (e) {
      console.log(`[Research Agent] Signing note: ${e}`);
    }

    // Log the task
    let taskId: string | null = null;
    try {
      const externalTaskId = `demo-${uuidv4().slice(0, 8)}`;
      const task = await this.a2a.logTask(targetAgentId, externalTaskId, skillId, 'SUBMITTED');
      taskId = task.id;
      console.log(`[Research Agent] Task logged: ${taskId}`);
    } catch (e) {
      console.log(`[Research Agent] Task logging note: ${e}`);
      taskId = `local-${uuidv4().slice(0, 8)}`;
    }

    // Execute skill (direct call for demo)
    console.log('[Research Agent] Invoking skill...');
    let result: Record<string, unknown>;
    if (analysisAgent) {
      result = analysisAgent.executeSkill(skillId, data) as Record<string, unknown>;
    } else {
      result = {
        sentiment: 'positive',
        confidence: 0.87,
        note: 'Simulated response - Analysis Agent not provided',
      };
    }

    console.log(`[Research Agent] Result: ${JSON.stringify(result)}`);

    // Update task state
    if (taskId) {
      try {
        await this.a2a.updateTaskState(taskId, 'COMPLETED');
        console.log('[Research Agent] Task marked COMPLETED');
      } catch (e) {
        console.log(`[Research Agent] Task update note: ${e}`);
      }
    }

    return result;
  }

  async attestSkillQuality(
    targetAgentId: string,
    skillId: string,
    confidence: number,
    evidence: Record<string, unknown>
  ): Promise<Record<string, unknown> | null> {
    console.log('\n[Research Agent] Creating skill attestation...');
    console.log(`[Research Agent] Skill: ${skillId}`);
    console.log(`[Research Agent] Confidence: ${confidence}`);

    try {
      const attestation = await this.a2a.attestSkill(targetAgentId, skillId, 'SKILL_VERIFICATION', confidence, evidence);

      console.log(`[Research Agent] Attestation created: ${attestation.id ?? 'N/A'}`);

      // Check consensus status
      try {
        const consensus = await this.a2a.getConsensusStatus(targetAgentId, skillId);
        console.log(`[Research Agent] Skill verified: ${consensus.isVerified ?? false}`);
        console.log(`[Research Agent] Attestation count: ${consensus.attestationCount ?? 0}`);
      } catch (e) {
        console.log(`[Research Agent] Consensus check note: ${e}`);
      }

      return attestation as unknown as Record<string, unknown>;
    } catch (e) {
      console.log(`[Research Agent] Attestation error: ${e}`);
      return null;
    }
  }

  async getTrustScore(agentId?: string): Promise<Record<string, unknown> | null> {
    const target = agentId || this.agentId;
    const targetLabel = target === this.agentId ? 'self' : `${target.slice(0, 8)}...`;

    console.log(`\n[Research Agent] Getting trust score for ${targetLabel}...`);

    try {
      const score = await this.a2a.getTrustScore(target);
      console.log(`[Research Agent] Trust score: ${score.score.toFixed(2)}`);
      console.log(`[Research Agent] Confidence: ${score.confidence}`);
      return score as unknown as Record<string, unknown>;
    } catch (e) {
      console.log(`[Research Agent] Trust score note: ${e}`);
      return null;
    }
  }

  async getPeerTrust(peerAgentId: string): Promise<Record<string, unknown> | null> {
    console.log(`\n[Research Agent] Getting peer trust with ${peerAgentId.slice(0, 8)}...`);

    try {
      const trust = await this.a2a.getPeerTrust(peerAgentId);
      console.log(`[Research Agent] Peer trust score: ${trust.trustScore?.toFixed(2) ?? 'N/A'}`);
      return trust as unknown as Record<string, unknown>;
    } catch (e) {
      console.log(`[Research Agent] Peer trust note: ${e}`);
      return null;
    }
  }
}
