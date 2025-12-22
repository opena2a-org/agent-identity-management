package domain

import (
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// DATA CLASSIFICATION
// ============================================================================

// ClassificationCategory represents high-level data classification categories
type ClassificationCategory string

const (
	ClassificationCategoryPII       ClassificationCategory = "PII"
	ClassificationCategoryPHI       ClassificationCategory = "PHI"
	ClassificationCategoryPCI       ClassificationCategory = "PCI"
	ClassificationCategorySensitive ClassificationCategory = "SENSITIVE"
	ClassificationCategoryInternal  ClassificationCategory = "INTERNAL"
	ClassificationCategoryPublic    ClassificationCategory = "PUBLIC"
)

// ClassificationTier represents how the classification was determined
// Lower tier = higher confidence = fewer false positives
type ClassificationTier int

const (
	// ClassificationTierExplicit - Developer explicitly tagged the data (100% confidence)
	ClassificationTierExplicit ClassificationTier = 1
	// ClassificationTierSchema - Inferred from database schema/column names (95% confidence)
	ClassificationTierSchema ClassificationTier = 2
	// ClassificationTierHeuristic - Inferred from naming patterns (80% confidence)
	ClassificationTierHeuristic ClassificationTier = 3
	// ClassificationTierContent - Detected via regex patterns (60-70% confidence, use with caution)
	ClassificationTierContent ClassificationTier = 4
)

// DataClassificationCategory defines a category like PII, PHI, PCI
type DataClassificationCategory struct {
	ID                   uuid.UUID `json:"id"`
	Name                 string    `json:"name"`                 // PII, PHI, PCI
	DisplayName          string    `json:"displayName"`
	Description          string    `json:"description,omitempty"`
	RiskLevel            string    `json:"riskLevel"`            // low, medium, high, critical
	RegulatoryFrameworks []string  `json:"regulatoryFrameworks"` // GDPR, HIPAA, PCI-DSS
	Color                string    `json:"color"`                // Hex color for UI
	Icon                 string    `json:"icon,omitempty"`
	SortOrder            int       `json:"sortOrder"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

// DataClassificationType defines a specific type within a category (e.g., SSN within PII)
type DataClassificationType struct {
	ID                       uuid.UUID `json:"id"`
	CategoryID               uuid.UUID `json:"categoryId"`
	CategoryName             string    `json:"categoryName,omitempty"` // Denormalized for API responses
	Name                     string    `json:"name"`                   // ssn, credit_card, email
	DisplayName              string    `json:"displayName"`
	Description              string    `json:"description,omitempty"`
	ColumnPatterns           []string  `json:"columnPatterns"`           // Regex for column names
	TablePatterns            []string  `json:"tablePatterns"`            // Regex for table names
	ContentPatterns          []string  `json:"contentPatterns"`          // Regex for content (Tier 4)
	ContentPatternConfidence float64   `json:"contentPatternConfidence"` // Confidence when matched by content
	Examples                 []string  `json:"examples,omitempty"`
	IsBuiltin                bool      `json:"isBuiltin"`
	CreatedAt                time.Time `json:"createdAt"`
	UpdatedAt                time.Time `json:"updatedAt"`
}

// ============================================================================
// DATA SOURCES - Where data originates (classification at source)
// ============================================================================

// DataSourceType represents the type of data source
type DataSourceType string

const (
	DataSourceTypeDatabase    DataSourceType = "database"
	DataSourceTypeAPI         DataSourceType = "api"
	DataSourceTypeFile        DataSourceType = "file"
	DataSourceTypeEnvironment DataSourceType = "environment"
	DataSourceTypeUserInput   DataSourceType = "user_input"
	DataSourceTypeManual      DataSourceType = "manual"
)

// DataSource represents a known source of data with its classification
type DataSource struct {
	ID               uuid.UUID              `json:"id"`
	OrganizationID   uuid.UUID              `json:"organizationId"`
	SourceType       DataSourceType         `json:"sourceType"`
	SourceName       string                 `json:"sourceName"`       // customers_db, stripe_api
	SourceIdentifier string                 `json:"sourceIdentifier"` // Full path/connection (hashed)

	// Database-specific fields
	DatabaseType string `json:"databaseType,omitempty"`
	SchemaName   string `json:"schemaName,omitempty"`
	TableName    string `json:"tableName,omitempty"`
	ColumnName   string `json:"columnName,omitempty"`

	// API-specific fields
	APIEndpoint  string `json:"apiEndpoint,omitempty"`
	APIMethod    string `json:"apiMethod,omitempty"`
	ResponsePath string `json:"responsePath,omitempty"` // JSONPath

	// Classification
	ClassificationTier       ClassificationTier `json:"classificationTier"`
	ClassificationTypeID     *uuid.UUID         `json:"classificationTypeId,omitempty"`
	ClassificationTypeName   string             `json:"classificationTypeName,omitempty"`   // Denormalized
	ClassificationCategory   string             `json:"classificationCategory,omitempty"`   // Denormalized
	ClassificationConfidence float64            `json:"classificationConfidence"`

	// Manual override
	IsManuallyClassified bool       `json:"isManuallyClassified"`
	ClassifiedBy         *uuid.UUID `json:"classifiedBy,omitempty"`
	ClassifiedAt         *time.Time `json:"classifiedAt,omitempty"`

	// Metadata
	Description  string                 `json:"description,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	IsActive     bool                   `json:"isActive"`
	DiscoveredAt time.Time              `json:"discoveredAt"`
	LastSeenAt   time.Time              `json:"lastSeenAt"`
	CreatedAt    time.Time              `json:"createdAt"`
	UpdatedAt    time.Time              `json:"updatedAt"`
}

// ============================================================================
// DATA FLOWS - Records of data movement
// ============================================================================

// DataFlowDestinationType represents where data is going
type DataFlowDestinationType string

const (
	DestinationTypeAPI      DataFlowDestinationType = "api"
	DestinationTypeMCP      DataFlowDestinationType = "mcp"
	DestinationTypeLLM      DataFlowDestinationType = "llm"
	DestinationTypeFile     DataFlowDestinationType = "file"
	DestinationTypeDatabase DataFlowDestinationType = "database"
	DestinationTypeResponse DataFlowDestinationType = "response"
	DestinationTypeAgent    DataFlowDestinationType = "agent"
)

// DataFlowStatus represents the status of a data flow
type DataFlowStatus string

const (
	DataFlowStatusInProgress DataFlowStatus = "in_progress"
	DataFlowStatusCompleted  DataFlowStatus = "completed"
	DataFlowStatusBlocked    DataFlowStatus = "blocked"
	DataFlowStatusFailed     DataFlowStatus = "failed"
)

// PolicyAction represents what action to take when a policy matches
type PolicyAction string

const (
	PolicyActionAllow  PolicyAction = "allow"
	PolicyActionBlock  PolicyAction = "block"
	PolicyActionAlert  PolicyAction = "alert"
	PolicyActionRedact PolicyAction = "redact"
)

// DataFlow represents a single data movement event
// Optimized for high-volume time-series storage
type DataFlow struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`

	// Agent context
	AgentID   uuid.UUID `json:"agentId"`
	AgentName string    `json:"agentName"` // Denormalized for performance

	// Flow chain tracking
	FlowID       string `json:"flowId"`                 // Unique ID for this flow chain
	ParentFlowID string `json:"parentFlowId,omitempty"` // Parent flow if this is a hop
	HopNumber    int    `json:"hopNumber"`              // Position in flow chain

	// Source
	SourceID      *uuid.UUID     `json:"sourceId,omitempty"`
	SourceType    DataSourceType `json:"sourceType"`
	SourceName    string         `json:"sourceName"`
	SourceDetails map[string]interface{} `json:"sourceDetails,omitempty"`

	// Destination
	DestinationType     DataFlowDestinationType `json:"destinationType"`
	DestinationName     string                  `json:"destinationName"`
	DestinationEndpoint string                  `json:"destinationEndpoint,omitempty"`
	DestinationDetails  map[string]interface{}  `json:"destinationDetails,omitempty"`

	// Classification (inherited from source)
	ClassificationCategory   string             `json:"classificationCategory,omitempty"`
	ClassificationType       string             `json:"classificationType,omitempty"`
	ClassificationTier       ClassificationTier `json:"classificationTier"`
	ClassificationConfidence float64            `json:"classificationConfidence"`

	// Data metrics (no actual data stored)
	DataSizeBytes  int64    `json:"dataSizeBytes,omitempty"`
	RecordCount    int      `json:"recordCount,omitempty"`
	FieldCount     int      `json:"fieldCount,omitempty"`
	FieldsDetected []string `json:"fieldsDetected,omitempty"`

	// Policy evaluation result
	PolicyID     *uuid.UUID   `json:"policyId,omitempty"`
	PolicyAction PolicyAction `json:"policyAction"`
	PolicyReason string       `json:"policyReason,omitempty"`

	// Request context
	RequestID string `json:"requestId,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
	UserID    string `json:"userId,omitempty"`
	ClientIP  string `json:"clientIp,omitempty"`

	// Verification linkage
	VerificationEventID *uuid.UUID `json:"verificationEventId,omitempty"`
	Capability          string     `json:"capability,omitempty"`

	// Timing
	StartedAt   time.Time  `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	DurationMs  int        `json:"durationMs,omitempty"`

	// Status
	Status       DataFlowStatus `json:"status"`
	ErrorMessage string         `json:"errorMessage,omitempty"`

	// Metadata
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"createdAt"`
}

