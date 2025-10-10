export interface AIMClientConfig {
  apiUrl: string;
  apiKey: string;
  agentId: string;
  autoDetect?: boolean;
  detectionMethods?: ('import' | 'connection')[];
  reportInterval?: number; // milliseconds, default 10000 (10 seconds)
}

export interface DetectedMCP {
  name: string;
  detectionMethod: 'sdk_import' | 'sdk_connection';
  confidenceScore: number; // 0-100
  details?: Record<string, any>;
}

export interface DetectionEvent {
  mcpServer: string;
  detectionMethod: 'sdk_import' | 'sdk_runtime' | 'direct_api';
  confidence: number; // 0-100
  details?: Record<string, any>;
  sdkVersion?: string;
  timestamp: Date;
}

export interface DetectionReportRequest {
  detections: DetectionEvent[];
}

export interface DetectionReportResponse {
  success: boolean;
  detectionsProcessed: number;
  newMCPs: string[];
  existingMCPs: string[];
  message: string;
}

export interface DetectionMethod {
  start(): void;
  stop(): void;
  getDetections(): DetectedMCP[];
}
