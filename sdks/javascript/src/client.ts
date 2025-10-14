import { AIMClientConfig, DetectedMCP, DetectionMethod, DetectionEvent } from './types';
import { ImportDetector } from './detection/import-detector';
import { ConnectionDetector } from './detection/connection-detector';
import { APIReporter } from './reporting/api-reporter';
import { registerAgent, registerAgentWithOAuth, RegisterOptions } from './registration';
import { autoDetectMCPs } from './detection/capability-detection';
import { loadCredentials } from './credentials';
import { KeyPair } from './signing';

export class AIMClient {
  private config: Required<AIMClientConfig>;
  private reporter: APIReporter;
  private detectors: DetectionMethod[] = [];
  private reportInterval?: NodeJS.Timeout;
  private keyPair?: KeyPair;

  constructor(config: AIMClientConfig) {
    // Set defaults
    this.config = {
      autoDetect: true,
      detectionMethods: ['import', 'connection'],
      reportInterval: 10000, // 10 seconds
      ...config,
    };

    this.reporter = new APIReporter(
      this.config.apiUrl,
      this.config.apiKey,
      this.config.agentId
    );

    if (this.config.autoDetect) {
      this.initializeDetectors();
    }
  }

  /**
   * Create a client with credentials loaded from keyring
   */
  static async fromKeyring(apiUrl: string): Promise<AIMClient> {
    const creds = await loadCredentials();
    if (!creds) {
      throw new Error('No credentials found in keyring. Please register an agent first.');
    }

    return new AIMClient({
      apiUrl,
      apiKey: creds.apiKey,
      agentId: creds.agentId,
    });
  }

  /**
   * Register a new agent
   */
  async registerAgent(options: RegisterOptions) {
    return await registerAgent(this.config.apiUrl, options);
  }

  /**
   * Register agent with OAuth
   */
  async registerAgentWithOAuth(options: RegisterOptions) {
    return await registerAgentWithOAuth(this.config.apiUrl, options);
  }

  /**
   * Auto-detect MCPs and report them
   */
  async autoDetectAndReport(): Promise<void> {
    const detection = await autoDetectMCPs();

    for (const mcp of detection.mcps) {
      try {
        await this.reportMCP(mcp.name);
        console.log(`✅ Reported: ${mcp.name}`);
      } catch (err) {
        console.warn(`Warning: Failed to report ${mcp.name}:`, err);
      }
    }
  }

  private initializeDetectors() {
    const methods = this.config.detectionMethods;

    if (methods.includes('import')) {
      const importDetector = new ImportDetector();
      importDetector.start();
      this.detectors.push(importDetector);
    }

    if (methods.includes('connection')) {
      const connectionDetector = new ConnectionDetector();
      connectionDetector.start();
      this.detectors.push(connectionDetector);
    }

    // Start periodic reporting
    this.reportInterval = setInterval(() => {
      this.reportDetections().catch(err => {
        console.error('[AIM SDK] Failed to report detections:', err);
      });
    }, this.config.reportInterval);
  }

  private async reportDetections(): Promise<void> {
    const allDetections = this.detectors.flatMap(d => d.getDetections());

    if (allDetections.length === 0) {
      return;
    }

    // Convert DetectedMCP to DetectionEvent format
    const events: DetectionEvent[] = allDetections.map(detection => ({
      mcpServer: detection.name,
      detectionMethod: detection.detectionMethod === 'sdk_import' ? 'sdk_import' : 'sdk_runtime',
      confidence: detection.confidenceScore,
      details: detection.details,
      sdkVersion: '1.0.0',
      timestamp: new Date(),
    }));

    await this.reporter.report({
      detections: events,
    });
  }

  /**
   * Manually trigger detection (for testing or on-demand use)
   */
  async detect(): Promise<DetectedMCP[]> {
    const allDetections = this.detectors.flatMap(d => d.getDetections());
    return allDetections;
  }

  /**
   * Manually report a specific MCP usage
   */
  async reportMCP(name: string): Promise<void> {
    await this.reporter.report({
      detections: [
        {
          mcpServer: name,
          detectionMethod: 'sdk_import',
          confidence: 100.0,
          sdkVersion: '1.0.0',
          timestamp: new Date(),
        },
      ],
    });
  }