// ============================================================================
// DATA FLOW POLICIES - Rules for data movement
// ============================================================================

// DataFlowPolicy defines rules for what data can flow where
type DataFlowPolicy struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`

	// Identity
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	// Priority (lower = higher priority, first match wins)
	Priority int `json:"priority"`

	// Conditions - Classification
	ClassificationCategories []string `json:"classificationCategories,omitempty"` // Match these categories
	ClassificationTypes      []string `json:"classificationTypes,omitempty"`      // Match these types
	MinClassificationTier    *int     `json:"minClassificationTier,omitempty"`    // Only tier >= this

	// Conditions - Source
	SourceTypes    []string `json:"sourceTypes,omitempty"`
	SourcePatterns []string `json:"sourcePatterns,omitempty"` // Regex

	// Conditions - Destination
	DestinationTypes    []string `json:"destinationTypes,omitempty"`
	DestinationPatterns []string `json:"destinationPatterns,omitempty"` // Regex
	BlockedDestinations []string `json:"blockedDestinations,omitempty"` // Explicit block list
	AllowedDestinations []string `json:"allowedDestinations,omitempty"` // Whitelist

	// Conditions - Agent
	AgentIDs   []uuid.UUID `json:"agentIds,omitempty"`
	AgentTypes []string    `json:"agentTypes,omitempty"`

	// Action
	Action PolicyAction `json:"action"`

	// Alert configuration
	AlertSeverity  string   `json:"alertSeverity,omitempty"`  // low, medium, high, critical
	AlertMessage   string   `json:"alertMessage,omitempty"`
	NotifyChannels []string `json:"notifyChannels,omitempty"` // slack, email, webhook

	// Redaction configuration
	RedactionStrategy string                 `json:"redactionStrategy,omitempty"` // mask, hash, tokenize
	RedactionConfig   map[string]interface{} `json:"redactionConfig,omitempty"`

	// Metadata
	IsEnabled bool       `json:"isEnabled"`
	IsBuiltin bool       `json:"isBuiltin"`
	CreatedBy *uuid.UUID `json:"createdBy,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// ============================================================================
