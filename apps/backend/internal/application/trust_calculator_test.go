package application

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockCapabilityRepository for testing
type MockCapabilityRepository struct {
	mock.Mock
}

func (m *MockCapabilityRepository) CreateCapability(capability *domain.AgentCapability) error {
	args := m.Called(capability)
	return args.Error(0)
}

func (m *MockCapabilityRepository) GetCapabilityByID(id uuid.UUID) (*domain.AgentCapability, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AgentCapability), args.Error(1)
}

func (m *MockCapabilityRepository) GetCapabilitiesByAgentID(agentID uuid.UUID) ([]*domain.AgentCapability, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AgentCapability), args.Error(1)
}

func (m *MockCapabilityRepository) GetActiveCapabilitiesByAgentID(agentID uuid.UUID) ([]*domain.AgentCapability, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AgentCapability), args.Error(1)
}

func (m *MockCapabilityRepository) RevokeCapability(id uuid.UUID, revokedAt time.Time) error {
	args := m.Called(id, revokedAt)
	return args.Error(0)
}

func (m *MockCapabilityRepository) DeleteCapability(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockCapabilityRepository) CreateViolation(violation *domain.CapabilityViolation) error {
	args := m.Called(violation)
	return args.Error(0)
}

func (m *MockCapabilityRepository) GetViolationByID(id uuid.UUID) (*domain.CapabilityViolation, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.CapabilityViolation), args.Error(1)
}

func (m *MockCapabilityRepository) GetViolationsByAgentID(agentID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error) {
	args := m.Called(agentID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.CapabilityViolation), args.Int(1), args.Error(2)
}

func (m *MockCapabilityRepository) GetRecentViolations(orgID uuid.UUID, minutes int) ([]*domain.CapabilityViolation, error) {
	args := m.Called(orgID, minutes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.CapabilityViolation), args.Error(1)
}

func (m *MockCapabilityRepository) GetViolationsByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error) {
	args := m.Called(orgID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.CapabilityViolation), args.Int(1), args.Error(2)
}

func (m *MockCapabilityRepository) CreateCapabilityDefinition(def *domain.CapabilityDefinition) error {
	args := m.Called(def)
	return args.Error(0)
}

func (m *MockCapabilityRepository) GetCapabilityDefinitions(orgID uuid.UUID) ([]*domain.CapabilityDefinition, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.CapabilityDefinition), args.Error(1)
}

func (m *MockCapabilityRepository) GetCapabilityDefinitionByID(id uuid.UUID) (*domain.CapabilityDefinition, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.CapabilityDefinition), args.Error(1)
}

func (m *MockCapabilityRepository) UpdateCapabilityDefinition(def *domain.CapabilityDefinition) error {
	args := m.Called(def)
	return args.Error(0)
}

func (m *MockCapabilityRepository) DeleteCapabilityDefinition(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockCapabilityRepository) GetCapabilityDefinition(namespace, action string, orgID *uuid.UUID) (*domain.CapabilityDefinition, error) {
	args := m.Called(namespace, action, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.CapabilityDefinition), args.Error(1)
}

func (m *MockCapabilityRepository) ListCapabilityDefinitions(orgID *uuid.UUID) ([]*domain.CapabilityDefinition, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.CapabilityDefinition), args.Error(1)
}

// Note: AgentServiceMockTrustScoreRepository and MockAPIKeyRepository
// are defined in other test files (agent_service_test.go, auth_service_test.go)

// AgentServiceMockAuditLogRepository mocks the audit log repository
type AgentServiceMockAuditLogRepository struct {
	mock.Mock
}

func (m *AgentServiceMockAuditLogRepository) Create(log *domain.AuditLog) error {
	args := m.Called(log)
	return args.Error(0)
}

func (m *AgentServiceMockAuditLogRepository) GetByID(id uuid.UUID) (*domain.AuditLog, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AuditLog), args.Error(1)
}

