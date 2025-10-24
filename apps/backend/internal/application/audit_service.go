package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// AuditService handles audit logging
type AuditService struct {
	auditRepo domain.AuditLogRepository
}

// NewAuditService creates a new audit service
func NewAuditService(auditRepo domain.AuditLogRepository) *AuditService {
	return &AuditService{
		auditRepo: auditRepo,
	}
}

// Log creates an audit log entry
func (s *AuditService) Log(ctx context.Context, log *domain.AuditLog) error {
	return s.auditRepo.Create(log)
}

// LogAction is a convenience method to log an action
func (s *AuditService) LogAction(
	ctx context.Context,
	orgID, userID uuid.UUID,
	action domain.AuditAction,
	resourceType string,
	resourceID uuid.UUID,
	ipAddress, userAgent string,
	metadata map[string]interface{},
) error {
	log := &domain.AuditLog{
		OrganizationID: orgID,
		UserID:         userID,
		Action:         action,
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		IPAddress:      ipAddress,
		UserAgent:      userAgent,
		Metadata:       metadata,
	}

	return s.auditRepo.Create(log)
}

// GetLogs retrieves audit logs for an organization
func (s *AuditService) GetLogs(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	return s.auditRepo.GetByOrganization(orgID, limit, offset)
}

// GetUserLogs retrieves audit logs for a specific user
func (s *AuditService) GetUserLogs(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	return s.auditRepo.GetByUser(userID, limit, offset)
}

// GetResourceLogs retrieves audit logs for a specific resource
func (s *AuditService) GetResourceLogs(ctx context.Context, resourceType string, resourceID uuid.UUID) ([]*domain.AuditLog, error) {
	return s.auditRepo.GetByResource(resourceType, resourceID)
}

// SearchLogs searches audit logs
func (s *AuditService) SearchLogs(ctx context.Context, query string, limit, offset int) ([]*domain.AuditLog, error) {
	return s.auditRepo.Search(query, limit, offset)
}

// GetAuditLogs retrieves audit logs with filtering
func (s *AuditService) GetAuditLogs(
	ctx context.Context,
	orgID uuid.UUID,
	action string,
	entityType string,
	entityID *uuid.UUID,
	userID *uuid.UUID,
	startDate *time.Time,
	endDate *time.Time,
	limit int,
	offset int,
) ([]*domain.AuditLog, int, error) {
	// For now, just return organization logs
	// TODO: Implement full filtering in repository layer
	logs, err := s.auditRepo.GetByOrganization(orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	return logs, len(logs), nil
}
