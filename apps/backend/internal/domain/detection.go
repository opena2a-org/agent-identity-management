package domain

import (
	"time"

	"github.com/google/uuid"
)

// DetectionMethod represents the method used to detect an MCP server
type DetectionMethod string

const (
	DetectionMethodManual       DetectionMethod = "manual"
	DetectionMethodClaudeConfig DetectionMethod = "claude_config"
	DetectionMethodSDKImport    DetectionMethod = "sdk_import"
	DetectionMethodSDKRuntime   DetectionMethod = "sdk_runtime"
	DetectionMethodDirectAPI    DetectionMethod = "direct_api"
)

// AgentMCPDetection represents a detection event stored in the database
type AgentMCPDetection struct {
	ID              uuid.UUID              `json:"id"`
	AgentID         uuid.UUID              `json:"agentId"`
	MCPServerName   string                 `json:"mcpServerName"`
	DetectionMethod DetectionMethod        `json:"detectionMethod"`
	ConfidenceScore float64                `json:"confidenceScore"`
	Details         map[string]interface{} `json:"details,omitempty"`
	SDKVersion      string                 `json:"sdkVersion,omitempty"`
	FirstDetectedAt time.Time              `json:"firstDetectedAt"`
	LastSeenAt      time.Time              `json:"lastSeenAt"`
	CreatedAt       time.Time              `json:"createdAt"`
	UpdatedAt       time.Time              `json:"updatedAt"`
}

// SDKInstallation represents an SDK installation for an agent
type SDKInstallation struct {
	ID                uuid.UUID `json:"id"`
	AgentID           uuid.UUID `json:"agentId"`
	SDKLanguage       string    `json:"sdkLanguage"`
	SDKVersion        string    `json:"sdkVersion"`
	InstalledAt       time.Time `json:"installedAt"`
	LastHeartbeatAt   time.Time `json:"lastHeartbeatAt"`
	AutoDetectEnabled bool      `json:"autoDetectEnabled"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// DetectionReportRequest is the request body for reporting detections
type DetectionReportRequest struct {
	Detections []DetectionEvent `json:"detections"`
}

// DetectionEvent represents a single detection event from SDK or Direct API
type DetectionEvent struct {
	MCPServer       string                 `json:"mcpServer"`
	DetectionMethod DetectionMethod        `json:"detectionMethod"`
	Confidence      float64                `json:"confidence"`
	Details         map[string]interface{} `json:"details,omitempty"`
	SDKVersion      string                 `json:"sdkVersion,omitempty"`
	Timestamp       time.Time              `json:"timestamp"`
}

// DetectionReportResponse is the response after processing detections
type DetectionReportResponse struct {
	Success             bool     `json:"success"`
	DetectionsProcessed int      `json:"detectionsProcessed"`
	NewMCPs             []string `json:"newMCPs"`
	ExistingMCPs        []string `json:"existingMCPs"`
	Message             string   `json:"message"`
}

// DetectionStatusResponse returns the current detection status for an agent
type DetectionStatusResponse struct {
	AgentID           uuid.UUID            `json:"agentId"`
	SDKVersion        string               `json:"sdkVersion,omitempty"`
	SDKInstalled      bool                 `json:"sdkInstalled"`
	AutoDetectEnabled bool                 `json:"autoDetectEnabled"`
	DetectedMCPs      []DetectedMCPSummary `json:"detectedMCPs"`
	LastReportedAt    *time.Time           `json:"lastReportedAt,omitempty"`
}

// DetectedMCPSummary provides a summary of a detected MCP server
type DetectedMCPSummary struct {
	Name            string            `json:"name"`
	ConfidenceScore float64           `json:"confidenceScore"`
	DetectedBy      []DetectionMethod `json:"detectedBy"`
	FirstDetected   time.Time         `json:"firstDetected"`
	LastSeen        time.Time         `json:"lastSeen"`
}
