package handlers

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
)

// ===========================
// Mock Implementations
// ===========================
// These mocks implement the interfaces defined in interfaces.go

// MockAgentServiceImpl implements AgentServicer interface
type MockAgentServiceImpl struct {
	CreateAgentFunc                 func(ctx context.Context, req *application.CreateAgentRequest, orgID, userID uuid.UUID, sdkTokenID *uuid.UUID, apiKeyID *uuid.UUID, userEmail string) (*domain.Agent, error)
	GetAgentFunc                    func(ctx context.Context, id uuid.UUID) (*domain.Agent, error)
	GetAgentByNameFunc              func(ctx context.Context, orgID uuid.UUID, name string) (*domain.Agent, error)
	ListAgentsFunc                  func(ctx context.Context, orgID uuid.UUID) ([]*domain.Agent, error)
	UpdateAgentFunc                 func(ctx context.Context, id uuid.UUID, req *application.CreateAgentRequest) (*domain.Agent, error)
	DeleteAgentFunc                 func(ctx context.Context, id uuid.UUID) error
	VerifyAgentFunc                 func(ctx context.Context, id uuid.UUID) error
	SuspendAgentFunc                func(ctx context.Context, id uuid.UUID) error
	ReactivateAgentFunc             func(ctx context.Context, id uuid.UUID) error
	RecalculateTrustScoreFunc       func(ctx context.Context, id uuid.UUID) (*domain.TrustScore, error)
	UpdateTrustScoreFunc            func(ctx context.Context, agentID uuid.UUID, newScore float64) error
	RotateCredentialsFunc           func(ctx context.Context, id uuid.UUID) (publicKey, privateKey string, err error)
	GetAgentCredentialsFunc         func(ctx context.Context, agentID uuid.UUID) (publicKey, privateKey string, err error)
	UpdateAgentPublicKeyFunc        func(ctx context.Context, agentID uuid.UUID, publicKey string) error
	AddMCPServersFunc               func(ctx context.Context, agentID uuid.UUID, mcpServerIdentifiers []string) (*domain.Agent, []string, error)
	RemoveMCPServerFunc             func(ctx context.Context, agentID uuid.UUID, mcpServerIdentifier string) (*domain.Agent, error)
	VerifyCapabilityFunc            func(ctx context.Context, agentID uuid.UUID, capability string, resource string, metadata map[string]interface{}, sourceIP string) (allowed bool, reason string, auditID uuid.UUID, err error)
	LogCapabilityResultFunc         func(ctx context.Context, agentID uuid.UUID, auditID uuid.UUID, success bool, errorMsg string, result map[string]interface{}) error
	HasCapabilityFunc               func(ctx context.Context, agentID uuid.UUID, capabilityToCheck string, resource string) (bool, error)
	UpdateLastActiveFunc            func(ctx context.Context, agentID uuid.UUID) error
	DetectMCPServersFromConfigFunc  func(ctx context.Context, agentID uuid.UUID, req *application.DetectMCPServersRequest, mcpService *application.MCPService, orgID, userID uuid.UUID) (*application.DetectMCPServersResult, error)
}

func (m *MockAgentServiceImpl) CreateAgent(ctx context.Context, req *application.CreateAgentRequest, orgID, userID uuid.UUID, sdkTokenID *uuid.UUID, apiKeyID *uuid.UUID, userEmail string) (*domain.Agent, error) {
	if m.CreateAgentFunc != nil {
		return m.CreateAgentFunc(ctx, req, orgID, userID, sdkTokenID, apiKeyID, userEmail)
	}
	return &domain.Agent{ID: uuid.New(), OrganizationID: orgID, Name: req.Name}, nil
}

