package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/opena2a/identity/backend/internal/infrastructure/repository"
)

type SecurityService struct {
	securityRepo *repository.SecurityRepository
	agentRepo    *repository.AgentRepository
}

func NewSecurityService(
	securityRepo *repository.SecurityRepository,
	agentRepo *repository.AgentRepository,
) *SecurityService {
	return &SecurityService{
		securityRepo: securityRepo,
		agentRepo:    agentRepo,
	}
}

// GetThreats retrieves security threats
func (s *SecurityService) GetThreats(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Threat, error) {
	return s.securityRepo.GetThreats(orgID, limit, offset)
}

// GetAnomalies retrieves detected anomalies
func (s *SecurityService) GetAnomalies(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Anomaly, error) {
	return s.securityRepo.GetAnomalies(orgID, limit, offset)
}

// GetSecurityMetrics retrieves overall security metrics
func (s *SecurityService) GetSecurityMetrics(ctx context.Context, orgID uuid.UUID) (*domain.SecurityMetrics, error) {
	return s.securityRepo.GetSecurityMetrics(orgID)
}

// RunSecurityScan initiates a security scan
func (s *SecurityService) RunSecurityScan(ctx context.Context, orgID uuid.UUID, scanType string) (*domain.SecurityScanResult, error) {
	scan := &domain.SecurityScanResult{
		ScanID:         uuid.New(),
		OrganizationID: orgID,
		ScanType:       scanType,
		Status:         "running",
		StartedAt:      time.Now().UTC(),
	}

	// Create scan record
	if err := s.securityRepo.CreateSecurityScan(scan); err != nil {
		return nil, err
	}

	// Perform scan asynchronously (in production, this would be a background job)
	go s.performSecurityScan(scan)

	return scan, nil
}

// performSecurityScan performs the actual security scanning
func (s *SecurityService) performSecurityScan(scan *domain.SecurityScanResult) {
	// TODO: Implement actual security scanning logic
	// For now, we'll simulate a scan

	// Get all agents for the organization
	agents, _ := s.agentRepo.GetByOrganization(scan.OrganizationID)

	threatsFound := 0
	anomaliesFound := 0
	vulnerabilitiesFound := 0

	// Check for low trust scores (potential threats)
	for _, agent := range agents {
		if agent.TrustScore < 50 {
			threatsFound++
		}
		if agent.TrustScore < 70 && agent.TrustScore >= 50 {
			anomaliesFound++
		}
	}

	// Calculate security score
	securityScore := 100.0
	if len(agents) > 0 {
		avgTrustScore := 0.0
		for _, agent := range agents {
			avgTrustScore += agent.TrustScore
		}
		avgTrustScore /= float64(len(agents))
		securityScore = avgTrustScore
	}

	// Update scan results
	scan.ThreatsFound = threatsFound
	scan.AnomaliesFound = anomaliesFound
	scan.VulnerabilitiesFound = vulnerabilitiesFound
	scan.SecurityScore = securityScore
	scan.Status = "completed"
	completedAt := time.Now().UTC()
	scan.CompletedAt = &completedAt
}

// GetSecurityScan retrieves a security scan by ID
func (s *SecurityService) GetSecurityScan(ctx context.Context, scanID uuid.UUID) (*domain.SecurityScanResult, error) {
	return s.securityRepo.GetSecurityScan(scanID)
}

// GetIncidents retrieves security incidents
func (s *SecurityService) GetIncidents(ctx context.Context, orgID uuid.UUID, status domain.IncidentStatus, limit, offset int) ([]*domain.SecurityIncident, error) {
	return s.securityRepo.GetIncidents(orgID, status, limit, offset)
}

// ResolveIncident marks a security incident as resolved
func (s *SecurityService) ResolveIncident(ctx context.Context, incidentID uuid.UUID, resolvedBy uuid.UUID, notes string) error {
	return s.securityRepo.UpdateIncidentStatus(incidentID, domain.IncidentStatusResolved, &resolvedBy, notes)
}

// CreateThreat creates a new security threat
func (s *SecurityService) CreateThreat(ctx context.Context, threat *domain.Threat) error {
	threat.ID = uuid.New()
	threat.CreatedAt = time.Now().UTC()
	return s.securityRepo.CreateThreat(threat)
}

// CreateAnomaly creates a new anomaly
func (s *SecurityService) CreateAnomaly(ctx context.Context, anomaly *domain.Anomaly) error {
	anomaly.ID = uuid.New()
	anomaly.CreatedAt = time.Now().UTC()
	return s.securityRepo.CreateAnomaly(anomaly)
}

// CreateIncident creates a new security incident
func (s *SecurityService) CreateIncident(ctx context.Context, incident *domain.SecurityIncident) error {
	incident.ID = uuid.New()
	incident.CreatedAt = time.Now().UTC()
	incident.UpdatedAt = time.Now().UTC()
	return s.securityRepo.CreateIncident(incident)
}

// BlockThreat blocks a security threat
func (s *SecurityService) BlockThreat(ctx context.Context, threatID uuid.UUID) error {
	return s.securityRepo.BlockThreat(threatID)
}
