package domain

import (
	"time"

	"github.com/google/uuid"
)

// ComplianceFramework represents a compliance framework
type ComplianceFramework string

const (
	FrameworkAIM ComplianceFramework = "aim" // AIM-specific compliance
)

// EvidenceType represents the type of compliance evidence
type EvidenceType string

const (
	EvidenceTypeAuditLog       EvidenceType = "audit_log"
	EvidenceTypeConfiguration  EvidenceType = "configuration"
	EvidenceTypeScreenshot     EvidenceType = "screenshot"
	EvidenceTypeDocument       EvidenceType = "document"
	EvidenceTypeAttestation    EvidenceType = "attestation"
	EvidenceTypeSystemCheck    EvidenceType = "system_check"
	EvidenceTypeAccessReview   EvidenceType = "access_review"
	EvidenceTypeSecurityScan   EvidenceType = "security_scan"
)

// ComplianceEvidence represents evidence collected for compliance audits
type ComplianceEvidence struct {
	ID              uuid.UUID              `json:"id"`
	OrganizationID  uuid.UUID              `json:"organizationId"`
	Framework       ComplianceFramework    `json:"framework"`
	CheckName       string                 `json:"checkName"`       // Which compliance check this evidence supports
	EvidenceType    EvidenceType           `json:"evidenceType"`
	Title           string                 `json:"title"`
	Description     string                 `json:"description"`
	Data            map[string]interface{} `json:"data"`            // Structured evidence data
	FileURL         string                 `json:"fileUrl,omitempty"` // Optional file attachment
	CollectedAt     time.Time              `json:"collectedAt"`
	CollectedBy     uuid.UUID              `json:"collectedBy"`     // User or system that collected
	IsAutomatic     bool                   `json:"isAutomatic"`     // Auto-collected vs manual
	ValidUntil      *time.Time             `json:"validUntil,omitempty"` // Evidence expiration
	CreatedAt       time.Time              `json:"createdAt"`
}

// ComplianceSnapshot represents a point-in-time compliance score
type ComplianceSnapshot struct {
	ID              uuid.UUID           `json:"id"`
	OrganizationID  uuid.UUID           `json:"organizationId"`
	Framework       ComplianceFramework `json:"framework"`
	Score           float64             `json:"score"`           // 0-100
	PassedChecks    int                 `json:"passedChecks"`
	FailedChecks    int                 `json:"failedChecks"`
	TotalChecks     int                 `json:"totalChecks"`
	CheckResults    map[string]bool     `json:"checkResults"`    // Individual check results
	SnapshotDate    time.Time           `json:"snapshotDate"`
	CreatedAt       time.Time           `json:"createdAt"`
}

// ComplianceEvidenceRepository defines the interface for evidence persistence
type ComplianceEvidenceRepository interface {
	Create(evidence *ComplianceEvidence) error
	GetByID(id uuid.UUID) (*ComplianceEvidence, error)
	GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*ComplianceEvidence, error)
	GetByFramework(orgID uuid.UUID, framework ComplianceFramework) ([]*ComplianceEvidence, error)
	GetByCheckName(orgID uuid.UUID, checkName string) ([]*ComplianceEvidence, error)
	GetRecent(orgID uuid.UUID, since time.Time) ([]*ComplianceEvidence, error)
	Delete(id uuid.UUID) error
	DeleteExpired(orgID uuid.UUID) (int, error)
}

// ComplianceSnapshotRepository defines the interface for snapshot persistence
type ComplianceSnapshotRepository interface {
	Create(snapshot *ComplianceSnapshot) error
	GetByID(id uuid.UUID) (*ComplianceSnapshot, error)
	GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*ComplianceSnapshot, error)
	GetByFramework(orgID uuid.UUID, framework ComplianceFramework, limit int) ([]*ComplianceSnapshot, error)
	GetTrending(orgID uuid.UUID, framework ComplianceFramework, startDate, endDate time.Time) ([]*ComplianceSnapshot, error)
	GetLatest(orgID uuid.UUID, framework ComplianceFramework) (*ComplianceSnapshot, error)
	DeleteOlderThan(orgID uuid.UUID, before time.Time) (int, error)
}

// ComplianceCheckResult represents a detailed compliance check result
type ComplianceCheckResult struct {
	CheckName       string                 `json:"checkName"`
	Category        string                 `json:"category"`
	Passed          bool                   `json:"passed"`
	Severity        string                 `json:"severity"`        // critical, high, medium, low
	Details         string                 `json:"details"`
	AffectedCount   int                    `json:"affectedCount"`
	AffectedItems   []map[string]interface{} `json:"affectedItems,omitempty"`
	ActionURL       string                 `json:"actionUrl,omitempty"`
	Evidence        []ComplianceEvidence   `json:"evidence,omitempty"`
	LastChecked     time.Time              `json:"lastChecked"`
}

// ComplianceReport represents a comprehensive compliance report for export
type ComplianceExportReport struct {
	ID               uuid.UUID                `json:"id"`
	OrganizationID   uuid.UUID                `json:"organizationId"`
	OrganizationName string                   `json:"organizationName"`
	Framework        ComplianceFramework      `json:"framework"`
	ReportPeriod     ReportPeriod             `json:"reportPeriod"`
	GeneratedAt      time.Time                `json:"generatedAt"`
	GeneratedBy      uuid.UUID                `json:"generatedBy"`

	// Summary
	OverallScore     float64                  `json:"overallScore"`
	ComplianceStatus string                   `json:"complianceStatus"` // compliant, partial, non_compliant

	// Detailed Results
	CheckResults     []ComplianceCheckResult  `json:"checkResults"`

	// Evidence
	EvidenceItems    []ComplianceEvidence     `json:"evidenceItems"`

	// Trending
	ScoreTrend       []ComplianceSnapshot     `json:"scoreTrend"`

	// Metadata
	Metadata         map[string]interface{}   `json:"metadata"`
}

// ReportPeriod represents the time period for a report
type ReportPeriod struct {
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`
}
