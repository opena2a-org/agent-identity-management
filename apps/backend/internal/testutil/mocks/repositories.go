package mocks

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/mock"
)

// MockAgentRepository is a mock implementation of AgentRepository
type MockAgentRepository struct {
	mock.Mock
}

func (m *MockAgentRepository) Create(agent *domain.Agent) error {
	args := m.Called(agent)
	return args.Error(0)
}

func (m *MockAgentRepository) GetByID(id uuid.UUID) (*domain.Agent, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *MockAgentRepository) GetByName(orgID uuid.UUID, name string) (*domain.Agent, error) {
	args := m.Called(orgID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *MockAgentRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.Agent, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *MockAgentRepository) Update(agent *domain.Agent) error {
	args := m.Called(agent)
	return args.Error(0)
}

func (m *MockAgentRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAgentRepository) List(limit, offset int) ([]*domain.Agent, error) {
	args := m.Called(limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *MockAgentRepository) ListRevokedIDs(limit, offset int) ([]uuid.UUID, error) {
	args := m.Called(limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]uuid.UUID), args.Error(1)
}

func (m *MockAgentRepository) UpdateTrustScore(id uuid.UUID, newScore float64) error {
	args := m.Called(id, newScore)
	return args.Error(0)
}

func (m *MockAgentRepository) MarkAsCompromised(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAgentRepository) UpdateLastActive(ctx context.Context, agentID uuid.UUID) error {
	args := m.Called(ctx, agentID)
	return args.Error(0)
}

func (m *MockAgentRepository) GetStaleAgents(ctx context.Context, staleSince time.Time) ([]*domain.Agent, error) {
	args := m.Called(ctx, staleSince)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *MockAgentRepository) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*domain.Agent, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

// MockAlertRepository is a mock implementation of AlertRepository
type MockAlertRepository struct {
	mock.Mock
}

func (m *MockAlertRepository) Create(alert *domain.Alert) error {
	args := m.Called(alert)
	return args.Error(0)
}

func (m *MockAlertRepository) GetByID(id uuid.UUID) (*domain.Alert, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Alert), args.Error(1)
}

func (m *MockAlertRepository) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.Alert, error) {
	args := m.Called(orgID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *MockAlertRepository) GetUnacknowledged(orgID uuid.UUID) ([]*domain.Alert, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *MockAlertRepository) CountUnacknowledged(orgID uuid.UUID) (int, error) {
	args := m.Called(orgID)
	return args.Int(0), args.Error(1)
}

func (m *MockAlertRepository) CountBySeverity(orgID uuid.UUID) (map[string]int, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockAlertRepository) Update(alert *domain.Alert) error {
	args := m.Called(alert)
	return args.Error(0)
}

func (m *MockAlertRepository) GetRecentByAgentAndType(agentID uuid.UUID, alertType domain.AlertType, since time.Time) ([]*domain.Alert, error) {
	args := m.Called(agentID, alertType, since)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *MockAlertRepository) GetRecentByMCPServerAndType(mcpServerID uuid.UUID, alertType domain.AlertType, since time.Time) ([]*domain.Alert, error) {
	args := m.Called(mcpServerID, alertType, since)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

// MockOrganizationRepository is a mock implementation of OrganizationRepository
type MockOrganizationRepository struct {
	mock.Mock
}

func (m *MockOrganizationRepository) Create(org *domain.Organization) error {
	args := m.Called(org)
	return args.Error(0)
}

func (m *MockOrganizationRepository) GetByID(id uuid.UUID) (*domain.Organization, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Organization), args.Error(1)
}

func (m *MockOrganizationRepository) GetByName(name string) (*domain.Organization, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Organization), args.Error(1)
}

func (m *MockOrganizationRepository) Update(org *domain.Organization) error {
	args := m.Called(org)
	return args.Error(0)
}

func (m *MockOrganizationRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockOrganizationRepository) List(limit, offset int) ([]*domain.Organization, error) {
	args := m.Called(limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Organization), args.Error(1)
}

func (m *MockOrganizationRepository) ListAll() ([]*domain.Organization, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Organization), args.Error(1)
}

// MockUserRepository is a mock implementation of UserRepository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(user *domain.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockUserRepository) GetByID(id uuid.UUID) (*domain.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmail(email string) (*domain.User, error) {
	args := m.Called(email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmailAndOrg(email string, orgID uuid.UUID) (*domain.User, error) {
	args := m.Called(email, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.User, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.User), args.Error(1)
}

func (m *MockUserRepository) Update(user *domain.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockUserRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockUserRepository) SetPasswordResetToken(email string, token string, expiry time.Time) error {
	args := m.Called(email, token, expiry)
	return args.Error(0)
}

func (m *MockUserRepository) GetByPasswordResetToken(token string) (*domain.User, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) ClearPasswordResetToken(userID uuid.UUID) error {
	args := m.Called(userID)
	return args.Error(0)
}

// MockTrustScoreRepository is a mock implementation of TrustScoreRepository
type MockTrustScoreRepository struct {
	mock.Mock
}

func (m *MockTrustScoreRepository) Create(score *domain.TrustScore) error {
	args := m.Called(score)
	return args.Error(0)
}

func (m *MockTrustScoreRepository) GetByAgentID(agentID uuid.UUID) (*domain.TrustScore, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TrustScore), args.Error(1)
}