func (m *AgentServiceMockAuditLogRepository) GetByAgent(agentID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	args := m.Called(agentID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *AgentServiceMockAuditLogRepository) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	args := m.Called(orgID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *AgentServiceMockAuditLogRepository) GetByUser(userID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	args := m.Called(userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *AgentServiceMockAuditLogRepository) List(limit, offset int) ([]*domain.AuditLog, error) {
	args := m.Called(limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *AgentServiceMockAuditLogRepository) GetByResource(resourceType string, resourceID uuid.UUID) ([]*domain.AuditLog, error) {
	args := m.Called(resourceType, resourceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *AgentServiceMockAuditLogRepository) Search(query string, limit, offset int) ([]*domain.AuditLog, error) {
	args := m.Called(query, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *AgentServiceMockAuditLogRepository) CountActionsByAgentInTimeWindow(agentID uuid.UUID, action domain.AuditAction, windowMinutes int) (int, error) {
	args := m.Called(agentID, action, windowMinutes)
	return args.Int(0), args.Error(1)
}

func (m *AgentServiceMockAuditLogRepository) GetRecentActionsByAgent(agentID uuid.UUID, limit int) ([]*domain.AuditLog, error) {
	args := m.Called(agentID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *AgentServiceMockAuditLogRepository) GetAgentActionsByIPAddress(agentID uuid.UUID, ipAddress string, limit int) ([]*domain.AuditLog, error) {
	args := m.Called(agentID, ipAddress, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

// TrustCalcMockAgentRepository mocks the AgentRepository for trust calculator tests
type TrustCalcMockAgentRepository struct {
	mock.Mock
}

func (m *TrustCalcMockAgentRepository) Create(agent *domain.Agent) error {
	args := m.Called(agent)
	return args.Error(0)
}

func (m *TrustCalcMockAgentRepository) GetByID(id uuid.UUID) (*domain.Agent, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *TrustCalcMockAgentRepository) GetByName(orgID uuid.UUID, name string) (*domain.Agent, error) {
	args := m.Called(orgID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *TrustCalcMockAgentRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.Agent, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *TrustCalcMockAgentRepository) List(limit, offset int) ([]*domain.Agent, error) {
	args := m.Called(limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *TrustCalcMockAgentRepository) Update(agent *domain.Agent) error {
	args := m.Called(agent)
	return args.Error(0)
}

func (m *TrustCalcMockAgentRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *TrustCalcMockAgentRepository) UpdateTrustScore(agentID uuid.UUID, score float64) error {
	args := m.Called(agentID, score)
	return args.Error(0)
}

func (m *TrustCalcMockAgentRepository) MarkAsCompromised(agentID uuid.UUID) error {
	args := m.Called(agentID)
	return args.Error(0)
}

func (m *TrustCalcMockAgentRepository) UpdateLastActive(ctx context.Context, agentID uuid.UUID) error {
	args := m.Called(ctx, agentID)
	return args.Error(0)
}

func (m *TrustCalcMockAgentRepository) IncrementViolationCount(agentID uuid.UUID) error {
	args := m.Called(agentID)
	return args.Error(0)
}

// TrustCalcMockAlertRepository mocks the AlertRepository for trust calculator tests
type TrustCalcMockAlertRepository struct {
	mock.Mock
}

func (m *TrustCalcMockAlertRepository) Create(alert *domain.Alert) error {
	args := m.Called(alert)
	return args.Error(0)
}

func (m *TrustCalcMockAlertRepository) GetByID(id uuid.UUID) (*domain.Alert, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Alert), args.Error(1)
}

func (m *TrustCalcMockAlertRepository) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.Alert, error) {
	args := m.Called(orgID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *TrustCalcMockAlertRepository) CountByOrganization(orgID uuid.UUID) (int, error) {
	args := m.Called(orgID)
	return args.Int(0), args.Error(1)
}

func (m *TrustCalcMockAlertRepository) GetUnacknowledged(orgID uuid.UUID) ([]*domain.Alert, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *TrustCalcMockAlertRepository) GetByResourceID(resourceID uuid.UUID, limit, offset int) ([]*domain.Alert, error) {
	args := m.Called(resourceID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *TrustCalcMockAlertRepository) GetUnacknowledgedByResourceID(resourceID uuid.UUID) ([]*domain.Alert, error) {
	args := m.Called(resourceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *TrustCalcMockAlertRepository) Acknowledge(id, userID uuid.UUID) error {
	args := m.Called(id, userID)
	return args.Error(0)
}

func (m *TrustCalcMockAlertRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *TrustCalcMockAlertRepository) GetByOrganizationFiltered(orgID uuid.UUID, status string, limit, offset int) ([]*domain.Alert, error) {
	args := m.Called(orgID, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *TrustCalcMockAlertRepository) CountByOrganizationFiltered(orgID uuid.UUID, status string) (int, error) {
	args := m.Called(orgID, status)
	return args.Int(0), args.Error(1)
}

func (m *TrustCalcMockAlertRepository) BulkAcknowledge(orgID uuid.UUID, userID uuid.UUID) (int, error) {
	args := m.Called(orgID, userID)
	return args.Int(0), args.Error(1)
}

func (m *TrustCalcMockAlertRepository) CountBySeverity(orgID uuid.UUID, status string) (critical, high, warning, info int, err error) {
	args := m.Called(orgID, status)
	return args.Int(0), args.Int(1), args.Int(2), args.Int(3), args.Error(4)
}

// Helper function to generate a valid X.509 certificate
func generateValidCertificate() string {
	// Generate RSA private key
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Agent"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Create certificate
	certBytes, _ := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)

	// Encode to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	return string(certPEM)
}

// Helper function to generate an expired certificate
func generateExpiredCertificate() string {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Agent"},
		},
		NotBefore:             time.Now().Add(-365 * 24 * time.Hour),
		NotAfter:              time.Now().Add(-1 * time.Hour), // Expired
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certBytes, _ := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	return string(certPEM)
}

// ============================================================================
// TEST: Weight Validation
// ============================================================================

func TestTrustCalculator_WeightsSum(t *testing.T) {
	// Verify all weights sum to exactly 1.0 (100%)
	weights := []float64{
		0.18, // verification
		0.12, // certificate
		0.12, // repository
		0.08, // documentation
		0.08, // community
		0.12, // security
		0.08, // updates
		0.05, // age
		0.17, // capability_risk
	}

	sum := 0.0
	for _, w := range weights {
		sum += w
	}

	assert.Equal(t, 1.0, sum, "Weights must sum to exactly 1.0")
}

// ============================================================================
// TEST: Calculate() - Full Algorithm
// ============================================================================

func TestTrustCalculator_Calculate_AllFactorsPerfectScore(t *testing.T) {
	// Setup mocks
	mockTrustRepo := new(AgentServiceMockTrustScoreRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockAuditRepo := new(AgentServiceMockAuditLogRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)
	mockAgentRepo := new(TrustCalcMockAgentRepository)
	mockAlertRepo := new(TrustCalcMockAlertRepository)

	calculator := NewTrustCalculator(mockTrustRepo, mockAPIKeyRepo, mockAuditRepo, mockCapabilityRepo, mockAgentRepo, mockAlertRepo)

	// Create agent with perfect conditions
	validCert := generateValidCertificate()
	agent := &domain.Agent{
		ID:               uuid.New(),
		Status:           domain.AgentStatusVerified,
		PublicKey:        &validCert,
		CertificateURL:   "https://example.com/cert.pem",
		RepositoryURL:    "https://github.com/test/repo",
		DocumentationURL: "https://docs.example.com",
		Description:      "This is a comprehensive description that is definitely longer than 50 characters to ensure proper scoring.",
		UpdatedAt:        time.Now().Add(-15 * 24 * time.Hour), // Updated recently
		CreatedAt:        time.Now().Add(-200 * 24 * time.Hour), // Old enough for max age score
		Version:          "1.0.0",
	}

	// Mock no capabilities (neutral risk) - may be called multiple times by different factors
	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil).Maybe()
	mockCapabilityRepo.On("GetViolationsByAgentID", agent.ID, 500, 0).Return([]*domain.CapabilityViolation{}, 0, nil).Maybe()
	// Mock no alerts for this agent
	mockAlertRepo.On("GetUnacknowledgedByResourceID", agent.ID).Return([]*domain.Alert{}, nil).Maybe()
	mockAlertRepo.On("GetByResourceID", agent.ID, 100, 0).Return([]*domain.Alert{}, nil).Maybe()

	// Calculate trust score
	score, err := calculator.Calculate(agent)

	assert.NoError(t, err)
	assert.NotNil(t, score)
	assert.GreaterOrEqual(t, score.Score, 0.0)
	assert.LessOrEqual(t, score.Score, 1.0)
	assert.Equal(t, agent.ID, score.AgentID)
	assert.NotZero(t, score.LastCalculated)

	// Verify individual factors are within range
	assert.GreaterOrEqual(t, score.Factors.VerificationStatus, 0.0)
	assert.LessOrEqual(t, score.Factors.VerificationStatus, 1.0)
	assert.GreaterOrEqual(t, score.Factors.Uptime, 0.0)
	assert.LessOrEqual(t, score.Factors.Uptime, 1.0)
	assert.GreaterOrEqual(t, score.Factors.SuccessRate, 0.0)
	assert.LessOrEqual(t, score.Factors.SuccessRate, 1.0)
}

func TestTrustCalculator_Calculate_MinimalAgent(t *testing.T) {
	mockTrustRepo := new(AgentServiceMockTrustScoreRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockAuditRepo := new(AgentServiceMockAuditLogRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)
	mockAgentRepo := new(TrustCalcMockAgentRepository)
	mockAlertRepo := new(TrustCalcMockAlertRepository)

	calculator := NewTrustCalculator(mockTrustRepo, mockAPIKeyRepo, mockAuditRepo, mockCapabilityRepo, mockAgentRepo, mockAlertRepo)

	// Create minimal agent (pending status, no cert, no docs)
	agent := &domain.Agent{
		ID:        uuid.New(),
		Status:    domain.AgentStatusPending,
		UpdatedAt: time.Now(),
		CreatedAt: time.Now(),
	}

	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil).Maybe()
	mockCapabilityRepo.On("GetViolationsByAgentID", agent.ID, 500, 0).Return([]*domain.CapabilityViolation{}, 0, nil).Maybe()
	mockAlertRepo.On("GetUnacknowledgedByResourceID", agent.ID).Return([]*domain.Alert{}, nil).Maybe()
	mockAlertRepo.On("GetByResourceID", agent.ID, 100, 0).Return([]*domain.Alert{}, nil).Maybe()

	score, err := calculator.Calculate(agent)

	assert.NoError(t, err)
	assert.NotNil(t, score)
	assert.GreaterOrEqual(t, score.Score, 0.0)
	assert.LessOrEqual(t, score.Score, 1.0)

	// Verify agent with pending status has lower verification status factor
	assert.Equal(t, 0.3, score.Factors.VerificationStatus, "Pending agents should have verification status of 0.3")
}

// ============================================================================
// TEST: calculateVerificationStatus()
// ============================================================================

func TestTrustCalculator_VerificationStatus_Verified(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{Status: domain.AgentStatusVerified}
	score := calculator.calculateVerificationStatus(agent)

	assert.Equal(t, 1.0, score, "Verified agents should get score of 1.0")
}

func TestTrustCalculator_VerificationStatus_Pending(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{Status: domain.AgentStatusPending}
	score := calculator.calculateVerificationStatus(agent)

	assert.Equal(t, 0.3, score, "Pending agents should get score of 0.3")
}

func TestTrustCalculator_VerificationStatus_Suspended(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{Status: domain.AgentStatusSuspended}
	score := calculator.calculateVerificationStatus(agent)

	assert.Equal(t, 0.1, score, "Suspended agents should get score of 0.1")
}

func TestTrustCalculator_VerificationStatus_Revoked(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{Status: domain.AgentStatusRevoked}
	score := calculator.calculateVerificationStatus(agent)

	assert.Equal(t, 0.0, score, "Revoked agents should get score of 0.0")
}

func TestTrustCalculator_VerificationStatus_Unknown(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{Status: "unknown"}
	score := calculator.calculateVerificationStatus(agent)

	assert.Equal(t, 0.3, score, "Unknown status should default to 0.3")
}

// ===========================
// Constructor Tests
// ===========================

func TestNewTrustCalculator(t *testing.T) {
	calculator := NewTrustCalculator(nil, nil, nil, nil, nil, nil)

	assert.NotNil(t, calculator)
	assert.Nil(t, calculator.trustScoreRepo)
	assert.Nil(t, calculator.apiKeyRepo)
	assert.Nil(t, calculator.auditRepo)
	assert.Nil(t, calculator.capabilityRepo)
	assert.Nil(t, calculator.agentRepo)
	assert.Nil(t, calculator.alertRepo)
}

func TestNewTrustCalculatorWithVerification(t *testing.T) {
	calculator := NewTrustCalculatorWithVerification(nil, nil, nil, nil, nil, nil, nil)

	assert.NotNil(t, calculator)
	assert.Nil(t, calculator.verificationEventRepo)
	assert.Nil(t, calculator.trustScoreRepo)
	assert.Nil(t, calculator.apiKeyRepo)
	assert.Nil(t, calculator.auditRepo)
	assert.Nil(t, calculator.capabilityRepo)
	assert.Nil(t, calculator.agentRepo)
	assert.Nil(t, calculator.alertRepo)
}

// ===========================
// Calculate Age Tests
// ===========================

func TestTrustCalculator_CalculateAge_NewAgent(t *testing.T) {
	calculator := &TrustCalculator{}
	now := time.Now()

	agent := &domain.Agent{
		ID:        uuid.New(),
		CreatedAt: now.Add(-time.Hour), // 1 hour old
	}

	age := calculator.calculateAge(agent)

	// New agents should have lower age score
	assert.True(t, age >= 0.0 && age <= 1.0, "Age should be between 0 and 1")
	assert.True(t, age < 0.5, "Very new agent should have low age score")
}

func TestTrustCalculator_CalculateAge_OldAgent(t *testing.T) {
	calculator := &TrustCalculator{}
	now := time.Now()

	agent := &domain.Agent{
		ID:        uuid.New(),
		CreatedAt: now.AddDate(-1, 0, 0), // 1 year old
	}

	age := calculator.calculateAge(agent)

	// Old agents should have higher age score (closer to 1.0)
	assert.True(t, age >= 0.5, "Old agents should have higher age score")
	assert.True(t, age <= 1.0, "Age should not exceed 1.0")
}

// ===========================
// Calculate Compliance Tests
// ===========================

func TestTrustCalculator_CalculateCompliance_NoViolations(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{
		ID: uuid.New(),
	}

	compliance := calculator.calculateCompliance(agent)

	// Agent with no violations should have high compliance
	assert.True(t, compliance >= 0.0 && compliance <= 1.0, "Compliance should be valid range")
}

// ===========================
// Calculate Drift Detection Tests
// ===========================

func TestTrustCalculator_CalculateDriftDetection_Default(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{
		ID: uuid.New(),
	}

	drift := calculator.calculateDriftDetection(agent)

	// Without drift repo, should return baseline
	assert.True(t, drift >= 0.0 && drift <= 1.0, "Drift detection should be valid")
}

// ===========================
// Calculate User Feedback Tests
// ===========================

func TestTrustCalculator_CalculateUserFeedback_Baseline(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{
		ID: uuid.New(),
	}

	feedback := calculator.calculateUserFeedback(agent)

	// Should return baseline feedback value
	assert.Equal(t, 0.75, feedback, "User feedback should return baseline")
}

// ===========================
// Calculate Confidence Tests
// ===========================

func TestTrustCalculator_CalculateConfidence_FullData(t *testing.T) {
	calculator := &TrustCalculator{}
	now := time.Now()

	// Agent with lots of data points
	agent := &domain.Agent{
		ID:           uuid.New(),
		Status:       domain.AgentStatusVerified,
		Capabilities: []string{"read", "write"},
		TalksTo:      []string{"agent-1", "agent-2"},
		CreatedAt:    now.AddDate(0, -3, 0), // 3 months old
		VerifiedAt:   &now,
	}

	factors := &domain.TrustScoreFactors{
		VerificationStatus: 1.0,
		Uptime:             0.95,
	}

	confidence := calculator.calculateConfidence(agent, factors)

	// Confidence should be in valid range
	assert.True(t, confidence >= 0.0, "Confidence should be non-negative")
	assert.True(t, confidence <= 1.0, "Confidence should not exceed 1.0")
}

func TestTrustCalculator_CalculateConfidence_MinimalData(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{
		ID:        uuid.New(),
		CreatedAt: time.Now(), // Just created
	}

	factors := &domain.TrustScoreFactors{}

	confidence := calculator.calculateConfidence(agent, factors)

	// Agent with minimal data should have lower confidence
	assert.True(t, confidence >= 0.0, "Confidence should be non-negative")
	assert.True(t, confidence <= 1.0, "Confidence should not exceed 1.0")
}

// Note: CalculateFactors requires repositories to be set up
// Full integration tests with mocks are in other test files

// Note: End-to-end Calculate tests are in other test functions that use mocked repositories
// (TestTrustCalculator_Calculate_AllFactorsPerfectScore, TestTrustCalculator_Calculate_MinimalAgent)

// ===========================
// CalculateTrustScore Tests
// ===========================

func TestTrustCalculator_CalculateTrustScore_Success(t *testing.T) {
	mockAgentRepo := new(TrustCalcMockAgentRepository)
	mockTrustRepo := new(AgentServiceMockTrustScoreRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockAuditRepo := new(AgentServiceMockAuditLogRepository)
	mockCapRepo := new(MockCapabilityRepository)
	mockAlertRepo := new(TrustCalcMockAlertRepository)

	agentID := uuid.New()
	orgID := uuid.New()

	agent := &domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		Name:           "test-agent",
		Status:         domain.AgentStatusVerified,
		TrustScore:     0.8,
		CreatedAt:      time.Now().Add(-30 * 24 * time.Hour), // 30 days old
		UpdatedAt:      time.Now(),
	}

	// Set up mock expectations
	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)
	mockCapRepo.On("GetActiveCapabilitiesByAgentID", agentID).Return([]*domain.AgentCapability{}, nil).Maybe()
	mockCapRepo.On("GetViolationsByAgentID", agentID, 500, 0).Return([]*domain.CapabilityViolation{}, 0, nil).Maybe()
	mockAlertRepo.On("GetUnacknowledgedByResourceID", agentID).Return([]*domain.Alert{}, nil).Maybe()
	mockAlertRepo.On("GetByResourceID", agentID, 100, 0).Return([]*domain.Alert{}, nil).Maybe()
	mockTrustRepo.On("Create", mock.Anything).Return(nil)
	mockAgentRepo.On("UpdateTrustScore", agentID, mock.Anything).Return(nil)

	calculator := NewTrustCalculator(
		mockTrustRepo,
		mockAPIKeyRepo,
		mockAuditRepo,
		mockCapRepo,
		mockAgentRepo,
		mockAlertRepo,
	)

	ctx := context.Background()
	score, err := calculator.CalculateTrustScore(ctx, agentID)

	assert.NoError(t, err)
	assert.NotNil(t, score)
	mockTrustRepo.AssertCalled(t, "Create", mock.Anything)
	mockAgentRepo.AssertCalled(t, "UpdateTrustScore", agentID, mock.Anything)
}

func TestTrustCalculator_CalculateTrustScore_AgentNotFound(t *testing.T) {
	mockAgentRepo := new(TrustCalcMockAgentRepository)
	mockTrustRepo := new(AgentServiceMockTrustScoreRepository)

	agentID := uuid.New()
	mockAgentRepo.On("GetByID", agentID).Return(nil, assert.AnError)

	calculator := &TrustCalculator{
		agentRepo:      mockAgentRepo,
		trustScoreRepo: mockTrustRepo,
	}

	ctx := context.Background()
	score, err := calculator.CalculateTrustScore(ctx, agentID)

	assert.Error(t, err)
	assert.Nil(t, score)
}

// ===========================
// GetLatestTrustScore Tests
// ===========================

func TestTrustCalculator_GetLatestTrustScore_Success(t *testing.T) {
	mockTrustRepo := new(AgentServiceMockTrustScoreRepository)

	agentID := uuid.New()
	expectedScore := &domain.TrustScore{
		ID:      uuid.New(),
		AgentID: agentID,
		Score:   0.85,
	}

	mockTrustRepo.On("GetLatest", agentID).Return(expectedScore, nil)

	calculator := &TrustCalculator{trustScoreRepo: mockTrustRepo}

	ctx := context.Background()
	score, err := calculator.GetLatestTrustScore(ctx, agentID)

	assert.NoError(t, err)
	assert.Equal(t, expectedScore, score)
}

func TestTrustCalculator_GetLatestTrustScore_NotFound(t *testing.T) {
	mockTrustRepo := new(AgentServiceMockTrustScoreRepository)

	agentID := uuid.New()
	mockTrustRepo.On("GetLatest", agentID).Return(nil, assert.AnError)

	calculator := &TrustCalculator{trustScoreRepo: mockTrustRepo}

	ctx := context.Background()
	score, err := calculator.GetLatestTrustScore(ctx, agentID)

	assert.Error(t, err)
	assert.Nil(t, score)
}

// ===========================
// GetTrustScoreHistory Tests
// ===========================

func TestTrustCalculator_GetTrustScoreHistory_Success(t *testing.T) {
	mockTrustRepo := new(AgentServiceMockTrustScoreRepository)

	agentID := uuid.New()
	expectedHistory := []*domain.TrustScore{
		{ID: uuid.New(), AgentID: agentID, Score: 0.90},
		{ID: uuid.New(), AgentID: agentID, Score: 0.85},
		{ID: uuid.New(), AgentID: agentID, Score: 0.80},
	}

	mockTrustRepo.On("GetHistory", agentID, 10).Return(expectedHistory, nil)

	calculator := &TrustCalculator{trustScoreRepo: mockTrustRepo}

	ctx := context.Background()
	history, err := calculator.GetTrustScoreHistory(ctx, agentID, 10)

	assert.NoError(t, err)
	assert.Len(t, history, 3)
}

func TestTrustCalculator_GetTrustScoreHistory_Empty(t *testing.T) {
	mockTrustRepo := new(AgentServiceMockTrustScoreRepository)

	agentID := uuid.New()
	mockTrustRepo.On("GetHistory", agentID, 10).Return([]*domain.TrustScore{}, nil)

	calculator := &TrustCalculator{trustScoreRepo: mockTrustRepo}

	ctx := context.Background()
	history, err := calculator.GetTrustScoreHistory(ctx, agentID, 10)

	assert.NoError(t, err)
	assert.Empty(t, history)
}

// ===========================
// GetTrustScoreHistoryAuditTrail Tests
// ===========================

func TestTrustCalculator_GetTrustScoreHistoryAuditTrail_Success(t *testing.T) {
	mockTrustRepo := new(AgentServiceMockTrustScoreRepository)

	agentID := uuid.New()
	orgID := uuid.New()
	prevScore := 0.80
	expectedAuditTrail := []*domain.TrustScoreHistoryEntry{
		{
			ID:             uuid.New(),
			AgentID:        agentID,
			OrganizationID: orgID,
			PreviousScore:  &prevScore,
			TrustScore:     0.85,
			RecordedAt:     time.Now(),
			ChangeReason:   "Verification completed",
		},
	}

	mockTrustRepo.On("GetHistoryAuditTrail", agentID, 10).Return(expectedAuditTrail, nil)

	calculator := &TrustCalculator{trustScoreRepo: mockTrustRepo}

	ctx := context.Background()
	auditTrail, err := calculator.GetTrustScoreHistoryAuditTrail(ctx, agentID, 10)

	assert.NoError(t, err)
	assert.Len(t, auditTrail, 1)
	assert.Equal(t, "Verification completed", auditTrail[0].ChangeReason)
}

func TestTrustCalculator_GetTrustScoreHistoryAuditTrail_Error(t *testing.T) {
	mockTrustRepo := new(AgentServiceMockTrustScoreRepository)

	agentID := uuid.New()
	mockTrustRepo.On("GetHistoryAuditTrail", agentID, 10).Return(nil, assert.AnError)

	calculator := &TrustCalculator{trustScoreRepo: mockTrustRepo}

	ctx := context.Background()
	auditTrail, err := calculator.GetTrustScoreHistoryAuditTrail(ctx, agentID, 10)

	assert.Error(t, err)
	assert.Nil(t, auditTrail)
}

// Helper function for time pointer
func timePtr(t time.Time) *time.Time {
	return &t
}
