package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/mock"
)

// ========================================
// Shared Mock Repositories
// Prefixed with "Shared" to avoid conflicts
// ========================================

// SharedMockAgentRepository is a mock implementation of AgentRepository
type SharedMockAgentRepository struct {
	mock.Mock
}

func (m *SharedMockAgentRepository) Create(agent *domain.Agent) error {
	args := m.Called(agent)
	return args.Error(0)
}

func (m *SharedMockAgentRepository) GetByID(id uuid.UUID) (*domain.Agent, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *SharedMockAgentRepository) GetByName(orgID uuid.UUID, name string) (*domain.Agent, error) {
	args := m.Called(orgID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *SharedMockAgentRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.Agent, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *SharedMockAgentRepository) Update(agent *domain.Agent) error {
	args := m.Called(agent)
	return args.Error(0)
}

func (m *SharedMockAgentRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *SharedMockAgentRepository) List(limit, offset int) ([]*domain.Agent, error) {
	args := m.Called(limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *SharedMockAgentRepository) UpdateTrustScore(id uuid.UUID, newScore float64) error {
	args := m.Called(id, newScore)
	return args.Error(0)
}

func (m *SharedMockAgentRepository) IncrementViolationCount(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *SharedMockAgentRepository) MarkAsCompromised(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *SharedMockAgentRepository) UpdateLastActive(ctx context.Context, agentID uuid.UUID) error {
	args := m.Called(ctx, agentID)
	return args.Error(0)
}

func (m *SharedMockAgentRepository) GetStaleAgents(ctx context.Context, staleSince time.Time) ([]*domain.Agent, error) {
	args := m.Called(ctx, staleSince)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *SharedMockAgentRepository) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*domain.Agent, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

// SharedMockAlertRepository is a mock implementation of AlertRepository
type SharedMockAlertRepository struct {
	mock.Mock
}

func (m *SharedMockAlertRepository) Create(alert *domain.Alert) error {
	args := m.Called(alert)
	return args.Error(0)
}

func (m *SharedMockAlertRepository) GetByID(id uuid.UUID) (*domain.Alert, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Alert), args.Error(1)
}

func (m *SharedMockAlertRepository) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.Alert, error) {
	args := m.Called(orgID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *SharedMockAlertRepository) GetUnacknowledged(orgID uuid.UUID) ([]*domain.Alert, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *SharedMockAlertRepository) CountUnacknowledged(orgID uuid.UUID) (int, error) {
	args := m.Called(orgID)
	return args.Int(0), args.Error(1)
}

func (m *SharedMockAlertRepository) CountBySeverity(orgID uuid.UUID) (map[string]int, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *SharedMockAlertRepository) Update(alert *domain.Alert) error {
	args := m.Called(alert)
	return args.Error(0)
}

func (m *SharedMockAlertRepository) GetRecentByAgentAndType(agentID uuid.UUID, alertType domain.AlertType, since time.Time) ([]*domain.Alert, error) {
	args := m.Called(agentID, alertType, since)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *SharedMockAlertRepository) GetRecentByMCPServerAndType(mcpServerID uuid.UUID, alertType domain.AlertType, since time.Time) ([]*domain.Alert, error) {
	args := m.Called(mcpServerID, alertType, since)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

// SharedMockMCPServerRepository is a mock implementation of MCPServerRepository
type SharedMockMCPServerRepository struct {
	mock.Mock
}

func (m *SharedMockMCPServerRepository) Create(server *domain.MCPServer) error {
	args := m.Called(server)
	return args.Error(0)
}

func (m *SharedMockMCPServerRepository) GetByID(id uuid.UUID) (*domain.MCPServer, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MCPServer), args.Error(1)
}

func (m *SharedMockMCPServerRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.MCPServer, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.MCPServer), args.Error(1)
}

func (m *SharedMockMCPServerRepository) GetByURL(url string, orgID uuid.UUID) (*domain.MCPServer, error) {
	args := m.Called(url, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MCPServer), args.Error(1)
}

func (m *SharedMockMCPServerRepository) Update(server *domain.MCPServer) error {
	args := m.Called(server)
	return args.Error(0)
}