// DATA FLOW VIOLATIONS
// ============================================================================

// ViolationStatus represents the status of a violation
type ViolationStatus string

const (
	ViolationStatusOpen          ViolationStatus = "open"
	ViolationStatusAcknowledged  ViolationStatus = "acknowledged"
	ViolationStatusResolved      ViolationStatus = "resolved"
	ViolationStatusFalsePositive ViolationStatus = "false_positive"
)

// DataFlowViolation represents a policy violation
type DataFlowViolation struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`

	// References
	DataFlowID uuid.UUID `json:"dataFlowId"`
	PolicyID   uuid.UUID `json:"policyId"`
	AgentID    uuid.UUID `json:"agentId"`

	// Violation details
	ViolationType string `json:"violationType"` // blocked, alerted, redacted
	Severity      string `json:"severity"`

	// What was attempted
	ClassificationCategory string `json:"classificationCategory"`
	ClassificationType     string `json:"classificationType,omitempty"`
	SourceName             string `json:"sourceName"`
	DestinationName        string `json:"destinationName"`
	DestinationEndpoint    string `json:"destinationEndpoint,omitempty"`

	// Action taken
	ActionTaken   string `json:"actionTaken"`
	ActionDetails string `json:"actionDetails,omitempty"`

	// Resolution
	Status          ViolationStatus `json:"status"`
	ResolvedBy      *uuid.UUID      `json:"resolvedBy,omitempty"`
	ResolvedAt      *time.Time      `json:"resolvedAt,omitempty"`
	ResolutionNotes string          `json:"resolutionNotes,omitempty"`

	// Alert linkage
	AlertID *uuid.UUID `json:"alertId,omitempty"`

	// Metadata
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"createdAt"`
}

// ============================================================================
// STATISTICS & AGGREGATIONS (for dashboard efficiency)
// ============================================================================