func (m *MockAgentServiceImpl) GetAgent(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
	if m.GetAgentFunc != nil {
		return m.GetAgentFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockAgentServiceImpl) GetAgentByName(ctx context.Context, orgID uuid.UUID, name string) (*domain.Agent, error) {
	if m.GetAgentByNameFunc != nil {
		return m.GetAgentByNameFunc(ctx, orgID, name)
	}
	return nil, nil
}

func (m *MockAgentServiceImpl) ListAgents(ctx context.Context, orgID uuid.UUID) ([]*domain.Agent, error) {
	if m.ListAgentsFunc != nil {
		return m.ListAgentsFunc(ctx, orgID)
	}
	return []*domain.Agent{}, nil
}

func (m *MockAgentServiceImpl) UpdateAgent(ctx context.Context, id uuid.UUID, req *application.CreateAgentRequest) (*domain.Agent, error) {
	if m.UpdateAgentFunc != nil {
		return m.UpdateAgentFunc(ctx, id, req)
	}
	return nil, nil
}

func (m *MockAgentServiceImpl) DeleteAgent(ctx context.Context, id uuid.UUID) error {
	if m.DeleteAgentFunc != nil {
		return m.DeleteAgentFunc(ctx, id)
	}
	return nil
}

func (m *MockAgentServiceImpl) VerifyAgent(ctx context.Context, id uuid.UUID) error {
	if m.VerifyAgentFunc != nil {
		return m.VerifyAgentFunc(ctx, id)
	}
	return nil
}

func (m *MockAgentServiceImpl) SuspendAgent(ctx context.Context, id uuid.UUID) error {
	if m.SuspendAgentFunc != nil {
		return m.SuspendAgentFunc(ctx, id)
	}
	return nil
}

func (m *MockAgentServiceImpl) ReactivateAgent(ctx context.Context, id uuid.UUID) error {
	if m.ReactivateAgentFunc != nil {
		return m.ReactivateAgentFunc(ctx, id)
	}
	return nil
}

func (m *MockAgentServiceImpl) RecalculateTrustScore(ctx context.Context, id uuid.UUID) (*domain.TrustScore, error) {
	if m.RecalculateTrustScoreFunc != nil {
		return m.RecalculateTrustScoreFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockAgentServiceImpl) UpdateTrustScore(ctx context.Context, agentID uuid.UUID, newScore float64) error {
	if m.UpdateTrustScoreFunc != nil {
		return m.UpdateTrustScoreFunc(ctx, agentID, newScore)
	}
	return nil
}

func (m *MockAgentServiceImpl) RotateCredentials(ctx context.Context, id uuid.UUID) (publicKey, privateKey string, err error) {
	if m.RotateCredentialsFunc != nil {
		return m.RotateCredentialsFunc(ctx, id)
	}
	return "mockPublicKey", "mockPrivateKey", nil
}

func (m *MockAgentServiceImpl) GetAgentCredentials(ctx context.Context, agentID uuid.UUID) (publicKey, privateKey string, err error) {
	if m.GetAgentCredentialsFunc != nil {
		return m.GetAgentCredentialsFunc(ctx, agentID)
	}
	return "mockPublicKey", "mockPrivateKey", nil
}

func (m *MockAgentServiceImpl) UpdateAgentPublicKey(ctx context.Context, agentID uuid.UUID, publicKey string) error {
	if m.UpdateAgentPublicKeyFunc != nil {
		return m.UpdateAgentPublicKeyFunc(ctx, agentID, publicKey)
	}
	return nil
}

func (m *MockAgentServiceImpl) AddMCPServers(ctx context.Context, agentID uuid.UUID, mcpServerIdentifiers []string) (*domain.Agent, []string, error) {
	if m.AddMCPServersFunc != nil {
		return m.AddMCPServersFunc(ctx, agentID, mcpServerIdentifiers)
	}
	return &domain.Agent{ID: agentID, TalksTo: mcpServerIdentifiers}, mcpServerIdentifiers, nil
}

func (m *MockAgentServiceImpl) RemoveMCPServer(ctx context.Context, agentID uuid.UUID, mcpServerIdentifier string) (*domain.Agent, error) {
	if m.RemoveMCPServerFunc != nil {
		return m.RemoveMCPServerFunc(ctx, agentID, mcpServerIdentifier)
	}
	return &domain.Agent{ID: agentID, TalksTo: []string{}}, nil
}

func (m *MockAgentServiceImpl) VerifyCapability(ctx context.Context, agentID uuid.UUID, capability string, resource string, metadata map[string]interface{}, sourceIP string) (allowed bool, reason string, auditID uuid.UUID, err error) {
	if m.VerifyCapabilityFunc != nil {
		return m.VerifyCapabilityFunc(ctx, agentID, capability, resource, metadata, sourceIP)
	}
	return true, "allowed", uuid.New(), nil
}

func (m *MockAgentServiceImpl) LogCapabilityResult(ctx context.Context, agentID uuid.UUID, auditID uuid.UUID, success bool, errorMsg string, result map[string]interface{}) error {
	if m.LogCapabilityResultFunc != nil {
		return m.LogCapabilityResultFunc(ctx, agentID, auditID, success, errorMsg, result)
	}
	return nil
}

func (m *MockAgentServiceImpl) HasCapability(ctx context.Context, agentID uuid.UUID, capabilityToCheck string, resource string) (bool, error) {
	if m.HasCapabilityFunc != nil {
		return m.HasCapabilityFunc(ctx, agentID, capabilityToCheck, resource)
	}
	return true, nil
}

func (m *MockAgentServiceImpl) UpdateLastActive(ctx context.Context, agentID uuid.UUID) error {
	if m.UpdateLastActiveFunc != nil {
		return m.UpdateLastActiveFunc(ctx, agentID)
	}
	return nil
}

func (m *MockAgentServiceImpl) DetectMCPServersFromConfig(ctx context.Context, agentID uuid.UUID, req *application.DetectMCPServersRequest, mcpService *application.MCPService, orgID, userID uuid.UUID) (*application.DetectMCPServersResult, error) {
	if m.DetectMCPServersFromConfigFunc != nil {
		return m.DetectMCPServersFromConfigFunc(ctx, agentID, req, mcpService, orgID, userID)
	}
	return &application.DetectMCPServersResult{}, nil
}

// MockMCPServiceImpl implements MCPServicer interface
type MockMCPServiceImpl struct {
	CreateMCPServerFunc         func(ctx context.Context, req *application.CreateMCPServerRequest, orgID, userID uuid.UUID, agentID *uuid.UUID, sdkTokenID *uuid.UUID, apiKeyID *uuid.UUID) (*domain.MCPServer, error)
	GetMCPServerFunc            func(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error)
	GetMCPServerByNameFunc      func(ctx context.Context, orgID uuid.UUID, name string) (*domain.MCPServer, error)
	ListMCPServersFunc          func(ctx context.Context, orgID uuid.UUID) ([]*domain.MCPServer, error)
	UpdateMCPServerFunc         func(ctx context.Context, id uuid.UUID, req *application.UpdateMCPServerRequest) (*domain.MCPServer, error)
	DeleteMCPServerFunc         func(ctx context.Context, id uuid.UUID) error
	VerifyMCPServerFunc         func(ctx context.Context, id uuid.UUID, userID uuid.UUID, userIP string) error
	AddPublicKeyFunc            func(ctx context.Context, serverID uuid.UUID, req *application.AddPublicKeyRequest) error
	GetVerificationStatusFunc   func(ctx context.Context, id uuid.UUID) (*domain.MCPServerVerificationStatus, error)
	VerifyMCPCapabilityFunc     func(ctx context.Context, mcpID uuid.UUID, capability string, resource string, targetService string, metadata map[string]interface{}) (allowed bool, reason string, auditID uuid.UUID, err error)
	GetConnectedAgentsFunc      func(ctx context.Context, mcpServerID uuid.UUID) ([]application.ConnectedAgent, error)
	GetConnectedAgentsCountFunc func(ctx context.Context, mcpServerID uuid.UUID) (int, error)
}

func (m *MockMCPServiceImpl) CreateMCPServer(ctx context.Context, req *application.CreateMCPServerRequest, orgID, userID uuid.UUID, agentID *uuid.UUID, sdkTokenID *uuid.UUID, apiKeyID *uuid.UUID) (*domain.MCPServer, error) {
	if m.CreateMCPServerFunc != nil {
		return m.CreateMCPServerFunc(ctx, req, orgID, userID, agentID, sdkTokenID, apiKeyID)
	}
	return &domain.MCPServer{ID: uuid.New(), OrganizationID: orgID, Name: req.Name}, nil
}

func (m *MockMCPServiceImpl) GetMCPServer(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error) {
	if m.GetMCPServerFunc != nil {
		return m.GetMCPServerFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockMCPServiceImpl) GetMCPServerByName(ctx context.Context, orgID uuid.UUID, name string) (*domain.MCPServer, error) {
	if m.GetMCPServerByNameFunc != nil {
		return m.GetMCPServerByNameFunc(ctx, orgID, name)
	}
	return nil, nil
}

func (m *MockMCPServiceImpl) ListMCPServers(ctx context.Context, orgID uuid.UUID) ([]*domain.MCPServer, error) {
	if m.ListMCPServersFunc != nil {
		return m.ListMCPServersFunc(ctx, orgID)
	}
	return []*domain.MCPServer{}, nil
}

func (m *MockMCPServiceImpl) UpdateMCPServer(ctx context.Context, id uuid.UUID, req *application.UpdateMCPServerRequest) (*domain.MCPServer, error) {
	if m.UpdateMCPServerFunc != nil {
		return m.UpdateMCPServerFunc(ctx, id, req)
	}
	return nil, nil
}

func (m *MockMCPServiceImpl) DeleteMCPServer(ctx context.Context, id uuid.UUID) error {
	if m.DeleteMCPServerFunc != nil {
		return m.DeleteMCPServerFunc(ctx, id)
	}
	return nil
}

func (m *MockMCPServiceImpl) VerifyMCPServer(ctx context.Context, id uuid.UUID, userID uuid.UUID, userIP string) error {
	if m.VerifyMCPServerFunc != nil {
		return m.VerifyMCPServerFunc(ctx, id, userID, userIP)
	}
	return nil
}

func (m *MockMCPServiceImpl) AddPublicKey(ctx context.Context, serverID uuid.UUID, req *application.AddPublicKeyRequest) error {
	if m.AddPublicKeyFunc != nil {
		return m.AddPublicKeyFunc(ctx, serverID, req)
	}
	return nil
}

func (m *MockMCPServiceImpl) GetVerificationStatus(ctx context.Context, id uuid.UUID) (*domain.MCPServerVerificationStatus, error) {
	if m.GetVerificationStatusFunc != nil {
		return m.GetVerificationStatusFunc(ctx, id)
	}
	return &domain.MCPServerVerificationStatus{IsVerified: true}, nil
}

func (m *MockMCPServiceImpl) VerifyMCPCapability(ctx context.Context, mcpID uuid.UUID, capability string, resource string, targetService string, metadata map[string]interface{}) (allowed bool, reason string, auditID uuid.UUID, err error) {
	if m.VerifyMCPCapabilityFunc != nil {
		return m.VerifyMCPCapabilityFunc(ctx, mcpID, capability, resource, targetService, metadata)
	}
	return true, "allowed", uuid.New(), nil
}

func (m *MockMCPServiceImpl) GetConnectedAgents(ctx context.Context, mcpServerID uuid.UUID) ([]application.ConnectedAgent, error) {
	if m.GetConnectedAgentsFunc != nil {
		return m.GetConnectedAgentsFunc(ctx, mcpServerID)
	}
	return []application.ConnectedAgent{}, nil
}

func (m *MockMCPServiceImpl) GetConnectedAgentsCount(ctx context.Context, mcpServerID uuid.UUID) (int, error) {
	if m.GetConnectedAgentsCountFunc != nil {
		return m.GetConnectedAgentsCountFunc(ctx, mcpServerID)
	}
	return 0, nil
}

// MockAuditServiceImpl implements AuditServicer interface
type MockAuditServiceImpl struct {
	LogActionFunc        func(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error
	GetAgentActivityFunc func(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error)
	GetAuditLogsFunc     func(ctx context.Context, orgID uuid.UUID, action string, entityType string, entityID *uuid.UUID, userID *uuid.UUID, startDate *time.Time, endDate *time.Time, limit int, offset int) ([]*domain.AuditLog, int, error)
}

func (m *MockAuditServiceImpl) LogAction(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error {
	if m.LogActionFunc != nil {
		return m.LogActionFunc(ctx, orgID, userID, action, resourceType, resourceID, ipAddress, userAgent, metadata)
	}
	return nil
}

func (m *MockAuditServiceImpl) GetAgentActivity(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	if m.GetAgentActivityFunc != nil {
		return m.GetAgentActivityFunc(ctx, agentID, limit, offset)
	}
	return []*domain.AuditLog{}, nil
}

func (m *MockAuditServiceImpl) GetAuditLogs(ctx context.Context, orgID uuid.UUID, action string, entityType string, entityID *uuid.UUID, userID *uuid.UUID, startDate *time.Time, endDate *time.Time, limit int, offset int) ([]*domain.AuditLog, int, error) {
	if m.GetAuditLogsFunc != nil {
		return m.GetAuditLogsFunc(ctx, orgID, action, entityType, entityID, userID, startDate, endDate, limit, offset)
	}
	return []*domain.AuditLog{}, 0, nil
}

// MockCapabilityServiceImpl implements CapabilityServicer interface
type MockCapabilityServiceImpl struct {
	VerifyActionFunc                  func(ctx context.Context, agentID uuid.UUID, requestedCapability string, signature []byte, payload []byte, sourceIP *string, metadata map[string]interface{}) (*application.VerificationResult, error)
	GrantCapabilityFunc               func(ctx context.Context, agentID uuid.UUID, capabilityType string, scope map[string]interface{}, grantedBy *uuid.UUID) (*domain.AgentCapability, error)
	RevokeCapabilityFunc              func(ctx context.Context, capabilityID uuid.UUID, revokedBy *uuid.UUID) error
	GetAgentCapabilitiesFunc          func(ctx context.Context, agentID uuid.UUID, activeOnly bool) ([]*domain.AgentCapability, error)
	ListCapabilitiesFunc              func(ctx context.Context, orgID uuid.UUID) ([]application.CapabilityDefinition, error)
	ListCapabilitiesWithMetadataFunc  func(ctx context.Context, orgID uuid.UUID) (*application.ListCapabilitiesResponse, error)
	ValidateAndRegisterCapabilityFunc func(ctx context.Context, capability string, orgID uuid.UUID) error
	GetViolationsByAgentFunc          func(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error)
	GetViolationsByOrganizationFunc   func(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error)
	GetRecentViolationsFunc           func(ctx context.Context, orgID uuid.UUID, minutes int) ([]*domain.CapabilityViolation, error)
}

func (m *MockCapabilityServiceImpl) VerifyAction(ctx context.Context, agentID uuid.UUID, requestedCapability string, signature []byte, payload []byte, sourceIP *string, metadata map[string]interface{}) (*application.VerificationResult, error) {
	if m.VerifyActionFunc != nil {
		return m.VerifyActionFunc(ctx, agentID, requestedCapability, signature, payload, sourceIP, metadata)
	}
	return &application.VerificationResult{IsValid: true, IsAuthorized: true, InScope: true}, nil
}

func (m *MockCapabilityServiceImpl) GrantCapability(ctx context.Context, agentID uuid.UUID, capabilityType string, scope map[string]interface{}, grantedBy *uuid.UUID) (*domain.AgentCapability, error) {
	if m.GrantCapabilityFunc != nil {
		return m.GrantCapabilityFunc(ctx, agentID, capabilityType, scope, grantedBy)
	}
	return &domain.AgentCapability{ID: uuid.New(), AgentID: agentID, CapabilityType: capabilityType}, nil
}

func (m *MockCapabilityServiceImpl) RevokeCapability(ctx context.Context, capabilityID uuid.UUID, revokedBy *uuid.UUID) error {
	if m.RevokeCapabilityFunc != nil {
		return m.RevokeCapabilityFunc(ctx, capabilityID, revokedBy)
	}
	return nil
}

func (m *MockCapabilityServiceImpl) GetAgentCapabilities(ctx context.Context, agentID uuid.UUID, activeOnly bool) ([]*domain.AgentCapability, error) {
	if m.GetAgentCapabilitiesFunc != nil {
		return m.GetAgentCapabilitiesFunc(ctx, agentID, activeOnly)
	}
	return []*domain.AgentCapability{}, nil
}

func (m *MockCapabilityServiceImpl) ListCapabilities(ctx context.Context, orgID uuid.UUID) ([]application.CapabilityDefinition, error) {
	if m.ListCapabilitiesFunc != nil {
		return m.ListCapabilitiesFunc(ctx, orgID)
	}
	return []application.CapabilityDefinition{}, nil
}

func (m *MockCapabilityServiceImpl) ListCapabilitiesWithMetadata(ctx context.Context, orgID uuid.UUID) (*application.ListCapabilitiesResponse, error) {
	if m.ListCapabilitiesWithMetadataFunc != nil {
		return m.ListCapabilitiesWithMetadataFunc(ctx, orgID)
	}
	return &application.ListCapabilitiesResponse{Capabilities: []application.CapabilityDefinition{}}, nil
}

func (m *MockCapabilityServiceImpl) ValidateAndRegisterCapability(ctx context.Context, capability string, orgID uuid.UUID) error {
	if m.ValidateAndRegisterCapabilityFunc != nil {
		return m.ValidateAndRegisterCapabilityFunc(ctx, capability, orgID)
	}
	return nil
}

func (m *MockCapabilityServiceImpl) GetViolationsByAgent(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error) {
	if m.GetViolationsByAgentFunc != nil {
		return m.GetViolationsByAgentFunc(ctx, agentID, limit, offset)
	}
	return []*domain.CapabilityViolation{}, 0, nil
}

func (m *MockCapabilityServiceImpl) GetViolationsByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error) {
	if m.GetViolationsByOrganizationFunc != nil {
		return m.GetViolationsByOrganizationFunc(ctx, orgID, limit, offset)
	}
	return []*domain.CapabilityViolation{}, 0, nil
}

func (m *MockCapabilityServiceImpl) GetRecentViolations(ctx context.Context, orgID uuid.UUID, minutes int) ([]*domain.CapabilityViolation, error) {
	if m.GetRecentViolationsFunc != nil {
		return m.GetRecentViolationsFunc(ctx, orgID, minutes)
	}
	return []*domain.CapabilityViolation{}, nil
}

// MockTagServiceImpl implements TagServicer interface
type MockTagServiceImpl struct {
	GetAgentTagsFunc     func(ctx context.Context, agentID uuid.UUID) ([]*domain.Tag, error)
	GetMCPServerTagsFunc func(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.Tag, error)
}

func (m *MockTagServiceImpl) GetAgentTags(ctx context.Context, agentID uuid.UUID) ([]*domain.Tag, error) {
	if m.GetAgentTagsFunc != nil {
		return m.GetAgentTagsFunc(ctx, agentID)
	}
	return []*domain.Tag{}, nil
}

func (m *MockTagServiceImpl) GetMCPServerTags(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.Tag, error) {
	if m.GetMCPServerTagsFunc != nil {
		return m.GetMCPServerTagsFunc(ctx, mcpServerID)
	}
	return []*domain.Tag{}, nil
}

// MockAPIKeyServiceImpl implements APIKeyServicer interface
type MockAPIKeyServiceImpl struct {
	GenerateAPIKeyFunc func(ctx context.Context, agentID uuid.UUID, orgID uuid.UUID, userID uuid.UUID, name string, expirationDays int) (fullKey string, record *domain.APIKey, err error)
}

func (m *MockAPIKeyServiceImpl) GenerateAPIKey(ctx context.Context, agentID uuid.UUID, orgID uuid.UUID, userID uuid.UUID, name string, expirationDays int) (fullKey string, record *domain.APIKey, err error) {
	if m.GenerateAPIKeyFunc != nil {
		return m.GenerateAPIKeyFunc(ctx, agentID, orgID, userID, name, expirationDays)
	}
	return "mock-api-key-12345", &domain.APIKey{ID: uuid.New(), Name: name, AgentID: agentID}, nil
}

// MockAlertServiceImpl implements AlertServicer interface
type MockAlertServiceImpl struct {
	CreateAlertFunc      func(ctx context.Context, alert *domain.Alert) error
	GetAlertsByAgentFunc func(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.Alert, error)
}

func (m *MockAlertServiceImpl) CreateAlert(ctx context.Context, alert *domain.Alert) error {
	if m.CreateAlertFunc != nil {
		return m.CreateAlertFunc(ctx, alert)
	}
	return nil
}

func (m *MockAlertServiceImpl) GetAlertsByAgent(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.Alert, error) {
	if m.GetAlertsByAgentFunc != nil {
		return m.GetAlertsByAgentFunc(ctx, agentID, limit, offset)
	}
	return []*domain.Alert{}, nil
}

// MockVerificationEventServiceImpl implements VerificationEventServicer interface
type MockVerificationEventServiceImpl struct {
	LogVerificationEventFunc func(ctx context.Context, orgID uuid.UUID, agentID uuid.UUID, protocol domain.VerificationProtocol, verificationType domain.VerificationType, status domain.VerificationEventStatus, durationMs int, initiatorType domain.InitiatorType, initiatorID *uuid.UUID, metadata map[string]interface{}) (*domain.VerificationEvent, error)
}

func (m *MockVerificationEventServiceImpl) LogVerificationEvent(ctx context.Context, orgID uuid.UUID, agentID uuid.UUID, protocol domain.VerificationProtocol, verificationType domain.VerificationType, status domain.VerificationEventStatus, durationMs int, initiatorType domain.InitiatorType, initiatorID *uuid.UUID, metadata map[string]interface{}) (*domain.VerificationEvent, error) {
	if m.LogVerificationEventFunc != nil {
		return m.LogVerificationEventFunc(ctx, orgID, agentID, protocol, verificationType, status, durationMs, initiatorType, initiatorID, metadata)
	}
	return &domain.VerificationEvent{ID: uuid.New()}, nil
}

// MockMCPAttestationServiceImpl implements MCPAttestationServicer interface
type MockMCPAttestationServiceImpl struct {
	GetMCPServersForAgentFunc func(ctx context.Context, agentID uuid.UUID) ([]*domain.MCPServer, error)
}

func (m *MockMCPAttestationServiceImpl) GetMCPServersForAgent(ctx context.Context, agentID uuid.UUID) ([]*domain.MCPServer, error) {
	if m.GetMCPServersForAgentFunc != nil {
		return m.GetMCPServersForAgentFunc(ctx, agentID)
	}
	return []*domain.MCPServer{}, nil
}

// MockOrganizationRepository implements domain.OrganizationRepository interface
type MockOrganizationRepository struct {
	GetByIDFunc     func(id uuid.UUID) (*domain.Organization, error)
	GetByDomainFunc func(dom string) (*domain.Organization, error)
	CreateFunc      func(org *domain.Organization) error
	UpdateFunc      func(org *domain.Organization) error
	DeleteFunc      func(id uuid.UUID) error
}

func (m *MockOrganizationRepository) GetByID(id uuid.UUID) (*domain.Organization, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(id)
	}
	return &domain.Organization{ID: id, Name: "Test Org", EnforcementMode: domain.EnforcementModeMonitoring}, nil
}

func (m *MockOrganizationRepository) GetByDomain(dom string) (*domain.Organization, error) {
	if m.GetByDomainFunc != nil {
		return m.GetByDomainFunc(dom)
	}
	return nil, nil
}

func (m *MockOrganizationRepository) Create(org *domain.Organization) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(org)
	}
	return nil
}

func (m *MockOrganizationRepository) Update(org *domain.Organization) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(org)
	}
	return nil
}

func (m *MockOrganizationRepository) Delete(id uuid.UUID) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(id)
	}
	return nil
}
