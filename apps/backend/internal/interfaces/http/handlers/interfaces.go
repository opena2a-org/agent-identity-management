package handlers

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
)

// ===========================
// Service Interfaces for Handlers
// ===========================
// These interfaces define the contracts for services used by handlers.
// Using interfaces enables dependency injection and testability with mocks.

// AgentServicer defines the methods from AgentService that handlers use
type AgentServicer interface {
	CreateAgent(ctx context.Context, req *application.CreateAgentRequest, orgID, userID uuid.UUID, sdkTokenID *uuid.UUID, apiKeyID *uuid.UUID, userEmail string) (*domain.Agent, error)
	GetAgent(ctx context.Context, id uuid.UUID) (*domain.Agent, error)
	GetAgentByName(ctx context.Context, orgID uuid.UUID, name string) (*domain.Agent, error)
	ListAgents(ctx context.Context, orgID uuid.UUID) ([]*domain.Agent, error)
	UpdateAgent(ctx context.Context, id uuid.UUID, req *application.CreateAgentRequest) (*domain.Agent, error)
	DeleteAgent(ctx context.Context, id uuid.UUID) error
	VerifyAgent(ctx context.Context, id uuid.UUID) error
	SuspendAgent(ctx context.Context, id uuid.UUID) error
	ReactivateAgent(ctx context.Context, id uuid.UUID) error
	RecalculateTrustScore(ctx context.Context, id uuid.UUID) (*domain.TrustScore, error)
	UpdateTrustScore(ctx context.Context, agentID uuid.UUID, newScore float64) error
	RotateCredentials(ctx context.Context, id uuid.UUID) (publicKey, privateKey string, err error)
	GetAgentCredentials(ctx context.Context, agentID uuid.UUID) (publicKey, privateKey string, err error)
	UpdateAgentPublicKey(ctx context.Context, agentID uuid.UUID, publicKey string) error
	AddMCPServers(ctx context.Context, agentID uuid.UUID, mcpServerIdentifiers []string) (*domain.Agent, []string, error)
	RemoveMCPServer(ctx context.Context, agentID uuid.UUID, mcpServerIdentifier string) (*domain.Agent, error)
	VerifyCapability(ctx context.Context, agentID uuid.UUID, capability string, resource string, metadata map[string]interface{}, sourceIP string) (allowed bool, reason string, auditID uuid.UUID, err error)
	LogCapabilityResult(ctx context.Context, agentID uuid.UUID, auditID uuid.UUID, success bool, errorMsg string, result map[string]interface{}) error
	HasCapability(ctx context.Context, agentID uuid.UUID, capabilityToCheck string, resource string) (bool, error)
	UpdateLastActive(ctx context.Context, agentID uuid.UUID) error
	DetectMCPServersFromConfig(ctx context.Context, agentID uuid.UUID, req *application.DetectMCPServersRequest, mcpService *application.MCPService, orgID, userID uuid.UUID) (*application.DetectMCPServersResult, error)
}

// MCPServicer defines the methods from MCPService that handlers use
type MCPServicer interface {
	CreateMCPServer(ctx context.Context, req *application.CreateMCPServerRequest, orgID, userID uuid.UUID, agentID *uuid.UUID, sdkTokenID *uuid.UUID, apiKeyID *uuid.UUID) (*domain.MCPServer, error)
	GetMCPServer(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error)
	GetMCPServerByName(ctx context.Context, orgID uuid.UUID, name string) (*domain.MCPServer, error)
	ListMCPServers(ctx context.Context, orgID uuid.UUID) ([]*domain.MCPServer, error)
	UpdateMCPServer(ctx context.Context, id uuid.UUID, req *application.UpdateMCPServerRequest) (*domain.MCPServer, error)
	DeleteMCPServer(ctx context.Context, id uuid.UUID) error
	VerifyMCPServer(ctx context.Context, id uuid.UUID, userID uuid.UUID, userIP string) error
	AddPublicKey(ctx context.Context, serverID uuid.UUID, req *application.AddPublicKeyRequest) error
	GetVerificationStatus(ctx context.Context, id uuid.UUID) (*domain.MCPServerVerificationStatus, error)
	VerifyMCPCapability(ctx context.Context, mcpID uuid.UUID, capability string, resource string, targetService string, metadata map[string]interface{}) (allowed bool, reason string, auditID uuid.UUID, err error)
	GetConnectedAgents(ctx context.Context, mcpServerID uuid.UUID) ([]application.ConnectedAgent, error)
	GetConnectedAgentsCount(ctx context.Context, mcpServerID uuid.UUID) (int, error)
}

