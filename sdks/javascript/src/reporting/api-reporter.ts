import { DetectionReportRequest, DetectionReportResponse } from '../types';

export class APIReporter {
  private apiUrl: string;
  private apiKey: string;
  private agentId: string;
  private lastReport: Record<string, number> = {}; // MCP name -> timestamp

  constructor(apiUrl: string, apiKey: string, agentId: string) {
    this.apiUrl = apiUrl;
    this.apiKey = apiKey;
    this.agentId = agentId;
  }

  async report(data: DetectionReportRequest): Promise<void> {
    // Deduplicate: Only report if MCP not reported in last 60 seconds
    const now = Date.now();
    const newDetections = data.detections.filter(detection => {
      const lastReported = this.lastReport[detection.mcpServer];
      return !lastReported || now - lastReported > 60000; // 60 seconds
    });

    if (newDetections.length === 0) {
      return;
    }

    try {
      const response = await fetch(
        `${this.apiUrl}/api/v1/detection/agents/${this.agentId}/report`,
        {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${this.apiKey}`,
          },
          body: JSON.stringify({
            detections: newDetections,
          }),
        }
      );

      if (!response.ok) {
        const errorText = await response.text();
        console.error('[AIM SDK] Failed to report detections:', response.status, errorText);
        return;
      }

      const result = await response.json() as DetectionReportResponse;

      // Update last report timestamps
      newDetections.forEach(detection => {
        this.lastReport[detection.mcpServer] = now;
      });

      console.log(`[AIM SDK] Reported ${result.detectionsProcessed} detections successfully`);

    } catch (error) {
      console.error('[AIM SDK] Failed to report detections:', error);
      // Fail silently - don't break agent execution
    }
  }
}
