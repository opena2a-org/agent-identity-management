import { AIMClientConfig, DetectedMCP, DetectionMethod, DetectionEvent } from './types';
import { ImportDetector } from './detection/import-detector';
import { ConnectionDetector } from './detection/connection-detector';
import { APIReporter } from './reporting/api-reporter';

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