// AuditServicer defines the methods from AuditService that handlers use
type AuditServicer interface {
	LogAction(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error
	GetAgentActivity(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error)
	GetAuditLogs(ctx context.Context, orgID uuid.UUID, action string, entityType string, entityID *uuid.UUID, userID *uuid.UUID, startDate *time.Time, endDate *time.Time, limit int, offset int) ([]*domain.AuditLog, int, error)
}

// CapabilityServicer defines the methods from CapabilityService that handlers use
type CapabilityServicer interface {
	VerifyAction(ctx context.Context, agentID uuid.UUID, requestedCapability string, signature []byte, payload []byte, sourceIP *string, metadata map[string]interface{}) (*application.VerificationResult, error)
	GrantCapability(ctx context.Context, agentID uuid.UUID, capabilityType string, scope map[string]interface{}, grantedBy *uuid.UUID) (*domain.AgentCapability, error)
	RevokeCapability(ctx context.Context, capabilityID uuid.UUID, revokedBy *uuid.UUID) error
	GetAgentCapabilities(ctx context.Context, agentID uuid.UUID, activeOnly bool) ([]*domain.AgentCapability, error)
	ListCapabilities(ctx context.Context, orgID uuid.UUID) ([]application.CapabilityDefinition, error)
	ListCapabilitiesWithMetadata(ctx context.Context, orgID uuid.UUID) (*application.ListCapabilitiesResponse, error)
	ValidateAndRegisterCapability(ctx context.Context, capability string, orgID uuid.UUID) error
	GetViolationsByAgent(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error)
	GetViolationsByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error)
	GetRecentViolations(ctx context.Context, orgID uuid.UUID, minutes int) ([]*domain.CapabilityViolation, error)
}

// TagServicer defines the methods from TagService that handlers use
type TagServicer interface {
	GetAgentTags(ctx context.Context, agentID uuid.UUID) ([]*domain.Tag, error)
	GetMCPServerTags(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.Tag, error)
}

// APIKeyServicer defines the methods from APIKeyService that handlers use
type APIKeyServicer interface {
	GenerateAPIKey(ctx context.Context, agentID uuid.UUID, orgID uuid.UUID, userID uuid.UUID, name string, expirationDays int) (fullKey string, record *domain.APIKey, err error)
}

// AlertServicer defines the methods from AlertService that handlers use
type AlertServicer interface {
	CreateAlert(ctx context.Context, alert *domain.Alert) error
	GetAlertsByAgent(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.Alert, error)
}

// VerificationEventServicer defines the methods from VerificationEventService that handlers use
type VerificationEventServicer interface {
	LogVerificationEvent(ctx context.Context, orgID uuid.UUID, agentID uuid.UUID, protocol domain.VerificationProtocol, verificationType domain.VerificationType, status domain.VerificationEventStatus, durationMs int, initiatorType domain.InitiatorType, initiatorID *uuid.UUID, metadata map[string]interface{}) (*domain.VerificationEvent, error)
}

// MCPAttestationServicer defines the methods from MCPAttestationService that handlers use
type MCPAttestationServicer interface {
	GetMCPServersForAgent(ctx context.Context, agentID uuid.UUID) ([]*domain.MCPServer, error)
}
