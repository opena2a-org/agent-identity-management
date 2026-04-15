package domain

import (
	"time"

	"github.com/google/uuid"
)

// RemediationStatus represents the lifecycle state of a remediation
type RemediationStatus string

const (
	RemediationStatusOpen       RemediationStatus = "open"
	RemediationStatusInProgress RemediationStatus = "in_progress"
	RemediationStatusRemediated RemediationStatus = "remediated"
	RemediationStatusWontFix    RemediationStatus = "wont_fix"
)

// RemediationRecord tracks the lifecycle of a security finding from detection to fix
type RemediationRecord struct {
	ID                   uuid.UUID              `json:"id"`
	FindingID            string                 `json:"findingId"`
	Source               string                 `json:"source"`
	SourceEventID        string                 `json:"sourceEventId,omitempty"`
	AgentIdentifier      string                 `json:"agentIdentifier,omitempty"`
	InitialScore         *int                   `json:"initialScore,omitempty"`
	InitialSeverity      string                 `json:"initialSeverity,omitempty"`
	RemediationApplied   bool                   `json:"remediationApplied"`
	RemediationScanID    string                 `json:"remediationScanId,omitempty"`
	RescanScore          *int                   `json:"rescanScore,omitempty"`
	ImprovementDelta     *int                   `json:"improvementDelta,omitempty"`
	RemediationDurationH *float64               `json:"remediationDurationHours,omitempty"`
	Status               RemediationStatus      `json:"status"`
	Metadata             map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt            time.Time              `json:"createdAt"`
	UpdatedAt            time.Time              `json:"updatedAt"`
}

// RemediationStats provides aggregate remediation metrics
type RemediationStats struct {
	TotalFindings       int            `json:"totalFindings"`
	Remediated          int            `json:"remediated"`
	InProgress          int            `json:"inProgress"`
	Open                int            `json:"open"`
	WontFix             int            `json:"wontFix"`
	RemediationRate     float64        `json:"remediationRate"`
	AvgRemediationHours float64        `json:"avgRemediationHours"`
	BySource            map[string]int `json:"bySource"`
	BySeverity          map[string]int `json:"bySeverity"`
}

// RemediationRepository defines persistence for remediation records
type RemediationRepository interface {
	Create(record *RemediationRecord) error
	GetByID(id uuid.UUID) (*RemediationRecord, error)
	GetByFindingAndAgent(findingID, agentIdentifier string) (*RemediationRecord, error)
	UpdateRemediation(id uuid.UUID, scanID string, rescanScore int) error
	GetStats() (*RemediationStats, error)
	ListRecent(limit int) ([]*RemediationRecord, error)
}
