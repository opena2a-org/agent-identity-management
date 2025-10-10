package aimsdk

import "time"

// DetectedMCP represents a detected MCP server
type DetectedMCP struct {
	Name            string                 `json:"name"`
	DetectionMethod string                 `json:"detectionMethod"`
	ConfidenceScore float64                `json:"confidenceScore"`
	Details         map[string]interface{} `json:"details,omitempty"`
}

// DetectionEvent represents a single detection event
type DetectionEvent struct {
	MCPServer       string                 `json:"mcpServer"`
	DetectionMethod string                 `json:"detectionMethod"`
	Confidence      float64                `json:"confidence"`
	Details         map[string]interface{} `json:"details,omitempty"`
	SDKVersion      string                 `json:"sdkVersion,omitempty"`
	Timestamp       time.Time              `json:"timestamp"`
}

// DetectionReportRequest is sent to AIM API
type DetectionReportRequest struct {
	Detections []DetectionEvent `json:"detections"`
}

// DetectionReportResponse is the response from AIM API
type DetectionReportResponse struct {
	Success             bool     `json:"success"`
	DetectionsProcessed int      `json:"detectionsProcessed"`
	NewMCPs             []string `json:"newMCPs"`
	ExistingMCPs        []string `json:"existingMCPs"`
	Message             string   `json:"message"`
}