func (m *SharedMockMCPServerRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *SharedMockMCPServerRepository) List(limit, offset int) ([]*domain.MCPServer, error) {
	args := m.Called(limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.MCPServer), args.Error(1)
}

func (m *SharedMockMCPServerRepository) GetVerificationStatus(id uuid.UUID) (*domain.MCPServerVerificationStatus, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MCPServerVerificationStatus), args.Error(1)
}

func (m *SharedMockMCPServerRepository) AddPublicKey(ctx context.Context, serverID uuid.UUID, publicKey string, keyType string) error {
	args := m.Called(ctx, serverID, publicKey, keyType)
	return args.Error(0)
}

func (m *SharedMockMCPServerRepository) VerifyServer(ctx context.Context, serverID uuid.UUID) error {
	args := m.Called(ctx, serverID)
	return args.Error(0)
}

func (m *SharedMockMCPServerRepository) GetByName(orgID uuid.UUID, name string) (*domain.MCPServer, error) {
	args := m.Called(orgID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MCPServer), args.Error(1)
}

// SharedMockTrustScoreRepository is a mock implementation of TrustScoreRepository
type SharedMockTrustScoreRepository struct {
	mock.Mock
}

func (m *SharedMockTrustScoreRepository) Create(score *domain.TrustScore) error {
	args := m.Called(score)
	return args.Error(0)
}

func (m *SharedMockTrustScoreRepository) GetByAgent(agentID uuid.UUID) (*domain.TrustScore, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TrustScore), args.Error(1)
}

func (m *SharedMockTrustScoreRepository) GetLatest(agentID uuid.UUID) (*domain.TrustScore, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TrustScore), args.Error(1)
}

func (m *SharedMockTrustScoreRepository) GetHistory(agentID uuid.UUID, limit int) ([]*domain.TrustScore, error) {
	args := m.Called(agentID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TrustScore), args.Error(1)
}

func (m *SharedMockTrustScoreRepository) GetHistoryAuditTrail(agentID uuid.UUID, limit int) ([]*domain.TrustScoreHistoryEntry, error) {
	args := m.Called(agentID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TrustScoreHistoryEntry), args.Error(1)
}

// SharedMockMCPTrustScoreRepository is a mock implementation of MCPTrustScoreRepository
type SharedMockMCPTrustScoreRepository struct {
	mock.Mock
}

func (m *SharedMockMCPTrustScoreRepository) Create(score *domain.MCPTrustScore) error {
	args := m.Called(score)
	return args.Error(0)
}

func (m *SharedMockMCPTrustScoreRepository) GetByMCPServer(mcpServerID uuid.UUID) (*domain.MCPTrustScore, error) {
	args := m.Called(mcpServerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MCPTrustScore), args.Error(1)
}

func (m *SharedMockMCPTrustScoreRepository) GetLatest(mcpServerID uuid.UUID) (*domain.MCPTrustScore, error) {
	args := m.Called(mcpServerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MCPTrustScore), args.Error(1)
}

func (m *SharedMockMCPTrustScoreRepository) GetHistory(mcpServerID uuid.UUID, limit int) ([]*domain.MCPTrustScore, error) {
	args := m.Called(mcpServerID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.MCPTrustScore), args.Error(1)
}

// SharedMockSecurityPolicyRepository is a mock implementation of SecurityPolicyRepository
type SharedMockSecurityPolicyRepository struct {
	mock.Mock
}

func (m *SharedMockSecurityPolicyRepository) Create(policy *domain.SecurityPolicy) error {
	args := m.Called(policy)
	return args.Error(0)
}

func (m *SharedMockSecurityPolicyRepository) GetByID(id uuid.UUID) (*domain.SecurityPolicy, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SecurityPolicy), args.Error(1)
}

func (m *SharedMockSecurityPolicyRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.SecurityPolicy, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SecurityPolicy), args.Error(1)
}

func (m *SharedMockSecurityPolicyRepository) GetEnabledByOrganization(orgID uuid.UUID) ([]*domain.SecurityPolicy, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SecurityPolicy), args.Error(1)
}

func (m *SharedMockSecurityPolicyRepository) GetByType(orgID uuid.UUID, policyType domain.PolicyType) ([]*domain.SecurityPolicy, error) {
	args := m.Called(orgID, policyType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SecurityPolicy), args.Error(1)
}