func (m *MockTrustScoreRepository) Update(score *domain.TrustScore) error {
	args := m.Called(score)
	return args.Error(0)
}

func (m *MockTrustScoreRepository) GetHistoryByAgentID(agentID uuid.UUID, limit int) ([]*domain.TrustScore, error) {
	args := m.Called(agentID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TrustScore), args.Error(1)
}

// MockCapabilityRepository is a mock implementation of CapabilityRepository
type MockCapabilityRepository struct {
	mock.Mock
}

func (m *MockCapabilityRepository) Create(cap *domain.AgentCapability) error {
	args := m.Called(cap)
	return args.Error(0)
}

func (m *MockCapabilityRepository) GetByAgentID(agentID uuid.UUID) ([]*domain.AgentCapability, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AgentCapability), args.Error(1)
}

func (m *MockCapabilityRepository) GetByAgentIDAndType(agentID uuid.UUID, capType string) (*domain.AgentCapability, error) {
	args := m.Called(agentID, capType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AgentCapability), args.Error(1)
}

func (m *MockCapabilityRepository) Update(cap *domain.AgentCapability) error {
	args := m.Called(cap)
	return args.Error(0)
}

func (m *MockCapabilityRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockCapabilityRepository) DeleteByAgentID(agentID uuid.UUID) error {
	args := m.Called(agentID)
	return args.Error(0)
}

func (m *MockCapabilityRepository) GetByID(id uuid.UUID) (*domain.AgentCapability, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AgentCapability), args.Error(1)
}

// MockTagRepository is a mock implementation of TagRepository
type MockTagRepository struct {
	mock.Mock
}

func (m *MockTagRepository) Create(tag *domain.Tag) error {
	args := m.Called(tag)
	return args.Error(0)
}

func (m *MockTagRepository) GetByID(id uuid.UUID) (*domain.Tag, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tag), args.Error(1)
}

func (m *MockTagRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.Tag, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Tag), args.Error(1)
}

func (m *MockTagRepository) GetByKeyAndValue(orgID uuid.UUID, key, value string) (*domain.Tag, error) {
	args := m.Called(orgID, key, value)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tag), args.Error(1)
}

func (m *MockTagRepository) Update(tag *domain.Tag) error {
	args := m.Called(tag)
	return args.Error(0)
}

func (m *MockTagRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockTagRepository) AddAgentTag(agentID, tagID uuid.UUID) error {
	args := m.Called(agentID, tagID)
	return args.Error(0)
}

func (m *MockTagRepository) RemoveAgentTag(agentID, tagID uuid.UUID) error {
	args := m.Called(agentID, tagID)
	return args.Error(0)
}

func (m *MockTagRepository) GetAgentTags(agentID uuid.UUID) ([]*domain.Tag, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Tag), args.Error(1)
}

func (m *MockTagRepository) GetAgentsByTag(tagID uuid.UUID) ([]uuid.UUID, error) {
	args := m.Called(tagID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]uuid.UUID), args.Error(1)
}

// MockAuditLogRepository is a mock implementation of AuditLogRepository
type MockAuditLogRepository struct {
	mock.Mock
}

func (m *MockAuditLogRepository) Create(log *domain.AuditLog) error {
	args := m.Called(log)
	return args.Error(0)
}

func (m *MockAuditLogRepository) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	args := m.Called(orgID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogRepository) GetByAgent(orgID, agentID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	args := m.Called(orgID, agentID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogRepository) GetByEventType(orgID uuid.UUID, eventType string, limit, offset int) ([]*domain.AuditLog, error) {
	args := m.Called(orgID, eventType, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogRepository) CountByOrganization(orgID uuid.UUID) (int, error) {
	args := m.Called(orgID)
	return args.Int(0), args.Error(1)
}

// MockAPIKeyRepository is a mock implementation of APIKeyRepository
type MockAPIKeyRepository struct {
	mock.Mock
}

func (m *MockAPIKeyRepository) Create(key *domain.APIKey) error {
	args := m.Called(key)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) GetByID(id uuid.UUID) (*domain.APIKey, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) GetByKeyHash(keyHash string) (*domain.APIKey, error) {
	args := m.Called(keyHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) GetByAgentID(agentID uuid.UUID) ([]*domain.APIKey, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) Update(key *domain.APIKey) error {
	args := m.Called(key)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.APIKey, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.APIKey), args.Error(1)
}

// MockMCPServerRepository is a mock implementation of MCPServerRepository
type MockMCPServerRepository struct {
	mock.Mock
}

func (m *MockMCPServerRepository) Create(server *domain.MCPServer) error {
	args := m.Called(server)
	return args.Error(0)
}

func (m *MockMCPServerRepository) GetByID(id uuid.UUID) (*domain.MCPServer, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MCPServer), args.Error(1)
}

func (m *MockMCPServerRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.MCPServer, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.MCPServer), args.Error(1)
}

