import { DetectionReportRequest, DetectionReportResponse } from '../types';

export class APIReporter {
  private apiUrl: string;
  private apiKey: string;
  private agentId: string;

  constructor(apiUrl: string, apiKey: string, agentId: string) {
    this.apiUrl = apiUrl;
    this.apiKey = apiKey;
    this.agentId = agentId;
  }

  async report(data: DetectionReportRequest): Promise<void> {
    // Always report detections - let server decide if they're significant
    // This ensures full audit trail and allows server-side analytics
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
            detections: data.detections,
          }),
        }
      );

      if (!response.ok) {
        const errorText = await response.text();
        console.error('[AIM SDK] Failed to report detections:', response.status, errorText);
        return;
      }

      const result = await response.json() as DetectionReportResponse;
      console.log(`[AIM SDK] Reported ${result.detectionsProcessed} detections successfully`);

    } catch (error) {
      console.error('[AIM SDK] Failed to report detections:', error);
      // Fail silently - don't break agent execution
    }
  }
}
