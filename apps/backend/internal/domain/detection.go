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
	Protocol          string               `json:"protocol,omitempty"` // SDK-detected protocol: "mcp", "a2a", "oauth", etc.
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

// AgentCapabilityReport represents a capability detection report from SDK
type AgentCapabilityReport struct {
	DetectedAt     string                    `json:"detectedAt"`
	Environment    ProgrammingEnvironment    `json:"environment"`
	AIModels       []AIModelUsage            `json:"aiModels"`
	Capabilities   AgentCapabilities         `json:"capabilities"`
	RiskAssessment RiskAssessment            `json:"riskAssessment"`
}

// ProgrammingEnvironment describes the agent's runtime environment
type ProgrammingEnvironment struct {
	Language        string   `json:"language"`
	Version         string   `json:"version"`
	Runtime         string   `json:"runtime"`
	Platform        string   `json:"platform"`
	Arch            string   `json:"arch"`
	Frameworks      []string `json:"frameworks,omitempty"`
	PackageManagers []string `json:"packageManagers,omitempty"`
}

// AIModelUsage describes AI model usage by the agent
type AIModelUsage struct {
	Provider      string   `json:"provider"`
	Models        []string `json:"models"`
	DetectionType string   `json:"detectionType"`
}

// AgentCapabilities contains all detected capabilities
type AgentCapabilities struct {
	FileSystem        *FileSystemCapability        `json:"fileSystem,omitempty"`
	Database          *DatabaseCapability          `json:"database,omitempty"`
	Network           *NetworkCapability           `json:"network,omitempty"`
	CodeExecution     *CodeExecutionCapability     `json:"codeExecution,omitempty"`
	CredentialAccess  *CredentialAccessCapability  `json:"credentialAccess,omitempty"`
	BrowserAutomation *BrowserAutomationCapability `json:"browserAutomation,omitempty"`
}

// FileSystemCapability describes file system operations
type FileSystemCapability struct {
	Read            bool     `json:"read"`
	Write           bool     `json:"write"`
	Delete          bool     `json:"delete"`
	Execute         bool     `json:"execute"`
	PathsAccessed   []string `json:"pathsAccessed,omitempty"`
	DetectionMethod string   `json:"detectionMethod"`
}

// DatabaseCapability describes database operations
type DatabaseCapability struct {
	PostgreSQL      bool     `json:"postgresql"`
	MongoDB         bool     `json:"mongodb"`
	MySQL           bool     `json:"mysql"`
	SQLite          bool     `json:"sqlite"`
	Redis           bool     `json:"redis"`
	Operations      []string `json:"operations,omitempty"`
	DetectionMethod string   `json:"detectionMethod"`
}

// NetworkCapability describes network operations
type NetworkCapability struct {
	HTTP            bool     `json:"http"`
	HTTPS           bool     `json:"https"`
	WebSocket       bool     `json:"websocket"`
	TCP             bool     `json:"tcp"`
	UDP             bool     `json:"udp"`
	ExternalAPIs    []string `json:"externalApis,omitempty"`
	DetectionMethod string   `json:"detectionMethod"`
}

// CodeExecutionCapability describes code execution capabilities
type CodeExecutionCapability struct {
	Eval            bool     `json:"eval"`
	Exec            bool     `json:"exec"`
	ShellCommands   bool     `json:"shellCommands"`
	ChildProcesses  bool     `json:"childProcesses"`
	VMExecution     bool     `json:"vmExecution"`
	DetectionMethod string   `json:"detectionMethod"`
}

// CredentialAccessCapability describes credential access capabilities
type CredentialAccessCapability struct {
	EnvVars         bool     `json:"envVars"`
	ConfigFiles     bool     `json:"configFiles"`
	Keyring         bool     `json:"keyring"`
	CredentialFiles []string `json:"credentialFiles,omitempty"`
	DetectionMethod string   `json:"detectionMethod"`
}

// BrowserAutomationCapability describes browser automation capabilities
type BrowserAutomationCapability struct {
	Puppeteer       bool   `json:"puppeteer"`
	Playwright      bool   `json:"playwright"`
	Selenium        bool   `json:"selenium"`
	DetectionMethod string `json:"detectionMethod"`
}

// RiskAssessment contains risk scoring and security alerts
type RiskAssessment struct {
	OverallRiskScore  int              `json:"overallRiskScore"`
	RiskLevel         string           `json:"riskLevel"`
	TrustScoreImpact  int              `json:"trustScoreImpact"`
	Alerts            []SecurityAlert  `json:"alerts"`
}

// SecurityAlert represents a security concern
type SecurityAlert struct {
	Severity           string `json:"severity"`
	Capability         string `json:"capability"`
	Message            string `json:"message"`
	Recommendation     string `json:"recommendation"`
	TrustScoreImpact   int    `json:"trustScoreImpact"`
}

// CapabilityReportResponse is the response after processing capability report
type CapabilityReportResponse struct {
	Success            bool      `json:"success"`
	AgentID            uuid.UUID `json:"agentId"`
	RiskLevel          string    `json:"riskLevel"`
	TrustScoreImpact   int       `json:"trustScoreImpact"`
	NewTrustScore      float64   `json:"newTrustScore"`
	SecurityAlertsCount int      `json:"securityAlertsCount"`
	Message            string    `json:"message"`
}
