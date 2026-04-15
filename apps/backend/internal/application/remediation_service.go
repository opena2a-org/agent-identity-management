package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// RemediationService handles the remediation tracking lifecycle
type RemediationService struct {
	repo domain.RemediationRepository
}

func NewRemediationService(repo domain.RemediationRepository) *RemediationService {
	return &RemediationService{repo: repo}
}

// RecordFinding creates a new remediation record when a finding is detected
func (s *RemediationService) RecordFinding(ctx context.Context, req RecordFindingRequest) (*domain.RemediationRecord, error) {
	// Check if we already have this finding for this agent
	existing, _ := s.repo.GetByFindingAndAgent(req.FindingID, req.AgentIdentifier)
	if existing != nil && existing.Status != domain.RemediationStatusRemediated {
		// Update existing open record
		return existing, nil
	}

	record := &domain.RemediationRecord{
		ID:              uuid.New(),
		FindingID:       req.FindingID,
		Source:          req.Source,
		SourceEventID:   req.SourceEventID,
		AgentIdentifier: req.AgentIdentifier,
		InitialScore:    req.InitialScore,
		InitialSeverity: req.InitialSeverity,
		Status:          domain.RemediationStatusOpen,
		Metadata:        req.Metadata,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	if err := s.repo.Create(record); err != nil {
		return nil, fmt.Errorf("failed to create remediation record: %w", err)
	}

	return record, nil
}

// RecordRemediation updates a finding with rescan results showing improvement
func (s *RemediationService) RecordRemediation(ctx context.Context, req RecordRemediationRequest) (*domain.RemediationRecord, error) {
	record, err := s.repo.GetByFindingAndAgent(req.FindingID, req.AgentIdentifier)
	if err != nil {
		return nil, fmt.Errorf("finding not found: %w", err)
	}

	if err := s.repo.UpdateRemediation(record.ID, req.ScanID, req.RescanScore); err != nil {
		return nil, fmt.Errorf("failed to update remediation: %w", err)
	}

	// Reload
	return s.repo.GetByID(record.ID)
}

// GetStats returns aggregate remediation statistics
func (s *RemediationService) GetStats(ctx context.Context) (*domain.RemediationStats, error) {
	return s.repo.GetStats()
}

// ListRecent returns recent remediation records
func (s *RemediationService) ListRecent(ctx context.Context, limit int) ([]*domain.RemediationRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repo.ListRecent(limit)
}

// RecordFindingRequest is the input for creating a new finding
type RecordFindingRequest struct {
	FindingID       string                 `json:"findingId"`
	Source          string                 `json:"source"`
	SourceEventID   string                 `json:"sourceEventId"`
	AgentIdentifier string                 `json:"agentIdentifier"`
	InitialScore    *int                   `json:"initialScore"`
	InitialSeverity string                 `json:"initialSeverity"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// RecordRemediationRequest is the input for recording a remediation
type RecordRemediationRequest struct {
	FindingID       string `json:"findingId"`
	AgentIdentifier string `json:"agentIdentifier"`
	ScanID          string `json:"scanId"`
	RescanScore     int    `json:"rescanScore"`
}