// DataFlowStatistics represents pre-aggregated hourly stats
type DataFlowStatistics struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`

	// Time bucket
	BucketTime time.Time `json:"bucketTime"`

	// Dimensions
	AgentID                *uuid.UUID `json:"agentId,omitempty"`
	ClassificationCategory string     `json:"classificationCategory,omitempty"`
	DestinationType        string     `json:"destinationType,omitempty"`

	// Metrics
	TotalFlows    int64 `json:"totalFlows"`
	AllowedFlows  int64 `json:"allowedFlows"`
	BlockedFlows  int64 `json:"blockedFlows"`
	AlertedFlows  int64 `json:"alertedFlows"`
	RedactedFlows int64 `json:"redactedFlows"`

	TotalBytes   int64 `json:"totalBytes"`
	TotalRecords int64 `json:"totalRecords"`

	AvgDurationMs float64 `json:"avgDurationMs"`
	P95DurationMs float64 `json:"p95DurationMs"`

	UniqueDestinations int `json:"uniqueDestinations"`
	UniqueSources      int `json:"uniqueSources"`

	ComputedAt time.Time `json:"computedAt"`
}

// DataFlowSummary provides a high-level overview for dashboards
type DataFlowSummary struct {
	TotalFlows           int64   `json:"totalFlows"`
	FlowsLast24h         int64   `json:"flowsLast24h"`
	BlockedFlows         int64   `json:"blockedFlows"`
	AlertedFlows         int64   `json:"alertedFlows"`
	PIIFlows             int64   `json:"piiFlows"`
	PHIFlows             int64   `json:"phiFlows"`
	PCIFlows             int64   `json:"pciFlows"`
	UniqueAgents         int     `json:"uniqueAgents"`
	UniqueDestinations   int     `json:"uniqueDestinations"`
	UniqueSources        int     `json:"uniqueSources"`
	AvgFlowsPerHour      float64 `json:"avgFlowsPerHour"`
	OpenViolations       int     `json:"openViolations"`
	CriticalViolations   int     `json:"criticalViolations"`
}

// ============================================================================
// REPOSITORY INTERFACES
// ============================================================================

// DataFlowRepository defines the interface for data flow persistence
type DataFlowRepository interface {
	// Data Flows
	CreateFlow(flow *DataFlow) error
	GetFlowByID(id uuid.UUID) (*DataFlow, error)
	GetFlowsByOrganization(orgID uuid.UUID, limit, offset int) ([]*DataFlow, int, error)
	GetFlowsByAgent(agentID uuid.UUID, limit, offset int) ([]*DataFlow, int, error)
	GetFlowsByClassification(orgID uuid.UUID, category, dataType string, limit, offset int) ([]*DataFlow, int, error)
	GetFlowChain(flowID string) ([]*DataFlow, error)
	UpdateFlowStatus(id uuid.UUID, status DataFlowStatus, completedAt time.Time, durationMs int) error
	GetRecentFlows(orgID uuid.UUID, minutes int) ([]*DataFlow, error)

	// Data Sources
	CreateSource(source *DataSource) error
	GetSourceByID(id uuid.UUID) (*DataSource, error)
	GetSourceByIdentifier(orgID uuid.UUID, sourceType DataSourceType, identifier string) (*DataSource, error)
	GetSourcesByOrganization(orgID uuid.UUID, limit, offset int) ([]*DataSource, int, error)
	UpdateSourceClassification(id uuid.UUID, typeID uuid.UUID, tier ClassificationTier, confidence float64) error
	UpdateSourceLastSeen(id uuid.UUID) error

	// Policies
	CreatePolicy(policy *DataFlowPolicy) error
	GetPolicyByID(id uuid.UUID) (*DataFlowPolicy, error)
	GetPoliciesByOrganization(orgID uuid.UUID) ([]*DataFlowPolicy, error)
	GetEnabledPoliciesByPriority(orgID uuid.UUID) ([]*DataFlowPolicy, error)
	UpdatePolicy(policy *DataFlowPolicy) error
	DeletePolicy(id uuid.UUID) error

	// Violations
	CreateViolation(violation *DataFlowViolation) error
	GetViolationByID(id uuid.UUID) (*DataFlowViolation, error)
	GetViolationsByOrganization(orgID uuid.UUID, status ViolationStatus, limit, offset int) ([]*DataFlowViolation, int, error)
	UpdateViolationStatus(id uuid.UUID, status ViolationStatus, resolvedBy *uuid.UUID, notes string) error

	// Statistics
	GetSummary(orgID uuid.UUID) (*DataFlowSummary, error)
	GetStatisticsByTimeRange(orgID uuid.UUID, start, end time.Time) ([]*DataFlowStatistics, error)
	AggregateStatistics(orgID uuid.UUID, bucketTime time.Time) error

	// Classification Types
	GetClassificationCategories() ([]*DataClassificationCategory, error)
	GetClassificationTypes(categoryID *uuid.UUID) ([]*DataClassificationType, error)
	GetClassificationTypeByName(category, typeName string) (*DataClassificationType, error)
}