  /**
   * Register an MCP server to this agent's "talks_to" list.
   *
   * This creates a relationship between the agent and an MCP server,
   * indicating that the agent communicates with this MCP server.
   *
   * @param mcpServerId - MCP server ID or name to register
   * @param detectionMethod - How the MCP was detected ("manual", "auto_sdk", "auto_config", "cli")
   * @param confidence - Detection confidence score (0-100, default: 100 for manual)
   * @param metadata - Optional additional context about the detection
   *
   * @example
   * ```typescript
   * // Register filesystem MCP server
   * const result = await client.registerMCP(
   *   "filesystem-mcp-server",
   *   "manual",
   *   100.0
   * );
   * console.log(`Registered ${result.added} MCP server(s)`);
   * ```
   */
  async registerMCP(
    mcpServerId: string,
    detectionMethod: string = 'manual',
    confidence: number = 100.0,
    metadata?: Record<string, any>
  ): Promise<{ success: boolean; message: string; added: number; agent_id: string; mcp_server_ids: string[] }> {
    try {
      const response = await fetch(
        `${this.config.apiUrl}/api/v1/sdk-api/agents/${this.config.agentId}/mcp-servers`,
        {
          method: 'PUT',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${this.config.apiKey}`,
          },
          body: JSON.stringify({
            mcp_server_ids: [mcpServerId],
            detected_method: detectionMethod,
            confidence: confidence,
            metadata: metadata || {},
          }),
        }
      );

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`MCP registration failed: ${errorText}`);
      }

      return (await response.json()) as { success: boolean; message: string; added: number; agent_id: string; mcp_server_ids: string[] };
    } catch (error) {
      throw new Error(`MCP registration failed: ${error}`);
    }
  }

  /**
   * Report SDK integration status to AIM dashboard.
   *
   * This updates the Detection tab to show that the AIM SDK is installed
   * and integrated with the agent, enabling auto-detection features.
   *
   * @param sdkVersion - SDK version string (e.g., "aim-sdk-js@1.0.0")
   * @param platform - Platform/language (e.g., "javascript", "typescript", "node")
   * @param capabilities - Optional list of SDK capabilities enabled
   *
   * @example
   * ```typescript
   * // Report SDK integration
   * const result = await client.reportSDKIntegration(
   *   "aim-sdk-js@1.0.0",
   *   "javascript",
   *   ["auto_detect_mcps", "capability_detection"]
   * );
   * console.log(`SDK integration reported: ${result.message}`);
   * ```
   */
  async reportSDKIntegration(
    sdkVersion: string,
    platform: string = 'javascript',
    capabilities?: string[]
  ): Promise<{ success: boolean; detectionsProcessed: number; message: string }> {
    try {
      // Create SDK integration detection event
      const detectionEvent = {
        mcpServer: 'aim-sdk-integration',
        detectionMethod: 'sdk_integration' as const,
        confidence: 100.0,
        details: {
          platform: platform,
          capabilities: capabilities || [],
          integrated: true,
        },
        sdkVersion: sdkVersion,
        timestamp: new Date(),
      };

      const response = await fetch(
        `${this.config.apiUrl}/api/v1/sdk-api/agents/${this.config.agentId}/detection/report`,
        {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${this.config.apiKey}`,
          },
          body: JSON.stringify({
            detections: [detectionEvent],
          }),
        }
      );

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`SDK integration report failed: ${errorText}`);
      }

      return (await response.json()) as { success: boolean; detectionsProcessed: number; message: string };
    } catch (error) {
      throw new Error(`SDK integration report failed: ${error}`);
    }
  }

  /**
   * Report detected agent capabilities to the backend.
   *
   * This populates the Capabilities tab in the AIM dashboard with the agent's
   * detected or manually specified capabilities.
   *
   * @param capabilities - List of capability strings (e.g., ["read_files", "access_database"])
   *
   * @example
   * ```typescript
   * // Auto-detect and report capabilities
   * import { autoDetectCapabilities } from '@opena2a/aim-sdk/capability_detection';
   *
   * const caps = await autoDetectCapabilities();
   * await client.reportCapabilities(caps);
   * console.log(`Reported ${caps.length} capabilities`);
   * ```
   */
  async reportCapabilities(capabilities: string[]): Promise<void> {
    if (!capabilities || capabilities.length === 0) {
      throw new Error('capabilities cannot be empty');
    }

    // Grant each capability to the agent
    // Note: Using the POST /agents/{id}/capabilities endpoint
    for (const capability of capabilities) {
      try {
        const response = await fetch(
          `${this.config.apiUrl}/api/v1/sdk-api/agents/${this.config.agentId}/capabilities`,
          {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
              'Authorization': `Bearer ${this.config.apiKey}`,
            },
            body: JSON.stringify({
              capabilityType: capability,
              scope: {},
            }),
          }
        );

        // Accept 201 (Created) or 409 (Conflict - already exists) as success
        if (!response.ok && response.status !== 409) {
          const errorText = await response.text();
          console.warn(`Failed to report capability ${capability}: ${errorText}`);
        }
      } catch (error) {
        console.warn(`Failed to report capability ${capability}:`, error);
      }
    }
  }

  /**
   * Auto-detect capabilities and report them to the backend in one call.
   *
   * @example
   * ```typescript
   * // Auto-detect and report in one call
   * const caps = await client.autoDetectAndReportCapabilities();
   * console.log(`Auto-detected and reported ${caps.length} capabilities: ${caps.join(', ')}`);
   * ```
   */
  async autoDetectAndReportCapabilities(): Promise<string[]> {
    const { autoDetectCapabilities } = await import('./capability_detection');

    // Auto-detect capabilities
    const caps = await autoDetectCapabilities();

    if (caps.length === 0) {
      return [];
    }

    // Report to backend
    await this.reportCapabilities(caps);

    return caps;
  }

  /**
   * Set the Ed25519 keypair for signing
   * @param keyPair Ed25519 keypair
   */
  setKeyPair(keyPair: KeyPair): void {
    this.keyPair = keyPair;
  }

  /**
   * Load keypair from base64-encoded private key
   * @param privateKeyBase64 Base64-encoded private key
   */
  loadKeyPairFromBase64(privateKeyBase64: string): void {
    this.keyPair = KeyPair.fromBase64(privateKeyBase64);
  }

  /**
   * Get public key as base64 string
   * @returns Base64-encoded public key, or empty string if no keypair
   */
  getPublicKey(): string {
    if (!this.keyPair) {
      return '';
    }
    return this.keyPair.publicKeyBase64();
  }

  /**
   * Sign a message using the agent's private key
   * @param message Message to sign
   * @returns Base64-encoded signature
   * @throws Error if no keypair is set
   */
  signMessage(message: string): string {
    if (!this.keyPair) {
      throw new Error('No keypair set. Call setKeyPair() or loadKeyPairFromBase64() first.');
    }
    return this.keyPair.sign(message);
  }

  /**
   * Send verification request to backend with cryptographic signature
   * @param actionType Type of action to verify (e.g., "execute", "read", "write")
   * @param resource Resource being accessed (e.g., "database", "api")
   * @param context Additional context for verification
   * @returns Verification result from backend
   */
  async verifyAction(
    actionType: string,
    resource: string,
    context: Record<string, any> = {}
  ): Promise<{
    verified: boolean;
    message: string;
    trustScore?: number;
    risk?: string;
  }> {
    if (!this.keyPair) {
      throw new Error('No keypair set. Call setKeyPair() or loadKeyPairFromBase64() first.');
    }

    // Create verification payload
    const payload = {
      action_type: actionType,
      resource: resource,
      context: context,
      timestamp: new Date().toISOString(),
    };

    // Sign the payload
    const signature = this.keyPair.signPayload(payload);

    // Send verification request to backend
    const response = await fetch(
      `${this.config.apiUrl}/api/v1/agents/${this.config.agentId}/verify`,
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${this.config.apiKey}`,
        },
        body: JSON.stringify({
          ...payload,
          signature: signature,
          public_key: this.keyPair.publicKeyBase64(),
        }),
      }
    );

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`Verification failed: ${errorText}`);
    }

    return (await response.json()) as {
      verified: boolean;
      message: string;
      trustScore?: number;
      risk?: string;
    };
  }

  /**
   * Clean up resources
   */
  destroy(): void {
    if (this.reportInterval) {
      clearInterval(this.reportInterval);
    }
    this.detectors.forEach(d => d.stop());
  }
}

export { AIMClientConfig };
