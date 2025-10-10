import { AIMClientConfig, DetectedMCP, DetectionMethod, DetectionEvent } from './types';
import { ImportDetector } from './detection/import-detector';
import { ConnectionDetector } from './detection/connection-detector';
import { APIReporter } from './reporting/api-reporter';
import { registerAgent, registerAgentWithOAuth, RegisterOptions } from './registration';
import { autoDetectMCPs } from './detection/capability-detection';
import { loadCredentials } from './credentials';

export class AIMClient {
  private config: Required<AIMClientConfig>;
  private reporter: APIReporter;
  private detectors: DetectionMethod[] = [];
  private reportInterval?: NodeJS.Timeout;

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
