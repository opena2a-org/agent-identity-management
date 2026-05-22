package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
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

// LogAction is a convenience method to log a user-initiated action
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
		UserID:         &userID,
		Action:         action,
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		IPAddress:      ipAddress,
		UserAgent:      userAgent,
		Metadata:       metadata,
	}

	return s.auditRepo.Create(log)
}

// LogAgentAction logs an action initiated by an agent (not a user)
// Use this for SDK-initiated actions like attestations
func (s *AuditService) LogAgentAction(
	ctx context.Context,
	orgID uuid.UUID,
	agentID uuid.UUID,
	action domain.AuditAction,
	resourceType string,
	resourceID uuid.UUID,
	ipAddress, userAgent string,
	metadata map[string]interface{},
) error {
	log := &domain.AuditLog{
		OrganizationID: orgID,
		AgentID:        &agentID,
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

// GetUserLogs retrieves audit logs for a specific user, scoped to the
// caller's organization. SECURITY: orgID is mandatory — see
// AuditLogRepository.GetByUser in audit_log.go for the rationale.
func (s *AuditService) GetUserLogs(ctx context.Context, orgID, userID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	return s.auditRepo.GetByUser(orgID, userID, limit, offset)
}

// GetAgentActivity retrieves audit logs for actions performed BY a
// specific agent within the caller's organization (attestations,
// verifications, other agent-initiated actions). SECURITY: orgID
// scope as in GetUserLogs.
func (s *AuditService) GetAgentActivity(ctx context.Context, orgID, agentID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	return s.auditRepo.GetByAgent(orgID, agentID, limit, offset)
}

// GetResourceLogs retrieves audit logs for a specific resource within
// the caller's organization. SECURITY: orgID scope as in GetUserLogs.
func (s *AuditService) GetResourceLogs(ctx context.Context, orgID uuid.UUID, resourceType string, resourceID uuid.UUID) ([]*domain.AuditLog, error) {
	return s.auditRepo.GetByResource(orgID, resourceType, resourceID)
}

// SearchLogs searches audit logs
func (s *AuditService) SearchLogs(ctx context.Context, query string, limit, offset int) ([]*domain.AuditLog, error) {
	return s.auditRepo.Search(query, limit, offset)
}

// GetByID retrieves a single audit log by ID
func (s *AuditService) GetByID(ctx context.Context, id uuid.UUID) (*domain.AuditLog, error) {
	return s.auditRepo.GetByID(id)
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
	// SECURITY: every branch below MUST filter by orgID. The pre-fix
	// implementation passed only the caller-supplied filter (resourceID
	// or userID) into the repo, leaving the SQL as
	// `WHERE resource_type = $1 AND resource_id = $2` /
	// `WHERE user_id = $1` — a system-wide cross-tenant read for any
	// authenticated admin
	// (todo/2026-05-21-a3d-viii-followup-audit-log-query-idor.md).
	// This is the documented sub-variant of the class-#3 lint blindspot
	// "orgID referenced only on default branch" — every branch in a
	// multi-filter service method must propagate orgID.

	// If filtering by specific entity (e.g., MCP server), use GetByResource
	if entityType != "" && entityID != nil {
		logs, err := s.auditRepo.GetByResource(orgID, entityType, *entityID)
		if err != nil {
			return nil, 0, err
		}

		// Apply pagination manually (repository returns all matching)
		total := len(logs)
		if offset >= total {
			return []*domain.AuditLog{}, total, nil
		}
		end := offset + limit
		if end > total {
			end = total
		}
		return logs[offset:end], total, nil
	}

	// If filtering by user
	if userID != nil {
		logs, err := s.auditRepo.GetByUser(orgID, *userID, limit, offset)
		if err != nil {
			return nil, 0, err
		}
		return logs, len(logs), nil
	}

	// Default: return all organization logs
	logs, err := s.auditRepo.GetByOrganization(orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	return logs, len(logs), nil
}