func (m *SharedMockSecurityPolicyRepository) Update(policy *domain.SecurityPolicy) error {
	args := m.Called(policy)
	return args.Error(0)
}

func (m *SharedMockSecurityPolicyRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

// SharedMockCapabilityRepository is a mock implementation of CapabilityRepository
type SharedMockCapabilityRepository struct {
	mock.Mock
}

func (m *SharedMockCapabilityRepository) CreateCapability(capability *domain.AgentCapability) error {
	args := m.Called(capability)
	return args.Error(0)
}

func (m *SharedMockCapabilityRepository) GetCapabilityByID(id uuid.UUID) (*domain.AgentCapability, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AgentCapability), args.Error(1)
}

func (m *SharedMockCapabilityRepository) GetCapabilitiesByAgentID(agentID uuid.UUID) ([]*domain.AgentCapability, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AgentCapability), args.Error(1)
}

func (m *SharedMockCapabilityRepository) GetActiveCapabilitiesByAgentID(agentID uuid.UUID) ([]*domain.AgentCapability, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AgentCapability), args.Error(1)
}

func (m *SharedMockCapabilityRepository) RevokeCapability(id uuid.UUID, revokedAt time.Time) error {
	args := m.Called(id, revokedAt)
	return args.Error(0)
}

func (m *SharedMockCapabilityRepository) DeleteCapability(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *SharedMockCapabilityRepository) UpdateCapability(capability *domain.AgentCapability) error {
	args := m.Called(capability)
	return args.Error(0)
}

// SharedMockAuditLogRepository is a mock implementation of AuditLogRepository
type SharedMockAuditLogRepository struct {
	mock.Mock
}

func (m *SharedMockAuditLogRepository) Create(log *domain.AuditLog) error {
	args := m.Called(log)
	return args.Error(0)
}

