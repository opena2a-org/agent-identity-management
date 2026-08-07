package handlers

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// ===========================
// Mock Implementations
// ===========================
// These mocks implement the interfaces defined in interfaces.go

// MockAgentServiceImpl implements AgentServicer interface
type MockAgentServiceImpl struct {
	CreateAgentFunc                func(ctx context.Context, req *application.CreateAgentRequest, orgID, userID uuid.UUID, sdkTokenID *uuid.UUID, apiKeyID *uuid.UUID, userEmail string) (*domain.Agent, error)
	GetAgentFunc                   func(ctx context.Context, id uuid.UUID) (*domain.Agent, error)
	GetAgentsByIDsFunc             func(ctx context.Context, ids []uuid.UUID) ([]*domain.Agent, error)
	GetAgentByNameFunc             func(ctx context.Context, orgID uuid.UUID, name string) (*domain.Agent, error)
	ListAgentsFunc                 func(ctx context.Context, orgID uuid.UUID) ([]*domain.Agent, error)
	ListAgentsPagedFunc            func(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Agent, int, error)
	UpdateAgentFunc                func(ctx context.Context, id uuid.UUID, req *application.CreateAgentRequest, requestedBy uuid.UUID) (*domain.Agent, error)
	DeleteAgentFunc                func(ctx context.Context, id uuid.UUID) error
	VerifyAgentFunc                func(ctx context.Context, id uuid.UUID) error
	SuspendAgentFunc               func(ctx context.Context, id uuid.UUID) error
	RevokeAgentFunc                func(ctx context.Context, id uuid.UUID) error
	ReactivateAgentFunc            func(ctx context.Context, id uuid.UUID) error
	RecalculateTrustScoreFunc      func(ctx context.Context, id uuid.UUID) (*domain.TrustScore, error)
	UpdateTrustScoreFunc           func(ctx context.Context, agentID uuid.UUID, newScore float64) error
	RotateCredentialsFunc          func(ctx context.Context, id uuid.UUID) (publicKey, privateKey string, err error)
	GetAgentCredentialsFunc        func(ctx context.Context, agentID uuid.UUID) (publicKey, privateKey string, err error)
	UpdateAgentPublicKeyFunc       func(ctx context.Context, agentID uuid.UUID, publicKey string) error
	AddMCPServersFunc              func(ctx context.Context, agentID uuid.UUID, mcpServerIdentifiers []string) (*domain.Agent, []string, error)
	RemoveMCPServerFunc            func(ctx context.Context, agentID uuid.UUID, mcpServerIdentifier string) (*domain.Agent, error)
	VerifyCapabilityFunc           func(ctx context.Context, agentID uuid.UUID, capability string, resource string, metadata map[string]interface{}, sourceIP string) (allowed bool, reason string, auditID uuid.UUID, err error)
	LogCapabilityResultFunc        func(ctx context.Context, agentID uuid.UUID, auditID uuid.UUID, success bool, errorMsg string, result map[string]interface{}) error
	HasCapabilityFunc              func(ctx context.Context, agentID uuid.UUID, capabilityToCheck string, resource string) (bool, error)
	HasCapabilityNoAlertFunc       func(ctx context.Context, agentID uuid.UUID, capabilityToCheck string, resource string) (bool, error)
	UpdateLastActiveFunc           func(ctx context.Context, agentID uuid.UUID) error
	DetectMCPServersFromConfigFunc func(ctx context.Context, agentID uuid.UUID, req *application.DetectMCPServersRequest, mcpService *application.MCPService, orgID, userID uuid.UUID) (*application.DetectMCPServersResult, error)
	// PQC methods
	UpdateAgentPQCKeyFunc  func(ctx context.Context, agentID uuid.UUID, pqcPublicKey string, algorithm string, hybridMode bool) error
	RotateAgentPQCKeyFunc  func(ctx context.Context, agentID uuid.UUID, newPQCPublicKey string, algorithm string) error
	SetAgentHybridModeFunc func(ctx context.Context, agentID uuid.UUID, enabled bool) error
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

func (m *MockAgentServiceImpl) GetAgentsByIDs(ctx context.Context, callerOrgID uuid.UUID, ids []uuid.UUID) ([]*domain.Agent, error) {
	if m.GetAgentsByIDsFunc != nil {
		return m.GetAgentsByIDsFunc(ctx, ids)
	}
	agents := make([]*domain.Agent, 0, len(ids))
	for _, id := range ids {
		agent, err := m.GetAgent(ctx, id)
		if err != nil {
			return nil, err
		}
		if agent != nil {
			agents = append(agents, agent)
		}
	}
	return agents, nil
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

func (m *MockAgentServiceImpl) UpdateAgent(ctx context.Context, id uuid.UUID, req *application.CreateAgentRequest, requestedBy uuid.UUID) (*domain.Agent, error) {
	if m.UpdateAgentFunc != nil {
		return m.UpdateAgentFunc(ctx, id, req, requestedBy)
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

func (m *MockAgentServiceImpl) RevokeAgent(ctx context.Context, id uuid.UUID) error {
	if m.RevokeAgentFunc != nil {
		return m.RevokeAgentFunc(ctx, id)
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

func (m *MockAgentServiceImpl) UpdateAgentPublicKey(ctx context.Context, agentID uuid.UUID, publicKey string, authMethod string) error {
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

func (m *MockAgentServiceImpl) HasCapabilityNoAlert(ctx context.Context, agentID uuid.UUID, capabilityToCheck string, resource string) (bool, error) {
	if m.HasCapabilityNoAlertFunc != nil {
		return m.HasCapabilityNoAlertFunc(ctx, agentID, capabilityToCheck, resource)
	}
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

func (m *MockAgentServiceImpl) UpdateAgentPQCKey(ctx context.Context, agentID uuid.UUID, pqcPublicKey string, algorithm string, hybridMode bool) error {
	if m.UpdateAgentPQCKeyFunc != nil {
		return m.UpdateAgentPQCKeyFunc(ctx, agentID, pqcPublicKey, algorithm, hybridMode)
	}
	return nil
}

func (m *MockAgentServiceImpl) RotateAgentPQCKey(ctx context.Context, agentID uuid.UUID, newPQCPublicKey string, algorithm string) error {
	if m.RotateAgentPQCKeyFunc != nil {
		return m.RotateAgentPQCKeyFunc(ctx, agentID, newPQCPublicKey, algorithm)
	}
	return nil
}

func (m *MockAgentServiceImpl) SetAgentHybridMode(ctx context.Context, agentID uuid.UUID, enabled bool) error {
	if m.SetAgentHybridModeFunc != nil {
		return m.SetAgentHybridModeFunc(ctx, agentID, enabled)
	}
	return nil
}

// MockMCPServiceImpl implements MCPServicer interface
type MockMCPServiceImpl struct {
	CreateMCPServerFunc               func(ctx context.Context, req *application.CreateMCPServerRequest, orgID, userID uuid.UUID, agentID *uuid.UUID, sdkTokenID *uuid.UUID, apiKeyID *uuid.UUID) (*domain.MCPServer, error)
	GetMCPServerFunc                  func(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error)
	GetMCPServerByNameFunc            func(ctx context.Context, orgID uuid.UUID, name string) (*domain.MCPServer, error)
	ListMCPServersFunc                func(ctx context.Context, orgID uuid.UUID) ([]*domain.MCPServer, error)
	UpdateMCPServerFunc               func(ctx context.Context, id uuid.UUID, req *application.UpdateMCPServerRequest) (*domain.MCPServer, error)
	DeleteMCPServerFunc               func(ctx context.Context, id uuid.UUID) error
	VerifyMCPServerFunc               func(ctx context.Context, id uuid.UUID, userID uuid.UUID, userIP string) error
	AddPublicKeyFunc                  func(ctx context.Context, serverID uuid.UUID, req *application.AddPublicKeyRequest) error
	GetVerificationStatusFunc         func(ctx context.Context, id uuid.UUID) (*domain.MCPServerVerificationStatus, error)
	VerifyMCPCapabilityFunc           func(ctx context.Context, mcpID uuid.UUID, capability string, resource string, targetService string, metadata map[string]interface{}) (allowed bool, reason string, auditID uuid.UUID, err error)
	GetConnectedAgentsFunc            func(ctx context.Context, mcpServerID uuid.UUID) ([]application.ConnectedAgent, error)
	GetConnectedAgentsCountFunc       func(ctx context.Context, mcpServerID uuid.UUID) (int, error)
	GenerateVerificationChallengeFunc func(ctx context.Context, serverID uuid.UUID) (string, error)
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

func (m *MockMCPServiceImpl) GenerateVerificationChallenge(ctx context.Context, serverID uuid.UUID) (string, error) {
	if m.GenerateVerificationChallengeFunc != nil {
		return m.GenerateVerificationChallengeFunc(ctx, serverID)
	}
	return "test-challenge", nil
}

// MockAuditServiceImpl implements AuditServicer interface
type MockAuditServiceImpl struct {
	LogActionFunc        func(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error
	LogAgentActionFunc   func(ctx context.Context, orgID uuid.UUID, agentID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error
	GetAgentActivityFunc func(ctx context.Context, orgID, agentID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error)
	GetAuditLogsFunc     func(ctx context.Context, orgID uuid.UUID, action string, entityType string, entityID *uuid.UUID, userID *uuid.UUID, startDate *time.Time, endDate *time.Time, limit int, offset int) ([]*domain.AuditLog, int, error)
	GetByIDFunc          func(ctx context.Context, id uuid.UUID) (*domain.AuditLog, error)
}

func (m *MockAuditServiceImpl) LogAction(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error {
	if m.LogActionFunc != nil {
		return m.LogActionFunc(ctx, orgID, userID, action, resourceType, resourceID, ipAddress, userAgent, metadata)
	}
	return nil
}

func (m *MockAuditServiceImpl) LogAgentAction(ctx context.Context, orgID uuid.UUID, agentID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error {
	if m.LogAgentActionFunc != nil {
		return m.LogAgentActionFunc(ctx, orgID, agentID, action, resourceType, resourceID, ipAddress, userAgent, metadata)
	}
	return nil
}

func (m *MockAuditServiceImpl) GetAgentActivity(ctx context.Context, orgID, agentID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	if m.GetAgentActivityFunc != nil {
		return m.GetAgentActivityFunc(ctx, orgID, agentID, limit, offset)
	}
	return []*domain.AuditLog{}, nil
}

func (m *MockAuditServiceImpl) GetAuditLogs(ctx context.Context, orgID uuid.UUID, action string, entityType string, entityID *uuid.UUID, userID *uuid.UUID, startDate *time.Time, endDate *time.Time, limit int, offset int) ([]*domain.AuditLog, int, error) {
	if m.GetAuditLogsFunc != nil {
		return m.GetAuditLogsFunc(ctx, orgID, action, entityType, entityID, userID, startDate, endDate, limit, offset)
	}
	return []*domain.AuditLog{}, 0, nil
}

func (m *MockAuditServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*domain.AuditLog, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return &domain.AuditLog{ID: id}, nil
}

// MockCapabilityServiceImpl implements CapabilityServicer interface
type MockCapabilityServiceImpl struct {
	VerifyActionFunc                  func(ctx context.Context, agentID uuid.UUID, requestedCapability string, signature []byte, payload []byte, sourceIP *string, metadata map[string]interface{}) (*application.VerificationResult, error)
	GrantCapabilityFunc               func(ctx context.Context, agentID uuid.UUID, capabilityType string, scope map[string]interface{}, grantedBy *uuid.UUID, executionMode string) (*domain.AgentCapability, error)
	RevokeCapabilityFunc              func(ctx context.Context, capabilityID uuid.UUID, revokedBy *uuid.UUID) error
	MarkHoneytokenFunc                func(ctx context.Context, capabilityID uuid.UUID, honeytoken bool, markedBy *uuid.UUID) (*domain.AgentCapability, error)
	GetCapabilityByIDFunc             func(ctx context.Context, capabilityID uuid.UUID) (*domain.AgentCapability, error)
	GetAgentCapabilitiesFunc          func(ctx context.Context, agentID uuid.UUID, activeOnly bool) ([]*domain.AgentCapability, error)
	GetCapabilitiesByAgentIDsFunc     func(ctx context.Context, agentIDs []uuid.UUID, activeOnly bool) (map[uuid.UUID][]*domain.AgentCapability, error)
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

func (m *MockCapabilityServiceImpl) GrantCapability(ctx context.Context, agentID uuid.UUID, capabilityType string, scope map[string]interface{}, grantedBy *uuid.UUID, executionMode string) (*domain.AgentCapability, error) {
	if m.GrantCapabilityFunc != nil {
		return m.GrantCapabilityFunc(ctx, agentID, capabilityType, scope, grantedBy, executionMode)
	}
	return &domain.AgentCapability{ID: uuid.New(), AgentID: agentID, CapabilityType: capabilityType, ExecutionMode: executionMode}, nil
}

func (m *MockCapabilityServiceImpl) RevokeCapability(ctx context.Context, capabilityID uuid.UUID, revokedBy *uuid.UUID) error {
	if m.RevokeCapabilityFunc != nil {
		return m.RevokeCapabilityFunc(ctx, capabilityID, revokedBy)
	}
	return nil
}

func (m *MockCapabilityServiceImpl) MarkHoneytoken(ctx context.Context, capabilityID uuid.UUID, honeytoken bool, markedBy *uuid.UUID) (*domain.AgentCapability, error) {
	if m.MarkHoneytokenFunc != nil {
		return m.MarkHoneytokenFunc(ctx, capabilityID, honeytoken, markedBy)
	}
	return nil, nil
}

func (m *MockCapabilityServiceImpl) GetCapabilityByID(ctx context.Context, capabilityID uuid.UUID) (*domain.AgentCapability, error) {
	if m.GetCapabilityByIDFunc != nil {
		return m.GetCapabilityByIDFunc(ctx, capabilityID)
	}
	return nil, nil
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
	GetAgentTagsFunc                func(ctx context.Context, agentID uuid.UUID) ([]*domain.Tag, error)
	GetAgentTagsByAgentIDsFunc      func(ctx context.Context, agentIDs []uuid.UUID) (map[uuid.UUID][]*domain.Tag, error)
	GetMCPServerTagsFunc            func(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.Tag, error)
	GetMCPServerTagsByServerIDsFunc func(ctx context.Context, mcpServerIDs []uuid.UUID) (map[uuid.UUID][]*domain.Tag, error)
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

func (m *MockTagServiceImpl) GetMCPServerTagsByServerIDs(ctx context.Context, mcpServerIDs []uuid.UUID) (map[uuid.UUID][]*domain.Tag, error) {
	if m.GetMCPServerTagsByServerIDsFunc != nil {
		return m.GetMCPServerTagsByServerIDsFunc(ctx, mcpServerIDs)
	}
	result := make(map[uuid.UUID][]*domain.Tag, len(mcpServerIDs))
	for _, serverID := range mcpServerIDs {
		tags, err := m.GetMCPServerTags(ctx, serverID)
		if err != nil {
			return nil, err
		}
		if len(tags) > 0 {
			result[serverID] = tags
		}
	}
	return result, nil
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
	CreateAlertFunc         func(ctx context.Context, alert *domain.Alert) error
	GetAlertsByAgentFunc    func(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.Alert, error)
	GetAlertsFunc           func(ctx context.Context, orgID uuid.UUID, severity, status string, limit, offset int) ([]*domain.Alert, int, error)
	CountUnacknowledgedFunc func(ctx context.Context, orgID uuid.UUID) (allCount, acknowledgedCount, unacknowledgedCount int, err error)
	CountBySeverityFunc     func(ctx context.Context, orgID uuid.UUID, status string) (critical, high, warning, info int, err error)
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

func (m *MockAlertServiceImpl) GetAlerts(ctx context.Context, orgID uuid.UUID, severity, status string, limit, offset int) ([]*domain.Alert, int, error) {
	if m.GetAlertsFunc != nil {
		return m.GetAlertsFunc(ctx, orgID, severity, status, limit, offset)
	}
	return []*domain.Alert{}, 0, nil
}

func (m *MockAlertServiceImpl) CountUnacknowledged(ctx context.Context, orgID uuid.UUID) (allCount, acknowledgedCount, unacknowledgedCount int, err error) {
	if m.CountUnacknowledgedFunc != nil {
		return m.CountUnacknowledgedFunc(ctx, orgID)
	}
	return 0, 0, 0, nil
}

func (m *MockAlertServiceImpl) CountBySeverity(ctx context.Context, orgID uuid.UUID, status string) (critical, high, warning, info int, err error) {
	if m.CountBySeverityFunc != nil {
		return m.CountBySeverityFunc(ctx, orgID, status)
	}
	return 0, 0, 0, 0, nil
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

func (m *MockOrganizationRepository) ListAll() ([]*domain.Organization, error) {
	return nil, nil
}

// ===========================
// Additional Mock Implementations for AdminHandler
// ===========================

// MockAuthServiceImpl implements AuthServicer interface
type MockAuthServiceImpl struct {
	GetUsersByOrganizationFunc func(ctx context.Context, orgID uuid.UUID) ([]*domain.User, error)
	GetUserByIDFunc            func(ctx context.Context, id uuid.UUID) (*domain.User, error)
	CountActiveUsersFunc       func(ctx context.Context, orgID uuid.UUID, withinMinutes int) (int, error)
	UpdateUserRoleFunc         func(ctx context.Context, userID, orgID uuid.UUID, role domain.UserRole, adminID uuid.UUID) (*domain.User, error)
	DeactivateUserFunc         func(ctx context.Context, userID, orgID, adminID uuid.UUID) error
}

func (m *MockAuthServiceImpl) GetUsersByOrganization(ctx context.Context, orgID uuid.UUID) ([]*domain.User, error) {
	if m.GetUsersByOrganizationFunc != nil {
		return m.GetUsersByOrganizationFunc(ctx, orgID)
	}
	return []*domain.User{}, nil
}

func (m *MockAuthServiceImpl) GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if m.GetUserByIDFunc != nil {
		return m.GetUserByIDFunc(ctx, id)
	}
	return &domain.User{ID: id}, nil
}

func (m *MockAuthServiceImpl) CountActiveUsers(ctx context.Context, orgID uuid.UUID, withinMinutes int) (int, error) {
	if m.CountActiveUsersFunc != nil {
		return m.CountActiveUsersFunc(ctx, orgID, withinMinutes)
	}
	return 0, nil
}

func (m *MockAuthServiceImpl) UpdateUserRole(ctx context.Context, userID, orgID uuid.UUID, role domain.UserRole, adminID uuid.UUID) (*domain.User, error) {
	if m.UpdateUserRoleFunc != nil {
		return m.UpdateUserRoleFunc(ctx, userID, orgID, role, adminID)
	}
	return &domain.User{ID: userID, Role: role}, nil
}

func (m *MockAuthServiceImpl) DeactivateUser(ctx context.Context, userID, orgID, adminID uuid.UUID) error {
	if m.DeactivateUserFunc != nil {
		return m.DeactivateUserFunc(ctx, userID, orgID, adminID)
	}
	return nil
}

// MockAdminServiceImpl implements AdminServicer interface
type MockAdminServiceImpl struct {
	GetPendingUsersFunc         func(ctx context.Context, adminOrgID uuid.UUID) ([]*domain.User, error)
	ApproveUserFunc             func(ctx context.Context, userID, adminID uuid.UUID) error
	RejectUserFunc              func(ctx context.Context, userID, adminID uuid.UUID, reason string) error
	ActivateUserFunc            func(ctx context.Context, userID, adminID uuid.UUID) error
	PermanentlyDeleteUserFunc   func(ctx context.Context, userID, adminID uuid.UUID) error
	GetOrganizationSettingsFunc func(ctx context.Context, orgID uuid.UUID) (*domain.Organization, error)
	GetEnforcementSettingsFunc  func(ctx context.Context, orgID uuid.UUID) (*application.EnforcementSettings, error)
	UpdateEnforcementModeFunc   func(ctx context.Context, orgID uuid.UUID, mode domain.EnforcementMode) error
}

// NOTE: Mock methods accept adminOrgID but delegate to original funcs that don't need it

func (m *MockAdminServiceImpl) GetPendingUsers(ctx context.Context, adminOrgID uuid.UUID) ([]*domain.User, error) {
	if m.GetPendingUsersFunc != nil {
		return m.GetPendingUsersFunc(ctx, adminOrgID)
	}
	return []*domain.User{}, nil
}

func (m *MockAdminServiceImpl) ApproveUser(ctx context.Context, userID, adminID, adminOrgID uuid.UUID) error {
	if m.ApproveUserFunc != nil {
		return m.ApproveUserFunc(ctx, userID, adminID)
	}
	return nil
}

func (m *MockAdminServiceImpl) RejectUser(ctx context.Context, userID, adminID, adminOrgID uuid.UUID, reason string) error {
	if m.RejectUserFunc != nil {
		return m.RejectUserFunc(ctx, userID, adminID, reason)
	}
	return nil
}

func (m *MockAdminServiceImpl) ActivateUser(ctx context.Context, userID, adminID, adminOrgID uuid.UUID) error {
	if m.ActivateUserFunc != nil {
		return m.ActivateUserFunc(ctx, userID, adminID)
	}
	return nil
}

func (m *MockAdminServiceImpl) PermanentlyDeleteUser(ctx context.Context, userID, adminID, adminOrgID uuid.UUID) error {
	if m.PermanentlyDeleteUserFunc != nil {
		return m.PermanentlyDeleteUserFunc(ctx, userID, adminID)
	}
	return nil
}

func (m *MockAdminServiceImpl) GetOrganizationSettings(ctx context.Context, orgID uuid.UUID) (*domain.Organization, error) {
	if m.GetOrganizationSettingsFunc != nil {
		return m.GetOrganizationSettingsFunc(ctx, orgID)
	}
	return &domain.Organization{ID: orgID, Name: "Test Org", EnforcementMode: domain.EnforcementModeMonitoring}, nil
}

func (m *MockAdminServiceImpl) GetEnforcementSettings(ctx context.Context, orgID uuid.UUID) (*application.EnforcementSettings, error) {
	if m.GetEnforcementSettingsFunc != nil {
		return m.GetEnforcementSettingsFunc(ctx, orgID)
	}
	return &application.EnforcementSettings{EnforcementMode: "monitoring"}, nil
}

func (m *MockAdminServiceImpl) UpdateEnforcementMode(ctx context.Context, orgID uuid.UUID, mode domain.EnforcementMode) error {
	if m.UpdateEnforcementModeFunc != nil {
		return m.UpdateEnforcementModeFunc(ctx, orgID, mode)
	}
	return nil
}

// MockAlertServiceExtendedImpl implements AlertServicerExtended interface
type MockAlertServiceExtendedImpl struct {
	GetAlertsFunc             func(ctx context.Context, orgID uuid.UUID, severity, status string, limit, offset int) ([]*domain.Alert, int, error)
	GetAlertsByAgentFunc      func(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.Alert, error)
	CreateAlertFunc           func(ctx context.Context, alert *domain.Alert) error
	AcknowledgeAlertFunc      func(ctx context.Context, alertID, orgID, userID uuid.UUID) error
	ResolveAlertFunc          func(ctx context.Context, alertID, orgID, userID uuid.UUID, resolution string) error
	BulkAcknowledgeAlertsFunc func(ctx context.Context, orgID, userID uuid.UUID) (int, error)
	CountUnacknowledgedFunc   func(ctx context.Context, orgID uuid.UUID) (allCount, acknowledgedCount, unacknowledgedCount int, err error)
	CountBySeverityFunc       func(ctx context.Context, orgID uuid.UUID, status string) (critical, high, warning, info int, err error)
}

func (m *MockAlertServiceExtendedImpl) GetAlerts(ctx context.Context, orgID uuid.UUID, severity, status string, limit, offset int) ([]*domain.Alert, int, error) {
	if m.GetAlertsFunc != nil {
		return m.GetAlertsFunc(ctx, orgID, severity, status, limit, offset)
	}
	return []*domain.Alert{}, 0, nil
}

func (m *MockAlertServiceExtendedImpl) GetAlertsByAgent(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.Alert, error) {
	if m.GetAlertsByAgentFunc != nil {
		return m.GetAlertsByAgentFunc(ctx, agentID, limit, offset)
	}
	return []*domain.Alert{}, nil
}

func (m *MockAlertServiceExtendedImpl) CreateAlert(ctx context.Context, alert *domain.Alert) error {
	if m.CreateAlertFunc != nil {
		return m.CreateAlertFunc(ctx, alert)
	}
	return nil
}

func (m *MockAlertServiceExtendedImpl) AcknowledgeAlert(ctx context.Context, alertID, orgID, userID uuid.UUID) error {
	if m.AcknowledgeAlertFunc != nil {
		return m.AcknowledgeAlertFunc(ctx, alertID, orgID, userID)
	}
	return nil
}

func (m *MockAlertServiceExtendedImpl) ResolveAlert(ctx context.Context, alertID, orgID, userID uuid.UUID, resolution string) error {
	if m.ResolveAlertFunc != nil {
		return m.ResolveAlertFunc(ctx, alertID, orgID, userID, resolution)
	}
	return nil
}

func (m *MockAlertServiceExtendedImpl) BulkAcknowledgeAlerts(ctx context.Context, orgID, userID uuid.UUID) (int, error) {
	if m.BulkAcknowledgeAlertsFunc != nil {
		return m.BulkAcknowledgeAlertsFunc(ctx, orgID, userID)
	}
	return 0, nil
}

func (m *MockAlertServiceExtendedImpl) CountUnacknowledged(ctx context.Context, orgID uuid.UUID) (allCount, acknowledgedCount, unacknowledgedCount int, err error) {
	if m.CountUnacknowledgedFunc != nil {
		return m.CountUnacknowledgedFunc(ctx, orgID)
	}
	return 0, 0, 0, nil
}

func (m *MockAlertServiceExtendedImpl) CountBySeverity(ctx context.Context, orgID uuid.UUID, status string) (critical, high, warning, info int, err error) {
	if m.CountBySeverityFunc != nil {
		return m.CountBySeverityFunc(ctx, orgID, status)
	}
	return 0, 0, 0, 0, nil
}

// MockSecurityServiceImpl implements SecurityServicer interface
type MockSecurityServiceImpl struct {
	CountOpenIncidentsFunc func(ctx context.Context, orgID uuid.UUID) (int, error)
}

func (m *MockSecurityServiceImpl) CountOpenIncidents(ctx context.Context, orgID uuid.UUID) (int, error) {
	if m.CountOpenIncidentsFunc != nil {
		return m.CountOpenIncidentsFunc(ctx, orgID)
	}
	return 0, nil
}

// MockSecurityServiceExtendedImpl implements SecurityServicerExtended interface
type MockSecurityServiceExtendedImpl struct {
	CountOpenIncidentsFunc func(ctx context.Context, orgID uuid.UUID) (int, error)
	GetThreatsFunc         func(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Threat, error)
	GetAnomaliesFunc       func(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Anomaly, error)
	GetSecurityMetricsFunc func(ctx context.Context, orgID uuid.UUID) (*domain.SecurityMetrics, error)
}

func (m *MockSecurityServiceExtendedImpl) CountOpenIncidents(ctx context.Context, orgID uuid.UUID) (int, error) {
	if m.CountOpenIncidentsFunc != nil {
		return m.CountOpenIncidentsFunc(ctx, orgID)
	}
	return 0, nil
}

func (m *MockSecurityServiceExtendedImpl) GetThreats(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Threat, error) {
	if m.GetThreatsFunc != nil {
		return m.GetThreatsFunc(ctx, orgID, limit, offset)
	}
	return []*domain.Threat{}, nil
}

func (m *MockSecurityServiceExtendedImpl) GetAnomalies(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Anomaly, error) {
	if m.GetAnomaliesFunc != nil {
		return m.GetAnomaliesFunc(ctx, orgID, limit, offset)
	}
	return []*domain.Anomaly{}, nil
}

func (m *MockSecurityServiceExtendedImpl) GetSecurityMetrics(ctx context.Context, orgID uuid.UUID) (*domain.SecurityMetrics, error) {
	if m.GetSecurityMetricsFunc != nil {
		return m.GetSecurityMetricsFunc(ctx, orgID)
	}
	return &domain.SecurityMetrics{}, nil
}

// MockComplianceServiceImpl implements ComplianceServicer interface
type MockComplianceServiceImpl struct {
	GetComplianceStatusFunc  func(ctx context.Context, orgID uuid.UUID) (interface{}, error)
	GetComplianceMetricsFunc func(ctx context.Context, orgID uuid.UUID, startDate, endDate time.Time, interval string) (interface{}, error)
	GetAccessReviewFunc      func(ctx context.Context, orgID uuid.UUID) (interface{}, error)
	ListEvidenceFunc         func(ctx context.Context, orgID uuid.UUID, framework string, limit, offset int) ([]*domain.ComplianceEvidence, int, error)
}

func (m *MockComplianceServiceImpl) GetComplianceStatus(ctx context.Context, orgID uuid.UUID) (interface{}, error) {
	if m.GetComplianceStatusFunc != nil {
		return m.GetComplianceStatusFunc(ctx, orgID)
	}
	return map[string]interface{}{
		"totalAgents":       5,
		"verifiedAgents":    3,
		"verificationRate":  60.0,
		"averageTrustScore": 85.0,
	}, nil
}

func (m *MockComplianceServiceImpl) GetComplianceMetrics(ctx context.Context, orgID uuid.UUID, startDate, endDate time.Time, interval string) (interface{}, error) {
	if m.GetComplianceMetricsFunc != nil {
		return m.GetComplianceMetricsFunc(ctx, orgID, startDate, endDate, interval)
	}
	return []map[string]interface{}{}, nil
}

func (m *MockComplianceServiceImpl) GetAccessReview(ctx context.Context, orgID uuid.UUID) (interface{}, error) {
	if m.GetAccessReviewFunc != nil {
		return m.GetAccessReviewFunc(ctx, orgID)
	}
	return map[string]interface{}{}, nil
}

func (m *MockComplianceServiceImpl) ListEvidence(ctx context.Context, orgID uuid.UUID, framework string, limit, offset int) ([]*domain.ComplianceEvidence, int, error) {
	if m.ListEvidenceFunc != nil {
		return m.ListEvidenceFunc(ctx, orgID, framework, limit, offset)
	}
	return []*domain.ComplianceEvidence{}, 0, nil
}

// MockRegistrationServiceImpl implements RegistrationServicer interface
type MockRegistrationServiceImpl struct {
	ListPendingRegistrationRequestsFunc func(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.UserRegistrationRequest, int, error)
	GetRegistrationRequestFunc          func(ctx context.Context, requestID uuid.UUID) (*domain.UserRegistrationRequest, error)
	ApproveRegistrationRequestFunc      func(ctx context.Context, requestID, adminID, orgID uuid.UUID) (*domain.User, error)
	RejectRegistrationRequestFunc       func(ctx context.Context, requestID, adminID uuid.UUID, reason string) error
}

func (m *MockRegistrationServiceImpl) ListPendingRegistrationRequests(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.UserRegistrationRequest, int, error) {
	if m.ListPendingRegistrationRequestsFunc != nil {
		return m.ListPendingRegistrationRequestsFunc(ctx, orgID, limit, offset)
	}
	return []*domain.UserRegistrationRequest{}, 0, nil
}

func (m *MockRegistrationServiceImpl) GetRegistrationRequest(ctx context.Context, requestID uuid.UUID) (*domain.UserRegistrationRequest, error) {
	if m.GetRegistrationRequestFunc != nil {
		return m.GetRegistrationRequestFunc(ctx, requestID)
	}
	return nil, nil
}

func (m *MockRegistrationServiceImpl) ApproveRegistrationRequest(ctx context.Context, requestID, adminID, orgID uuid.UUID) (*domain.User, error) {
	if m.ApproveRegistrationRequestFunc != nil {
		return m.ApproveRegistrationRequestFunc(ctx, requestID, adminID, orgID)
	}
	return &domain.User{ID: uuid.New(), Email: "test@example.com"}, nil
}

func (m *MockRegistrationServiceImpl) RejectRegistrationRequest(ctx context.Context, requestID, adminID uuid.UUID, reason string) error {
	if m.RejectRegistrationRequestFunc != nil {
		return m.RejectRegistrationRequestFunc(ctx, requestID, adminID, reason)
	}
	return nil
}

// MockTrustScoreHandlerImpl provides a mock for TrustScoreHandler
type MockTrustScoreHandlerImpl struct {
	GetTrustScoreFunc        func(c interface{}) error
	GetTrustScoreHistoryFunc func(c interface{}) error
	CalculateTrustScoreFunc  func(c interface{}) error
}

// ===========================
// MCPHandler Mock Implementations
// ===========================

// MockMCPServiceExtendedImpl implements MCPServicerExtended interface
type MockMCPServiceExtendedImpl struct {
	CreateMCPServerFunc               func(ctx context.Context, req *application.CreateMCPServerRequest, orgID, userID uuid.UUID, agentID *uuid.UUID, sdkTokenID *uuid.UUID, apiKeyID *uuid.UUID) (*domain.MCPServer, error)
	GetMCPServerFunc                  func(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error)
	GetMCPServerByNameFunc            func(ctx context.Context, orgID uuid.UUID, name string) (*domain.MCPServer, error)
	ListMCPServersFunc                func(ctx context.Context, orgID uuid.UUID) ([]*domain.MCPServer, error)
	UpdateMCPServerFunc               func(ctx context.Context, id uuid.UUID, req *application.UpdateMCPServerRequest) (*domain.MCPServer, error)
	DeleteMCPServerFunc               func(ctx context.Context, id uuid.UUID) error
	VerifyMCPServerFunc               func(ctx context.Context, id uuid.UUID, userID uuid.UUID, userIP string) error
	AddPublicKeyFunc                  func(ctx context.Context, serverID uuid.UUID, req *application.AddPublicKeyRequest) error
	GetVerificationStatusFunc         func(ctx context.Context, id uuid.UUID) (*domain.MCPServerVerificationStatus, error)
	VerifyMCPCapabilityFunc           func(ctx context.Context, mcpID uuid.UUID, capability string, resource string, targetService string, metadata map[string]interface{}) (allowed bool, reason string, auditID uuid.UUID, err error)
	GetConnectedAgentsFunc            func(ctx context.Context, mcpServerID uuid.UUID) ([]application.ConnectedAgent, error)
	GetConnectedAgentsCountFunc       func(ctx context.Context, mcpServerID uuid.UUID) (int, error)
	GenerateVerificationChallengeFunc func(ctx context.Context, serverID uuid.UUID) (string, error)
}

func (m *MockMCPServiceExtendedImpl) CreateMCPServer(ctx context.Context, req *application.CreateMCPServerRequest, orgID, userID uuid.UUID, agentID *uuid.UUID, sdkTokenID *uuid.UUID, apiKeyID *uuid.UUID) (*domain.MCPServer, error) {
	if m.CreateMCPServerFunc != nil {
		return m.CreateMCPServerFunc(ctx, req, orgID, userID, agentID, sdkTokenID, apiKeyID)
	}
	return &domain.MCPServer{ID: uuid.New(), OrganizationID: orgID, Name: req.Name}, nil
}

func (m *MockMCPServiceExtendedImpl) GetMCPServer(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error) {
	if m.GetMCPServerFunc != nil {
		return m.GetMCPServerFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockMCPServiceExtendedImpl) GetMCPServerByName(ctx context.Context, orgID uuid.UUID, name string) (*domain.MCPServer, error) {
	if m.GetMCPServerByNameFunc != nil {
		return m.GetMCPServerByNameFunc(ctx, orgID, name)
	}
	return nil, nil
}

func (m *MockMCPServiceExtendedImpl) ListMCPServers(ctx context.Context, orgID uuid.UUID) ([]*domain.MCPServer, error) {
	if m.ListMCPServersFunc != nil {
		return m.ListMCPServersFunc(ctx, orgID)
	}
	return []*domain.MCPServer{}, nil
}

func (m *MockMCPServiceExtendedImpl) UpdateMCPServer(ctx context.Context, id uuid.UUID, req *application.UpdateMCPServerRequest) (*domain.MCPServer, error) {
	if m.UpdateMCPServerFunc != nil {
		return m.UpdateMCPServerFunc(ctx, id, req)
	}
	return nil, nil
}

func (m *MockMCPServiceExtendedImpl) DeleteMCPServer(ctx context.Context, id uuid.UUID) error {
	if m.DeleteMCPServerFunc != nil {
		return m.DeleteMCPServerFunc(ctx, id)
	}
	return nil
}

func (m *MockMCPServiceExtendedImpl) VerifyMCPServer(ctx context.Context, id uuid.UUID, userID uuid.UUID, userIP string) error {
	if m.VerifyMCPServerFunc != nil {
		return m.VerifyMCPServerFunc(ctx, id, userID, userIP)
	}
	return nil
}

func (m *MockMCPServiceExtendedImpl) AddPublicKey(ctx context.Context, serverID uuid.UUID, req *application.AddPublicKeyRequest) error {
	if m.AddPublicKeyFunc != nil {
		return m.AddPublicKeyFunc(ctx, serverID, req)
	}
	return nil
}

func (m *MockMCPServiceExtendedImpl) GetVerificationStatus(ctx context.Context, id uuid.UUID) (*domain.MCPServerVerificationStatus, error) {
	if m.GetVerificationStatusFunc != nil {
		return m.GetVerificationStatusFunc(ctx, id)
	}
	return &domain.MCPServerVerificationStatus{}, nil
}

func (m *MockMCPServiceExtendedImpl) VerifyMCPCapability(ctx context.Context, mcpID uuid.UUID, capability string, resource string, targetService string, metadata map[string]interface{}) (allowed bool, reason string, auditID uuid.UUID, err error) {
	if m.VerifyMCPCapabilityFunc != nil {
		return m.VerifyMCPCapabilityFunc(ctx, mcpID, capability, resource, targetService, metadata)
	}
	return true, "allowed", uuid.New(), nil
}

func (m *MockMCPServiceExtendedImpl) GetConnectedAgents(ctx context.Context, mcpServerID uuid.UUID) ([]application.ConnectedAgent, error) {
	if m.GetConnectedAgentsFunc != nil {
		return m.GetConnectedAgentsFunc(ctx, mcpServerID)
	}
	return []application.ConnectedAgent{}, nil
}

func (m *MockMCPServiceExtendedImpl) GetConnectedAgentsCount(ctx context.Context, mcpServerID uuid.UUID) (int, error) {
	if m.GetConnectedAgentsCountFunc != nil {
		return m.GetConnectedAgentsCountFunc(ctx, mcpServerID)
	}
	return 0, nil
}

func (m *MockMCPServiceExtendedImpl) GenerateVerificationChallenge(ctx context.Context, serverID uuid.UUID) (string, error) {
	if m.GenerateVerificationChallengeFunc != nil {
		return m.GenerateVerificationChallengeFunc(ctx, serverID)
	}
	return "test-challenge", nil
}

// MockMCPCapabilityServiceImpl implements MCPCapabilityServicer interface
type MockMCPCapabilityServiceImpl struct {
	GetCapabilitiesFunc            func(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.MCPServerCapability, error)
	GetCapabilitiesByServerIDsFunc func(ctx context.Context, mcpServerIDs []uuid.UUID) (map[uuid.UUID][]*domain.MCPServerCapability, error)
	DetectCapabilitiesFunc         func(ctx context.Context, mcpServerID uuid.UUID) error
}

func (m *MockMCPCapabilityServiceImpl) GetCapabilities(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.MCPServerCapability, error) {
	if m.GetCapabilitiesFunc != nil {
		return m.GetCapabilitiesFunc(ctx, mcpServerID)
	}
	return []*domain.MCPServerCapability{}, nil
}

// GetCapabilitiesByServerIDs aggregates the per-server mocks for tests.
func (m *MockMCPCapabilityServiceImpl) GetCapabilitiesByServerIDs(ctx context.Context, mcpServerIDs []uuid.UUID) (map[uuid.UUID][]*domain.MCPServerCapability, error) {
	if m.GetCapabilitiesByServerIDsFunc != nil {
		return m.GetCapabilitiesByServerIDsFunc(ctx, mcpServerIDs)
	}
	result := make(map[uuid.UUID][]*domain.MCPServerCapability, len(mcpServerIDs))
	for _, serverID := range mcpServerIDs {
		caps, err := m.GetCapabilities(ctx, serverID)
		if err != nil {
			return nil, err
		}
		if len(caps) > 0 {
			result[serverID] = caps
		}
	}
	return result, nil
}

func (m *MockMCPCapabilityServiceImpl) DetectCapabilities(ctx context.Context, mcpServerID uuid.UUID) error {
	if m.DetectCapabilitiesFunc != nil {
		return m.DetectCapabilitiesFunc(ctx, mcpServerID)
	}
	return nil
}

// MockAgentRepositoryerImpl implements AgentRepositoryer interface
type MockAgentRepositoryerImpl struct {
	GetByIDFunc            func(id uuid.UUID) (*domain.Agent, error)
	GetByMCPServerFunc     func(mcpServerID, orgID uuid.UUID) ([]*domain.Agent, error)
	GetByMCPServerNameFunc func(mcpServerName string, orgID uuid.UUID) ([]*domain.Agent, error)
}

func (m *MockAgentRepositoryerImpl) GetByID(id uuid.UUID) (*domain.Agent, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(id)
	}
	return nil, nil
}

func (m *MockAgentRepositoryerImpl) GetByMCPServer(mcpServerID, orgID uuid.UUID) ([]*domain.Agent, error) {
	if m.GetByMCPServerFunc != nil {
		return m.GetByMCPServerFunc(mcpServerID, orgID)
	}
	return []*domain.Agent{}, nil
}

func (m *MockAgentRepositoryerImpl) GetByMCPServerName(mcpServerName string, orgID uuid.UUID) ([]*domain.Agent, error) {
	if m.GetByMCPServerNameFunc != nil {
		return m.GetByMCPServerNameFunc(mcpServerName, orgID)
	}
	return []*domain.Agent{}, nil
}

// MockMCPServerRepositoryerImpl satisfies the mcpServerByIDLookup
// single-method subset used by TagHandler (A3d-ii). Mirrors
// MockAgentRepositoryerImpl in shape.
type MockMCPServerRepositoryerImpl struct {
	GetByIDFunc func(id uuid.UUID) (*domain.MCPServer, error)
}

func (m *MockMCPServerRepositoryerImpl) GetByID(id uuid.UUID) (*domain.MCPServer, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(id)
	}
	return nil, nil
}

// MockVerificationEventRepositoryerImpl implements VerificationEventRepositoryer interface
type MockVerificationEventRepositoryerImpl struct {
	GetByMCPServerFunc func(mcpServerID uuid.UUID, limit, offset int) ([]*domain.VerificationEvent, int, error)
}

func (m *MockVerificationEventRepositoryerImpl) GetByMCPServer(mcpServerID uuid.UUID, limit, offset int) ([]*domain.VerificationEvent, int, error) {
	if m.GetByMCPServerFunc != nil {
		return m.GetByMCPServerFunc(mcpServerID, limit, offset)
	}
	return []*domain.VerificationEvent{}, 0, nil
}

// MockMCPAttestationServiceExtendedImpl implements MCPAttestationServicerExtended interface
type MockMCPAttestationServiceExtendedImpl struct {
	GetMCPServersForAgentFunc func(ctx context.Context, agentID uuid.UUID) ([]*domain.MCPServer, error)
	GetMCPAttestationsFunc    func(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.AttestationWithAgentDetails, float64, time.Time, error)
}

func (m *MockMCPAttestationServiceExtendedImpl) GetMCPServersForAgent(ctx context.Context, agentID uuid.UUID) ([]*domain.MCPServer, error) {
	if m.GetMCPServersForAgentFunc != nil {
		return m.GetMCPServersForAgentFunc(ctx, agentID)
	}
	return []*domain.MCPServer{}, nil
}

func (m *MockMCPAttestationServiceExtendedImpl) GetMCPAttestations(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.AttestationWithAgentDetails, float64, time.Time, error) {
	if m.GetMCPAttestationsFunc != nil {
		return m.GetMCPAttestationsFunc(ctx, mcpServerID)
	}
	return []*domain.AttestationWithAgentDetails{}, 0.0, time.Time{}, nil
}

// MockComplianceServiceExtendedImpl implements ComplianceServicerExtended interface
type MockComplianceServiceExtendedImpl struct {
	GetComplianceStatusFunc        func(ctx context.Context, orgID uuid.UUID) (interface{}, error)
	GetComplianceMetricsFunc       func(ctx context.Context, orgID uuid.UUID, startDate, endDate time.Time, interval string) (interface{}, error)
	GetAccessReviewFunc            func(ctx context.Context, orgID uuid.UUID) (interface{}, error)
	ListEvidenceFunc               func(ctx context.Context, orgID uuid.UUID, framework string, limit, offset int) ([]*domain.ComplianceEvidence, error)
	RunComplianceCheckFunc         func(ctx context.Context, orgID uuid.UUID, checkType string) (interface{}, error)
	ExportComplianceReportFullFunc func(ctx context.Context, orgID uuid.UUID, framework string, startDate, endDate time.Time, userID uuid.UUID) (*domain.ComplianceExportReport, error)
	ExportToCSVFunc                func(ctx context.Context, orgID uuid.UUID, framework string) (string, error)
	GetComplianceTrendingFunc      func(ctx context.Context, orgID uuid.UUID, framework string, startDate, endDate time.Time) (interface{}, error)
	RecordComplianceSnapshotFunc   func(ctx context.Context, orgID uuid.UUID, framework domain.ComplianceFramework) (*domain.ComplianceSnapshot, error)
	CollectEvidenceFunc            func(ctx context.Context, orgID uuid.UUID, framework domain.ComplianceFramework, checkName string, userID uuid.UUID) (*domain.ComplianceEvidence, error)
	GetEvidenceForCheckFunc        func(ctx context.Context, orgID uuid.UUID, checkName string) ([]*domain.ComplianceEvidence, error)
}

func (m *MockComplianceServiceExtendedImpl) GetComplianceStatus(ctx context.Context, orgID uuid.UUID) (interface{}, error) {
	if m.GetComplianceStatusFunc != nil {
		return m.GetComplianceStatusFunc(ctx, orgID)
	}
	return map[string]interface{}{"status": "ok"}, nil
}

func (m *MockComplianceServiceExtendedImpl) GetComplianceMetrics(ctx context.Context, orgID uuid.UUID, startDate, endDate time.Time, interval string) (interface{}, error) {
	if m.GetComplianceMetricsFunc != nil {
		return m.GetComplianceMetricsFunc(ctx, orgID, startDate, endDate, interval)
	}
	return map[string]interface{}{"metrics": []interface{}{}}, nil
}

func (m *MockComplianceServiceExtendedImpl) GetAccessReview(ctx context.Context, orgID uuid.UUID) (interface{}, error) {
	if m.GetAccessReviewFunc != nil {
		return m.GetAccessReviewFunc(ctx, orgID)
	}
	return map[string]interface{}{"review": []interface{}{}}, nil
}

func (m *MockComplianceServiceExtendedImpl) ListEvidence(ctx context.Context, orgID uuid.UUID, framework string, limit, offset int) ([]*domain.ComplianceEvidence, error) {
	if m.ListEvidenceFunc != nil {
		return m.ListEvidenceFunc(ctx, orgID, framework, limit, offset)
	}
	return []*domain.ComplianceEvidence{}, nil
}

func (m *MockComplianceServiceExtendedImpl) RunComplianceCheck(ctx context.Context, orgID uuid.UUID, checkType string) (interface{}, error) {
	if m.RunComplianceCheckFunc != nil {
		return m.RunComplianceCheckFunc(ctx, orgID, checkType)
	}
	return map[string]interface{}{"results": []interface{}{}}, nil
}

func (m *MockComplianceServiceExtendedImpl) ExportComplianceReportFull(ctx context.Context, orgID uuid.UUID, framework string, startDate, endDate time.Time, userID uuid.UUID) (*domain.ComplianceExportReport, error) {
	if m.ExportComplianceReportFullFunc != nil {
		return m.ExportComplianceReportFullFunc(ctx, orgID, framework, startDate, endDate, userID)
	}
	return &domain.ComplianceExportReport{}, nil
}

func (m *MockComplianceServiceExtendedImpl) ExportToCSV(ctx context.Context, orgID uuid.UUID, framework string) (string, error) {
	if m.ExportToCSVFunc != nil {
		return m.ExportToCSVFunc(ctx, orgID, framework)
	}
	return "header1,header2\nvalue1,value2", nil
}

func (m *MockComplianceServiceExtendedImpl) GetComplianceTrending(ctx context.Context, orgID uuid.UUID, framework string, startDate, endDate time.Time) (interface{}, error) {
	if m.GetComplianceTrendingFunc != nil {
		return m.GetComplianceTrendingFunc(ctx, orgID, framework, startDate, endDate)
	}
	return map[string]interface{}{"trending": []interface{}{}}, nil
}

func (m *MockComplianceServiceExtendedImpl) RecordComplianceSnapshot(ctx context.Context, orgID uuid.UUID, framework domain.ComplianceFramework) (*domain.ComplianceSnapshot, error) {
	if m.RecordComplianceSnapshotFunc != nil {
		return m.RecordComplianceSnapshotFunc(ctx, orgID, framework)
	}
	return &domain.ComplianceSnapshot{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Framework:      framework,
		Score:          85.0,
	}, nil
}

func (m *MockComplianceServiceExtendedImpl) CollectEvidence(ctx context.Context, orgID uuid.UUID, framework domain.ComplianceFramework, checkName string, userID uuid.UUID) (*domain.ComplianceEvidence, error) {
	if m.CollectEvidenceFunc != nil {
		return m.CollectEvidenceFunc(ctx, orgID, framework, checkName, userID)
	}
	return &domain.ComplianceEvidence{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Framework:      framework,
		CheckName:      checkName,
	}, nil
}

func (m *MockComplianceServiceExtendedImpl) GetEvidenceForCheck(ctx context.Context, orgID uuid.UUID, checkName string) ([]*domain.ComplianceEvidence, error) {
	if m.GetEvidenceForCheckFunc != nil {
		return m.GetEvidenceForCheckFunc(ctx, orgID, checkName)
	}
	return []*domain.ComplianceEvidence{}, nil
}

// ===========================
// AnalyticsHandler Mock Implementations
// ===========================

// MockVerificationEventServiceExtendedImpl implements VerificationEventServicerExtended interface
type MockVerificationEventServiceExtendedImpl struct {
	LogVerificationEventFunc     func(ctx context.Context, orgID uuid.UUID, agentID uuid.UUID, protocol domain.VerificationProtocol, verificationType domain.VerificationType, status domain.VerificationEventStatus, durationMs int, initiatorType domain.InitiatorType, initiatorID *uuid.UUID, metadata map[string]interface{}) (*domain.VerificationEvent, error)
	GetLast24HoursStatisticsFunc func(ctx context.Context, orgID uuid.UUID) (*domain.VerificationStatistics, error)
}

func (m *MockVerificationEventServiceExtendedImpl) LogVerificationEvent(ctx context.Context, orgID uuid.UUID, agentID uuid.UUID, protocol domain.VerificationProtocol, verificationType domain.VerificationType, status domain.VerificationEventStatus, durationMs int, initiatorType domain.InitiatorType, initiatorID *uuid.UUID, metadata map[string]interface{}) (*domain.VerificationEvent, error) {
	if m.LogVerificationEventFunc != nil {
		return m.LogVerificationEventFunc(ctx, orgID, agentID, protocol, verificationType, status, durationMs, initiatorType, initiatorID, metadata)
	}
	return &domain.VerificationEvent{ID: uuid.New(), OrganizationID: orgID, AgentID: &agentID}, nil
}

func (m *MockVerificationEventServiceExtendedImpl) GetLast24HoursStatistics(ctx context.Context, orgID uuid.UUID) (*domain.VerificationStatistics, error) {
	if m.GetLast24HoursStatisticsFunc != nil {
		return m.GetLast24HoursStatisticsFunc(ctx, orgID)
	}
	return &domain.VerificationStatistics{
		TotalVerifications: 10,
		SuccessCount:       8,
		FailedCount:        2,
		PendingCount:       0,
		AvgDurationMs:      150,
	}, nil
}

// MockSecurityServiceForAnalyticsImpl implements SecurityServicerForAnalytics interface
type MockSecurityServiceForAnalyticsImpl struct {
	CountOpenIncidentsFunc func(ctx context.Context, orgID uuid.UUID) (int, error)
	GetIncidentsFunc       func(ctx context.Context, orgID uuid.UUID, status domain.IncidentStatus, limit, offset int) ([]*domain.SecurityIncident, error)
}

func (m *MockSecurityServiceForAnalyticsImpl) CountOpenIncidents(ctx context.Context, orgID uuid.UUID) (int, error) {
	if m.CountOpenIncidentsFunc != nil {
		return m.CountOpenIncidentsFunc(ctx, orgID)
	}
	return 0, nil
}

func (m *MockSecurityServiceForAnalyticsImpl) GetIncidents(ctx context.Context, orgID uuid.UUID, status domain.IncidentStatus, limit, offset int) ([]*domain.SecurityIncident, error) {
	if m.GetIncidentsFunc != nil {
		return m.GetIncidentsFunc(ctx, orgID, status, limit, offset)
	}
	return []*domain.SecurityIncident{}, nil
}

// ===========================
// VerificationHandler Mock Implementations
// ===========================

// MockAgentServiceForVerificationImpl implements AgentServicerForVerification interface
type MockAgentServiceForVerificationImpl struct {
	MockAgentServiceImpl
	CreateSecurityAlertFunc func(ctx context.Context, alert *domain.Alert) error
}

func (m *MockAgentServiceForVerificationImpl) CreateSecurityAlert(ctx context.Context, alert *domain.Alert) error {
	if m.CreateSecurityAlertFunc != nil {
		return m.CreateSecurityAlertFunc(ctx, alert)
	}
	return nil
}

// MockAuditServiceForVerificationImpl implements AuditServicerForVerification interface
type MockAuditServiceForVerificationImpl struct {
	LogActionFunc        func(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error
	GetAgentActivityFunc func(ctx context.Context, orgID, agentID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error)
	GetAuditLogsFunc     func(ctx context.Context, orgID uuid.UUID, action string, entityType string, entityID *uuid.UUID, userID *uuid.UUID, startDate *time.Time, endDate *time.Time, limit int, offset int) ([]*domain.AuditLog, int, error)
	GetByIDFunc          func(ctx context.Context, id uuid.UUID) (*domain.AuditLog, error)
	LogFunc              func(ctx context.Context, entry *domain.AuditLog) error
}

func (m *MockAuditServiceForVerificationImpl) LogAction(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error {
	if m.LogActionFunc != nil {
		return m.LogActionFunc(ctx, orgID, userID, action, resourceType, resourceID, ipAddress, userAgent, metadata)
	}
	return nil
}

func (m *MockAuditServiceForVerificationImpl) LogAgentAction(ctx context.Context, orgID uuid.UUID, agentID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error {
	return nil
}

func (m *MockAuditServiceForVerificationImpl) GetAgentActivity(ctx context.Context, orgID, agentID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	if m.GetAgentActivityFunc != nil {
		return m.GetAgentActivityFunc(ctx, orgID, agentID, limit, offset)
	}
	return []*domain.AuditLog{}, nil
}

func (m *MockAuditServiceForVerificationImpl) GetAuditLogs(ctx context.Context, orgID uuid.UUID, action string, entityType string, entityID *uuid.UUID, userID *uuid.UUID, startDate *time.Time, endDate *time.Time, limit int, offset int) ([]*domain.AuditLog, int, error) {
	if m.GetAuditLogsFunc != nil {
		return m.GetAuditLogsFunc(ctx, orgID, action, entityType, entityID, userID, startDate, endDate, limit, offset)
	}
	return []*domain.AuditLog{}, 0, nil
}

func (m *MockAuditServiceForVerificationImpl) GetByID(ctx context.Context, id uuid.UUID) (*domain.AuditLog, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockAuditServiceForVerificationImpl) Log(ctx context.Context, entry *domain.AuditLog) error {
	if m.LogFunc != nil {
		return m.LogFunc(ctx, entry)
	}
	return nil
}

// MockAlertServiceForVerificationImpl implements AlertServicerForVerification interface
type MockAlertServiceForVerificationImpl struct {
	CreateAlertFunc                 func(ctx context.Context, alert *domain.Alert) error
	GetAlertsByAgentFunc            func(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.Alert, error)
	GetAlertsFunc                   func(ctx context.Context, orgID uuid.UUID, severity, status string, limit, offset int) ([]*domain.Alert, int, error)
	CountUnacknowledgedFunc         func(ctx context.Context, orgID uuid.UUID) (allCount, acknowledgedCount, unacknowledgedCount int, err error)
	CountBySeverityFunc             func(ctx context.Context, orgID uuid.UUID, status string) (critical, high, warning, info int, err error)
	DetectUnusualAccessPatternsFunc func(ctx context.Context, orgID uuid.UUID, agentID uuid.UUID) ([]*domain.Alert, error)
}

func (m *MockAlertServiceForVerificationImpl) CreateAlert(ctx context.Context, alert *domain.Alert) error {
	if m.CreateAlertFunc != nil {
		return m.CreateAlertFunc(ctx, alert)
	}
	return nil
}

func (m *MockAlertServiceForVerificationImpl) GetAlertsByAgent(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.Alert, error) {
	if m.GetAlertsByAgentFunc != nil {
		return m.GetAlertsByAgentFunc(ctx, agentID, limit, offset)
	}
	return []*domain.Alert{}, nil
}

func (m *MockAlertServiceForVerificationImpl) GetAlerts(ctx context.Context, orgID uuid.UUID, severity, status string, limit, offset int) ([]*domain.Alert, int, error) {
	if m.GetAlertsFunc != nil {
		return m.GetAlertsFunc(ctx, orgID, severity, status, limit, offset)
	}
	return []*domain.Alert{}, 0, nil
}

func (m *MockAlertServiceForVerificationImpl) CountUnacknowledged(ctx context.Context, orgID uuid.UUID) (allCount, acknowledgedCount, unacknowledgedCount int, err error) {
	if m.CountUnacknowledgedFunc != nil {
		return m.CountUnacknowledgedFunc(ctx, orgID)
	}
	return 0, 0, 0, nil
}

func (m *MockAlertServiceForVerificationImpl) CountBySeverity(ctx context.Context, orgID uuid.UUID, status string) (critical, high, warning, info int, err error) {
	if m.CountBySeverityFunc != nil {
		return m.CountBySeverityFunc(ctx, orgID, status)
	}
	return 0, 0, 0, 0, nil
}

func (m *MockAlertServiceForVerificationImpl) DetectUnusualAccessPatterns(ctx context.Context, orgID uuid.UUID, agentID uuid.UUID) ([]*domain.Alert, error) {
	if m.DetectUnusualAccessPatternsFunc != nil {
		return m.DetectUnusualAccessPatternsFunc(ctx, orgID, agentID)
	}
	return []*domain.Alert{}, nil
}

// MockVerificationEventServiceForVerificationImpl implements VerificationEventServicerForVerification interface
type MockVerificationEventServiceForVerificationImpl struct {
	LogVerificationEventFunc     func(ctx context.Context, orgID uuid.UUID, agentID uuid.UUID, protocol domain.VerificationProtocol, verificationType domain.VerificationType, status domain.VerificationEventStatus, durationMs int, initiatorType domain.InitiatorType, initiatorID *uuid.UUID, metadata map[string]interface{}) (*domain.VerificationEvent, error)
	CreateVerificationEventFunc  func(ctx context.Context, req *application.CreateVerificationEventRequest) (*domain.VerificationEvent, error)
	GetVerificationEventFunc     func(ctx context.Context, id uuid.UUID) (*domain.VerificationEvent, error)
	UpdateVerificationResultFunc func(ctx context.Context, id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error
	SearchVerificationsFunc      func(ctx context.Context, orgID uuid.UUID, params domain.VerificationQueryParams) ([]*domain.VerificationEvent, int, *domain.VerificationStatusCounts, error)
	UpdateExecutionStatusFunc    func(ctx context.Context, id uuid.UUID, executed bool, strictMode bool, executedAt time.Time, executionError *string) error
}

func (m *MockVerificationEventServiceForVerificationImpl) LogVerificationEvent(ctx context.Context, orgID uuid.UUID, agentID uuid.UUID, protocol domain.VerificationProtocol, verificationType domain.VerificationType, status domain.VerificationEventStatus, durationMs int, initiatorType domain.InitiatorType, initiatorID *uuid.UUID, metadata map[string]interface{}) (*domain.VerificationEvent, error) {
	if m.LogVerificationEventFunc != nil {
		return m.LogVerificationEventFunc(ctx, orgID, agentID, protocol, verificationType, status, durationMs, initiatorType, initiatorID, metadata)
	}
	return &domain.VerificationEvent{ID: uuid.New(), OrganizationID: orgID, AgentID: &agentID}, nil
}

func (m *MockVerificationEventServiceForVerificationImpl) CreateVerificationEvent(ctx context.Context, req *application.CreateVerificationEventRequest) (*domain.VerificationEvent, error) {
	if m.CreateVerificationEventFunc != nil {
		return m.CreateVerificationEventFunc(ctx, req)
	}
	return &domain.VerificationEvent{ID: uuid.New(), OrganizationID: req.OrganizationID, AgentID: &req.AgentID}, nil
}

func (m *MockVerificationEventServiceForVerificationImpl) GetVerificationEvent(ctx context.Context, id uuid.UUID) (*domain.VerificationEvent, error) {
	if m.GetVerificationEventFunc != nil {
		return m.GetVerificationEventFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockVerificationEventServiceForVerificationImpl) UpdateVerificationResult(ctx context.Context, id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error {
	if m.UpdateVerificationResultFunc != nil {
		return m.UpdateVerificationResultFunc(ctx, id, result, reason, metadata)
	}
	return nil
}

func (m *MockVerificationEventServiceForVerificationImpl) SearchVerifications(ctx context.Context, orgID uuid.UUID, params domain.VerificationQueryParams) ([]*domain.VerificationEvent, int, *domain.VerificationStatusCounts, error) {
	if m.SearchVerificationsFunc != nil {
		return m.SearchVerificationsFunc(ctx, orgID, params)
	}
	return []*domain.VerificationEvent{}, 0, nil, nil
}

func (m *MockVerificationEventServiceForVerificationImpl) UpdateExecutionStatus(ctx context.Context, id uuid.UUID, executed bool, strictMode bool, executedAt time.Time, executionError *string) error {
	if m.UpdateExecutionStatusFunc != nil {
		return m.UpdateExecutionStatusFunc(ctx, id, executed, strictMode, executedAt, executionError)
	}
	return nil
}

// MockOrganizationRepositoryerImpl implements OrganizationRepositoryer interface
type MockOrganizationRepositoryerImpl struct {
	GetByIDFunc func(id uuid.UUID) (*domain.Organization, error)
}

func (m *MockOrganizationRepositoryerImpl) GetByID(id uuid.UUID) (*domain.Organization, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(id)
	}
	return &domain.Organization{ID: id, EnforcementMode: domain.EnforcementModeMonitoring}, nil
}

// ===========================
// TrustScoreHandler Mocks
// ===========================

// MockTrustCalculatorServicerImpl implements TrustCalculatorServicer interface
type MockTrustCalculatorServicerImpl struct {
	CalculateTrustScoreFunc            func(ctx context.Context, agentID uuid.UUID) (*domain.TrustScore, error)
	GetLatestTrustScoreFunc            func(ctx context.Context, agentID uuid.UUID) (*domain.TrustScore, error)
	GetTrustScoreHistoryAuditTrailFunc func(ctx context.Context, agentID uuid.UUID, limit int) ([]*domain.TrustScoreHistoryEntry, error)
	RecordUserFeedbackFunc             func(ctx context.Context, agentID, orgID uuid.UUID, userID *uuid.UUID, rating int, comment string) (*domain.UserFeedback, error)
	RecordIsolationAttestationFunc     func(ctx context.Context, agentID uuid.UUID, sandbox domain.SandboxType, network domain.NetworkIsolation, filesystem domain.FilesystemIsolation, process domain.ProcessIsolation) (*domain.IsolationAttestation, error)
}

func (m *MockTrustCalculatorServicerImpl) CalculateTrustScore(ctx context.Context, agentID uuid.UUID) (*domain.TrustScore, error) {
	if m.CalculateTrustScoreFunc != nil {
		return m.CalculateTrustScoreFunc(ctx, agentID)
	}
	return &domain.TrustScore{
		AgentID:        agentID,
		Score:          0.85,
		Factors:        domain.TrustScoreFactors{VerificationStatus: 1.0, Uptime: 0.9, SuccessRate: 0.95},
		Confidence:     0.8,
		LastCalculated: time.Now(),
	}, nil
}

func (m *MockTrustCalculatorServicerImpl) GetLatestTrustScore(ctx context.Context, agentID uuid.UUID) (*domain.TrustScore, error) {
	if m.GetLatestTrustScoreFunc != nil {
		return m.GetLatestTrustScoreFunc(ctx, agentID)
	}
	return &domain.TrustScore{
		AgentID:        agentID,
		Score:          0.85,
		Factors:        domain.TrustScoreFactors{VerificationStatus: 1.0, Uptime: 0.9, SuccessRate: 0.95},
		Confidence:     0.8,
		LastCalculated: time.Now(),
	}, nil
}

func (m *MockTrustCalculatorServicerImpl) GetTrustScoreHistoryAuditTrail(ctx context.Context, agentID uuid.UUID, limit int) ([]*domain.TrustScoreHistoryEntry, error) {
	if m.GetTrustScoreHistoryAuditTrailFunc != nil {
		return m.GetTrustScoreHistoryAuditTrailFunc(ctx, agentID, limit)
	}
	return []*domain.TrustScoreHistoryEntry{}, nil
}

func (m *MockTrustCalculatorServicerImpl) RecordUserFeedback(ctx context.Context, agentID, orgID uuid.UUID, userID *uuid.UUID, rating int, comment string) (*domain.UserFeedback, error) {
	if m.RecordUserFeedbackFunc != nil {
		return m.RecordUserFeedbackFunc(ctx, agentID, orgID, userID, rating, comment)
	}
	return &domain.UserFeedback{
		ID:             uuid.New(),
		AgentID:        agentID,
		OrganizationID: orgID,
		UserID:         userID,
		Rating:         rating,
		Comment:        comment,
		CreatedAt:      time.Now(),
	}, nil
}

func (m *MockTrustCalculatorServicerImpl) RecordIsolationAttestation(ctx context.Context, agentID uuid.UUID, sandbox domain.SandboxType, network domain.NetworkIsolation, filesystem domain.FilesystemIsolation, process domain.ProcessIsolation) (*domain.IsolationAttestation, error) {
	if m.RecordIsolationAttestationFunc != nil {
		return m.RecordIsolationAttestationFunc(ctx, agentID, sandbox, network, filesystem, process)
	}
	return &domain.IsolationAttestation{
		ID:         uuid.New(),
		AgentID:    agentID,
		Sandbox:    sandbox,
		Network:    network,
		Filesystem: filesystem,
		Process:    process,
		Score:      domain.ScoreIsolation(sandbox, network, filesystem, process),
		ReportedAt: time.Now(),
		CreatedAt:  time.Now(),
	}, nil
}

// ===========================
// WebhookHandler Mocks
// ===========================

// MockWebhookServicerImpl implements WebhookServicer interface
type MockWebhookServicerImpl struct {
	CreateWebhookFunc func(ctx context.Context, req *application.CreateWebhookRequest, orgID, userID uuid.UUID) (*domain.Webhook, error)
	ListWebhooksFunc  func(ctx context.Context, orgID uuid.UUID) ([]*domain.Webhook, error)
	GetWebhookFunc    func(ctx context.Context, id uuid.UUID) (*domain.Webhook, error)
	DeleteWebhookFunc func(ctx context.Context, id uuid.UUID) error
	UpdateWebhookFunc func(ctx context.Context, id uuid.UUID, req *application.CreateWebhookRequest) (*domain.Webhook, error)
	TestWebhookFunc   func(ctx context.Context, id uuid.UUID) (*application.WebhookTestResult, error)
}

func (m *MockWebhookServicerImpl) CreateWebhook(ctx context.Context, req *application.CreateWebhookRequest, orgID, userID uuid.UUID) (*domain.Webhook, error) {
	if m.CreateWebhookFunc != nil {
		return m.CreateWebhookFunc(ctx, req, orgID, userID)
	}
	return &domain.Webhook{ID: uuid.New(), OrganizationID: orgID, Name: req.Name, URL: req.URL}, nil
}

func (m *MockWebhookServicerImpl) ListWebhooks(ctx context.Context, orgID uuid.UUID) ([]*domain.Webhook, error) {
	if m.ListWebhooksFunc != nil {
		return m.ListWebhooksFunc(ctx, orgID)
	}
	return []*domain.Webhook{}, nil
}

func (m *MockWebhookServicerImpl) GetWebhook(ctx context.Context, id uuid.UUID) (*domain.Webhook, error) {
	if m.GetWebhookFunc != nil {
		return m.GetWebhookFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockWebhookServicerImpl) DeleteWebhook(ctx context.Context, id uuid.UUID) error {
	if m.DeleteWebhookFunc != nil {
		return m.DeleteWebhookFunc(ctx, id)
	}
	return nil
}

func (m *MockWebhookServicerImpl) UpdateWebhook(ctx context.Context, id uuid.UUID, req *application.CreateWebhookRequest) (*domain.Webhook, error) {
	if m.UpdateWebhookFunc != nil {
		return m.UpdateWebhookFunc(ctx, id, req)
	}
	return &domain.Webhook{ID: id, Name: req.Name, URL: req.URL}, nil
}

func (m *MockWebhookServicerImpl) TestWebhook(ctx context.Context, id uuid.UUID) (*application.WebhookTestResult, error) {
	if m.TestWebhookFunc != nil {
		return m.TestWebhookFunc(ctx, id)
	}
	return &application.WebhookTestResult{Success: true, StatusCode: 200}, nil
}

// GetAgentTagsByAgentIDs aggregates the per-agent mocks for tests.
func (m *MockTagServiceImpl) GetAgentTagsByAgentIDs(ctx context.Context, agentIDs []uuid.UUID) (map[uuid.UUID][]*domain.Tag, error) {
	if m.GetAgentTagsByAgentIDsFunc != nil {
		return m.GetAgentTagsByAgentIDsFunc(ctx, agentIDs)
	}
	result := make(map[uuid.UUID][]*domain.Tag, len(agentIDs))
	for _, agentID := range agentIDs {
		tags, err := m.GetAgentTags(ctx, agentID)
		if err != nil {
			return nil, err
		}
		if len(tags) > 0 {
			result[agentID] = tags
		}
	}
	return result, nil
}

// GetCapabilitiesByAgentIDs aggregates the per-agent mocks for tests.
func (m *MockCapabilityServiceImpl) GetCapabilitiesByAgentIDs(ctx context.Context, agentIDs []uuid.UUID, activeOnly bool) (map[uuid.UUID][]*domain.AgentCapability, error) {
	if m.GetCapabilitiesByAgentIDsFunc != nil {
		return m.GetCapabilitiesByAgentIDsFunc(ctx, agentIDs, activeOnly)
	}
	result := make(map[uuid.UUID][]*domain.AgentCapability, len(agentIDs))
	for _, agentID := range agentIDs {
		caps, err := m.GetAgentCapabilities(ctx, agentID, activeOnly)
		if err != nil {
			return nil, err
		}
		if len(caps) > 0 {
			result[agentID] = caps
		}
	}
	return result, nil
}

// ListAgentsPaged pages over ListAgents for tests.
func (m *MockAgentServiceImpl) ListAgentsPaged(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Agent, int, error) {
	if m.ListAgentsPagedFunc != nil {
		return m.ListAgentsPagedFunc(ctx, orgID, limit, offset)
	}
	agents, err := m.ListAgents(ctx, orgID)
	if err != nil {
		return nil, 0, err
	}
	total := len(agents)
	if limit > 0 {
		if offset >= len(agents) {
			agents = nil
		} else {
			agents = agents[offset:]
		}
		if len(agents) > limit {
			agents = agents[:limit]
		}
	}
	return agents, total, nil
}

func (m *MockAgentRepositoryerImpl) ListRevokedIDs(limit, offset int) ([]uuid.UUID, error) {
	return nil, nil
}