func (m *MockMCPServerRepository) GetByURL(url string, orgID uuid.UUID) (*domain.MCPServer, error) {
	args := m.Called(url, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MCPServer), args.Error(1)
}

func (m *MockMCPServerRepository) Update(server *domain.MCPServer) error {
	args := m.Called(server)
	return args.Error(0)
}

func (m *MockMCPServerRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

// MockVerificationEventRepository is a mock implementation of VerificationEventRepository
type MockVerificationEventRepository struct {
	mock.Mock
}

func (m *MockVerificationEventRepository) Create(event *domain.VerificationEvent) error {
	args := m.Called(event)
	return args.Error(0)
}

func (m *MockVerificationEventRepository) GetByID(id uuid.UUID) (*domain.VerificationEvent, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VerificationEvent), args.Error(1)
}

func (m *MockVerificationEventRepository) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.VerificationEvent, error) {
	args := m.Called(orgID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Error(1)
}

func (m *MockVerificationEventRepository) GetByAgent(agentID uuid.UUID, limit, offset int) ([]*domain.VerificationEvent, error) {
	args := m.Called(agentID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Error(1)
}

func (m *MockVerificationEventRepository) GetByMCPServer(serverID uuid.UUID, limit, offset int) ([]*domain.VerificationEvent, error) {
	args := m.Called(serverID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Error(1)
}

func (m *MockVerificationEventRepository) GetFailuresByOrganization(orgID uuid.UUID, limit int) ([]*domain.VerificationEvent, error) {
	args := m.Called(orgID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Error(1)
}

func (m *MockVerificationEventRepository) CountByOrganization(orgID uuid.UUID) (int, error) {
	args := m.Called(orgID)
	return args.Int(0), args.Error(1)
}

func (m *MockVerificationEventRepository) CountSuccessByOrganization(orgID uuid.UUID) (int, error) {
	args := m.Called(orgID)
	return args.Int(0), args.Error(1)
}

func (m *MockVerificationEventRepository) CountFailuresByOrganization(orgID uuid.UUID) (int, error) {
	args := m.Called(orgID)
	return args.Int(0), args.Error(1)
}

func (m *MockVerificationEventRepository) CountByOrganizationSince(orgID uuid.UUID, since time.Time) (int, error) {
	args := m.Called(orgID, since)
	return args.Int(0), args.Error(1)
}

func (m *MockVerificationEventRepository) CountSuccessByOrganizationSince(orgID uuid.UUID, since time.Time) (int, error) {
	args := m.Called(orgID, since)
	return args.Int(0), args.Error(1)
}

func (m *MockVerificationEventRepository) CountFailuresByOrganizationSince(orgID uuid.UUID, since time.Time) (int, error) {
	args := m.Called(orgID, since)
	return args.Int(0), args.Error(1)
}

// MockSDKTokenRepository is a mock implementation of SDKTokenRepository
type MockSDKTokenRepository struct {
	mock.Mock
}

func (m *MockSDKTokenRepository) Create(token *domain.SDKToken) error {
	args := m.Called(token)
	return args.Error(0)
}

func (m *MockSDKTokenRepository) GetByID(id uuid.UUID) (*domain.SDKToken, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SDKToken), args.Error(1)
}

func (m *MockSDKTokenRepository) GetByTokenHash(tokenHash string) (*domain.SDKToken, error) {
	args := m.Called(tokenHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SDKToken), args.Error(1)
}

func (m *MockSDKTokenRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.SDKToken, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SDKToken), args.Error(1)
}

func (m *MockSDKTokenRepository) Update(token *domain.SDKToken) error {
	args := m.Called(token)
	return args.Error(0)
}

func (m *MockSDKTokenRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockSDKTokenRepository) GetByRecoveryCodeHash(recoveryCodeHash string) (*domain.SDKToken, error) {
	args := m.Called(recoveryCodeHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SDKToken), args.Error(1)
}

// GetByOrganizationPaged pages over GetByOrganization for tests.
func (m *MockAgentRepository) GetByOrganizationPaged(orgID uuid.UUID, limit, offset int) ([]*domain.Agent, int, error) {
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