func (m *SharedMockAuditLogRepository) GetByAgent(orgID, agentID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	args := m.Called(orgID, agentID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *SharedMockAuditLogRepository) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	args := m.Called(orgID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *SharedMockAuditLogRepository) GetSuccessRate(agentID uuid.UUID, since time.Time) (float64, error) {
	args := m.Called(agentID, since)
	return args.Get(0).(float64), args.Error(1)
}

func (m *SharedMockAuditLogRepository) GetRecentLogs(agentID uuid.UUID, limit int) ([]*domain.AuditLog, error) {
	args := m.Called(agentID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *SharedMockAuditLogRepository) GetByID(id uuid.UUID) (*domain.AuditLog, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AuditLog), args.Error(1)
}

func (m *SharedMockAuditLogRepository) GetByUser(orgID, userID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	args := m.Called(orgID, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *SharedMockAuditLogRepository) GetByResource(orgID uuid.UUID, resourceType string, resourceID uuid.UUID) ([]*domain.AuditLog, error) {
	args := m.Called(orgID, resourceType, resourceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *SharedMockAuditLogRepository) Search(query string, limit, offset int) ([]*domain.AuditLog, error) {
	args := m.Called(query, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *SharedMockAuditLogRepository) CountActionsByAgentInTimeWindow(agentID uuid.UUID, action domain.AuditAction, windowMinutes int) (int, error) {
	args := m.Called(agentID, action, windowMinutes)
	return args.Int(0), args.Error(1)
}

func (m *SharedMockAuditLogRepository) GetRecentActionsByAgent(agentID uuid.UUID, limit int) ([]*domain.AuditLog, error) {
	args := m.Called(agentID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *SharedMockAuditLogRepository) GetAgentActionsByIPAddress(agentID uuid.UUID, ipAddress string, limit int) ([]*domain.AuditLog, error) {
	args := m.Called(agentID, ipAddress, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

// SharedMockAPIKeyRepository is a mock implementation of APIKeyRepository
type SharedMockAPIKeyRepository struct {
	mock.Mock
}

func (m *SharedMockAPIKeyRepository) Create(key *domain.APIKey) error {
	args := m.Called(key)
	return args.Error(0)
}

func (m *SharedMockAPIKeyRepository) GetByID(id uuid.UUID) (*domain.APIKey, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func (m *SharedMockAPIKeyRepository) GetByKeyHash(keyHash string) (*domain.APIKey, error) {
	args := m.Called(keyHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func (m *SharedMockAPIKeyRepository) GetByAgent(agentID uuid.UUID) ([]*domain.APIKey, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.APIKey), args.Error(1)
}

func (m *SharedMockAPIKeyRepository) Update(key *domain.APIKey) error {
	args := m.Called(key)
	return args.Error(0)
}

func (m *SharedMockAPIKeyRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *SharedMockAPIKeyRepository) GetExpiringSoon(within time.Duration) ([]*domain.APIKey, error) {
	args := m.Called(within)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.APIKey), args.Error(1)
}

func (m *SharedMockAPIKeyRepository) RecordUsage(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *SharedMockAPIKeyRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.APIKey, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.APIKey), args.Error(1)
}

// SharedMockUserRepository is a mock implementation of UserRepository
type SharedMockUserRepository struct {
	mock.Mock
}

func (m *SharedMockUserRepository) Create(user *domain.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *SharedMockUserRepository) GetByID(id uuid.UUID) (*domain.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *SharedMockUserRepository) GetByEmail(email string) (*domain.User, error) {
	args := m.Called(email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *SharedMockUserRepository) GetByPasswordResetToken(resetToken string) (*domain.User, error) {
	args := m.Called(resetToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *SharedMockUserRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.User, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.User), args.Error(1)
}

func (m *SharedMockUserRepository) GetByOrganizationAndStatus(orgID uuid.UUID, status domain.UserStatus) ([]*domain.User, error) {
	args := m.Called(orgID, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.User), args.Error(1)
}

func (m *SharedMockUserRepository) Update(user *domain.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *SharedMockUserRepository) UpdateRole(id uuid.UUID, role domain.UserRole) error {
	args := m.Called(id, role)
	return args.Error(0)
}

func (m *SharedMockUserRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *SharedMockUserRepository) CountActiveUsers(orgID uuid.UUID, withinMinutes int) (int, error) {
	args := m.Called(orgID, withinMinutes)
	return args.Int(0), args.Error(1)
}

// SharedMockOrganizationRepository is a mock implementation of OrganizationRepository
type SharedMockOrganizationRepository struct {
	mock.Mock
}

func (m *SharedMockOrganizationRepository) Create(org *domain.Organization) error {
	args := m.Called(org)
	return args.Error(0)
}

func (m *SharedMockOrganizationRepository) GetByID(id uuid.UUID) (*domain.Organization, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Organization), args.Error(1)
}

func (m *SharedMockOrganizationRepository) GetByDomain(domainName string) (*domain.Organization, error) {
	args := m.Called(domainName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Organization), args.Error(1)
}

func (m *SharedMockOrganizationRepository) Update(org *domain.Organization) error {
	args := m.Called(org)
	return args.Error(0)
}

func (m *SharedMockOrganizationRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *SharedMockOrganizationRepository) ListAll() ([]*domain.Organization, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Organization), args.Error(1)
}

// SharedMockSDKTokenRepository is a mock implementation of SDKTokenRepository
type SharedMockSDKTokenRepository struct {
	mock.Mock
}

func (m *SharedMockSDKTokenRepository) Create(token *domain.SDKToken) error {
	args := m.Called(token)
	return args.Error(0)
}

func (m *SharedMockSDKTokenRepository) GetByID(id uuid.UUID) (*domain.SDKToken, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SDKToken), args.Error(1)
}

func (m *SharedMockSDKTokenRepository) GetByTokenID(tokenID string) (*domain.SDKToken, error) {
	args := m.Called(tokenID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SDKToken), args.Error(1)
}

func (m *SharedMockSDKTokenRepository) GetByTokenHash(tokenHash string) (*domain.SDKToken, error) {
	args := m.Called(tokenHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SDKToken), args.Error(1)
}

func (m *SharedMockSDKTokenRepository) GetByUserID(userID uuid.UUID, includeRevoked bool) ([]*domain.SDKToken, error) {
	args := m.Called(userID, includeRevoked)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SDKToken), args.Error(1)
}

func (m *SharedMockSDKTokenRepository) GetByOrganizationID(organizationID uuid.UUID, includeRevoked bool) ([]*domain.SDKToken, error) {
	args := m.Called(organizationID, includeRevoked)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SDKToken), args.Error(1)
}

func (m *SharedMockSDKTokenRepository) Update(token *domain.SDKToken) error {
	args := m.Called(token)
	return args.Error(0)
}

func (m *SharedMockSDKTokenRepository) Revoke(id uuid.UUID, reason string) error {
	args := m.Called(id, reason)
	return args.Error(0)
}

func (m *SharedMockSDKTokenRepository) RevokeByTokenHash(tokenHash string, reason string) error {
	args := m.Called(tokenHash, reason)
	return args.Error(0)
}

func (m *SharedMockSDKTokenRepository) RevokeAllForUser(userID uuid.UUID, reason string) error {
	args := m.Called(userID, reason)
	return args.Error(0)
}

func (m *SharedMockSDKTokenRepository) RecordUsage(tokenID string, ipAddress string) error {
	args := m.Called(tokenID, ipAddress)
	return args.Error(0)
}

func (m *SharedMockSDKTokenRepository) DeleteExpired() error {
	args := m.Called()
	return args.Error(0)
}

func (m *SharedMockSDKTokenRepository) GetActiveCount(userID uuid.UUID) (int, error) {
	args := m.Called(userID)
	return args.Int(0), args.Error(1)
}

// SharedMockVerificationEventRepository is a mock implementation of VerificationEventRepository
type SharedMockVerificationEventRepository struct {
	mock.Mock
}

func (m *SharedMockVerificationEventRepository) Create(event *domain.VerificationEvent) error {
	args := m.Called(event)
	return args.Error(0)
}

func (m *SharedMockVerificationEventRepository) GetByID(id uuid.UUID) (*domain.VerificationEvent, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VerificationEvent), args.Error(1)
}

func (m *SharedMockVerificationEventRepository) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.VerificationEvent, int, error) {
	args := m.Called(orgID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Int(1), args.Error(2)
}

func (m *SharedMockVerificationEventRepository) GetByAgent(agentID uuid.UUID, limit, offset int) ([]*domain.VerificationEvent, int, error) {
	args := m.Called(agentID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Int(1), args.Error(2)
}

func (m *SharedMockVerificationEventRepository) GetByMCPServer(mcpServerID uuid.UUID, limit, offset int) ([]*domain.VerificationEvent, int, error) {
	args := m.Called(mcpServerID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Int(1), args.Error(2)
}

func (m *SharedMockVerificationEventRepository) GetRecentEvents(orgID uuid.UUID, minutes int) ([]*domain.VerificationEvent, error) {
	args := m.Called(orgID, minutes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Error(1)
}

func (m *SharedMockVerificationEventRepository) GetPendingVerifications(orgID uuid.UUID) ([]*domain.VerificationEvent, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Error(1)
}

func (m *SharedMockVerificationEventRepository) SearchAdminVerifications(orgID uuid.UUID, params domain.VerificationQueryParams) ([]*domain.VerificationEvent, int, *domain.VerificationStatusCounts, error) {
	args := m.Called(orgID, params)
	if args.Get(0) == nil {
		return nil, args.Int(1), nil, args.Error(3)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Int(1), args.Get(2).(*domain.VerificationStatusCounts), args.Error(3)
}

func (m *SharedMockVerificationEventRepository) GetStatistics(orgID uuid.UUID, startTime, endTime time.Time) (*domain.VerificationStatistics, error) {
	args := m.Called(orgID, startTime, endTime)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VerificationStatistics), args.Error(1)
}

func (m *SharedMockVerificationEventRepository) GetAgentStatistics(agentID uuid.UUID, startTime, endTime time.Time) (*domain.AgentVerificationStatistics, error) {
	args := m.Called(agentID, startTime, endTime)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AgentVerificationStatistics), args.Error(1)
}

func (m *SharedMockVerificationEventRepository) UpdateResult(id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error {
	args := m.Called(id, result, reason, metadata)
	return args.Error(0)
}

func (m *SharedMockVerificationEventRepository) UpdateExecutionStatus(id uuid.UUID, executed bool, strictMode bool, executedAt time.Time, executionError *string) error {
	args := m.Called(id, executed, strictMode, executedAt, executionError)
	return args.Error(0)
}

func (m *SharedMockVerificationEventRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

// SharedMockTagRepository is a mock implementation of TagRepository
type SharedMockTagRepository struct {
	mock.Mock
}

func (m *SharedMockTagRepository) Create(ctx context.Context, tag *domain.Tag) error {
	args := m.Called(ctx, tag)
	return args.Error(0)
}

func (m *SharedMockTagRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tag, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tag), args.Error(1)
}

func (m *SharedMockTagRepository) Update(ctx context.Context, tag *domain.Tag) error {
	args := m.Called(ctx, tag)
	return args.Error(0)
}

func (m *SharedMockTagRepository) List(ctx context.Context, organizationID uuid.UUID, category *domain.TagCategory) ([]*domain.Tag, error) {
	args := m.Called(ctx, organizationID, category)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Tag), args.Error(1)
}

func (m *SharedMockTagRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *SharedMockTagRepository) GetPopularTags(ctx context.Context, organizationID uuid.UUID, limit int) ([]*domain.Tag, error) {
	args := m.Called(ctx, organizationID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Tag), args.Error(1)
}

func (m *SharedMockTagRepository) SearchTags(ctx context.Context, organizationID uuid.UUID, query string, category *domain.TagCategory) ([]*domain.Tag, error) {
	args := m.Called(ctx, organizationID, query, category)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Tag), args.Error(1)
}

func (m *SharedMockTagRepository) AddTagsToAgent(ctx context.Context, agentID uuid.UUID, tagIDs []uuid.UUID) error {
	args := m.Called(ctx, agentID, tagIDs)
	return args.Error(0)
}

func (m *SharedMockTagRepository) RemoveTagFromAgent(ctx context.Context, agentID uuid.UUID, tagID uuid.UUID) error {
	args := m.Called(ctx, agentID, tagID)
	return args.Error(0)
}

func (m *SharedMockTagRepository) GetAgentTags(ctx context.Context, agentID uuid.UUID) ([]*domain.Tag, error) {
	args := m.Called(ctx, agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Tag), args.Error(1)
}

func (m *SharedMockTagRepository) AddTagsToMCPServer(ctx context.Context, mcpServerID uuid.UUID, tagIDs []uuid.UUID) error {
	args := m.Called(ctx, mcpServerID, tagIDs)
	return args.Error(0)
}

func (m *SharedMockTagRepository) RemoveTagFromMCPServer(ctx context.Context, mcpServerID uuid.UUID, tagID uuid.UUID) error {
	args := m.Called(ctx, mcpServerID, tagID)
	return args.Error(0)
}

func (m *SharedMockTagRepository) GetMCPServerTags(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.Tag, error) {
	args := m.Called(ctx, mcpServerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Tag), args.Error(1)
}

// GetByOrganizationPaged pages over GetByOrganization for tests.
func (m *SharedMockAgentRepository) GetByOrganizationPaged(orgID uuid.UUID, limit, offset int) ([]*domain.Agent, int, error) {
	agents, err := m.GetByOrganization(orgID)
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

// GetCapabilitiesByAgentIDs aggregates the per-agent mocks for tests.
func (m *SharedMockCapabilityRepository) GetCapabilitiesByAgentIDs(agentIDs []uuid.UUID, activeOnly bool) (map[uuid.UUID][]*domain.AgentCapability, error) {
	result := make(map[uuid.UUID][]*domain.AgentCapability, len(agentIDs))
	for _, agentID := range agentIDs {
		var caps []*domain.AgentCapability
		var err error
		if activeOnly {
			caps, err = m.GetActiveCapabilitiesByAgentID(agentID)
		} else {
			caps, err = m.GetCapabilitiesByAgentID(agentID)
		}
		if err != nil {
			return nil, err
		}
		if len(caps) > 0 {
			result[agentID] = caps
		}
	}
	return result, nil
}

// GetAgentTagsByAgentIDs aggregates the per-agent mocks for tests.
func (m *SharedMockTagRepository) GetAgentTagsByAgentIDs(ctx context.Context, agentIDs []uuid.UUID) (map[uuid.UUID][]*domain.Tag, error) {
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
