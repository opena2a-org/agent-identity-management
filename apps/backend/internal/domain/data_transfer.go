package domain

import (
	"time"

	"github.com/google/uuid"
)

// DataTransfer represents an individual data transfer record
type DataTransfer struct {
	ID             uuid.UUID              `json:"id"`
	AgentID        uuid.UUID              `json:"agentId"`
	OrganizationID uuid.UUID              `json:"organizationId"`

	// Transfer details
	ActionType        string `json:"actionType"`
	Resource          string `json:"resource,omitempty"`
	DestinationIP     string `json:"destinationIp,omitempty"`
	DestinationDomain string `json:"destinationDomain,omitempty"`

	// Size tracking (in bytes)
	RequestSize  int64 `json:"requestSize"`
	ResponseSize int64 `json:"responseSize"`

	// Metadata
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"createdAt"`
}

// DataTransferAggregate represents hourly aggregated transfer metrics
type DataTransferAggregate struct {
	ID             uuid.UUID `json:"id"`
	AgentID        uuid.UUID `json:"agentId"`
	OrganizationID uuid.UUID `json:"organizationId"`

	// Time bucket
	BucketStart time.Time `json:"bucketStart"`
	BucketEnd   time.Time `json:"bucketEnd"`

	// Aggregated metrics
	TotalRequestBytes  int64 `json:"totalRequestBytes"`
	TotalResponseBytes int64 `json:"totalResponseBytes"`
	TransferCount      int   `json:"transferCount"`
	UniqueDestinations int   `json:"uniqueDestinations"`

	// Top destinations for forensics
	TopDestinations []DestinationSummary `json:"topDestinations"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// DestinationSummary represents a destination with transfer volume
type DestinationSummary struct {
	Domain     string `json:"domain,omitempty"`
	IP         string `json:"ip,omitempty"`
	TotalBytes int64  `json:"totalBytes"`
	Count      int    `json:"count"`
}

// DataTransferRepository defines the interface for data transfer tracking
type DataTransferRepository interface {
	// Record a data transfer
	RecordTransfer(transfer *DataTransfer) error

	// Get recent transfers for an agent
	GetRecentTransfers(agentID uuid.UUID, limit int) ([]*DataTransfer, error)

	// Get transfers within a time window
	GetTransfersInWindow(agentID uuid.UUID, windowMinutes int) ([]*DataTransfer, error)

	// Get cumulative transfer size for an agent within a time window
	GetCumulativeSize(agentID uuid.UUID, windowMinutes int) (requestBytes, responseBytes int64, err error)

	// Get or create aggregate for current hour
	GetOrCreateCurrentAggregate(agentID, orgID uuid.UUID) (*DataTransferAggregate, error)

	// Update aggregate with new transfer
	UpdateAggregate(aggregate *DataTransferAggregate) error

	// Get aggregates for an agent within a time range
	GetAggregates(agentID uuid.UUID, since time.Time) ([]*DataTransferAggregate, error)

	// Get top transferring agents for an organization
	GetTopTransferringAgents(orgID uuid.UUID, windowHours int, limit int) ([]AgentTransferSummary, error)

	// Cleanup old records (for data retention)
	CleanupOldRecords(olderThan time.Duration) (int64, error)
}

// AgentTransferSummary represents an agent's transfer summary
type AgentTransferSummary struct {
	AgentID        uuid.UUID `json:"agentId"`
	AgentName      string    `json:"agentName"`
	TotalBytes     int64     `json:"totalBytes"`
	TransferCount  int       `json:"transferCount"`
	TopDestination string    `json:"topDestination"`
}

// DataExfiltrationConfig holds configuration for data exfiltration detection
type DataExfiltrationConfig struct {
	// Threshold in MB for alerting (default: 100MB per hour)
	ThresholdMB int `json:"thresholdMb"`

	// Time window in minutes for threshold checking (default: 60)
	TimeWindowMins int `json:"timeWindowMins"`

	// Whether to block transfers exceeding threshold
	BlockOnThreshold bool `json:"blockOnThreshold"`

	// Alert severity for threshold breach
	AlertSeverity AlertSeverity `json:"alertSeverity"`
}

// DefaultDataExfiltrationConfig returns the default configuration
func DefaultDataExfiltrationConfig() DataExfiltrationConfig {
	return DataExfiltrationConfig{
		ThresholdMB:      100,  // 100MB per hour
		TimeWindowMins:   60,   // 1 hour window
		BlockOnThreshold: false, // Alert only by default
		AlertSeverity:    AlertSeverityHigh,
	}
}

// BytesToMB converts bytes to megabytes
func BytesToMB(bytes int64) float64 {
	return float64(bytes) / (1024 * 1024)
}

// MBToBytes converts megabytes to bytes
func MBToBytes(mb int) int64 {
	return int64(mb) * 1024 * 1024
}
