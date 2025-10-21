package application

import (
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

	calculator := NewTrustCalculator(mockTrustRepo, mockAPIKeyRepo, mockAuditRepo, mockCapabilityRepo)

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

	// Mock no capabilities (neutral risk)
	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil)
	mockCapabilityRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return([]*domain.CapabilityViolation{}, 0, nil)

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
	assert.GreaterOrEqual(t, score.Factors.CertificateValidity, 0.0)
	assert.LessOrEqual(t, score.Factors.CertificateValidity, 1.0)
	assert.GreaterOrEqual(t, score.Factors.CapabilityRisk, 0.0)
	assert.LessOrEqual(t, score.Factors.CapabilityRisk, 1.0)

	mockCapabilityRepo.AssertExpectations(t)
}

func TestTrustCalculator_Calculate_MinimalAgent(t *testing.T) {
	mockTrustRepo := new(AgentServiceMockTrustScoreRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockAuditRepo := new(AgentServiceMockAuditLogRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)

	calculator := NewTrustCalculator(mockTrustRepo, mockAPIKeyRepo, mockAuditRepo, mockCapabilityRepo)

	// Create minimal agent (pending status, no cert, no docs)
	agent := &domain.Agent{
		ID:        uuid.New(),
		Status:    domain.AgentStatusPending,
		UpdatedAt: time.Now(),
		CreatedAt: time.Now(),
	}

	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil)
	mockCapabilityRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return([]*domain.CapabilityViolation{}, 0, nil)

	score, err := calculator.Calculate(agent)

	assert.NoError(t, err)
	assert.NotNil(t, score)
	assert.GreaterOrEqual(t, score.Score, 0.0)
	assert.LessOrEqual(t, score.Score, 1.0)

	// Minimal agent should have low score
	assert.Less(t, score.Score, 0.5, "Minimal agent should have score < 0.5")

	mockCapabilityRepo.AssertExpectations(t)
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

// ============================================================================
// TEST: calculateCertificateValidity()
// ============================================================================

func TestTrustCalculator_CertificateValidity_ValidCertificate(t *testing.T) {
	calculator := &TrustCalculator{}

	validCert := generateValidCertificate()
	agent := &domain.Agent{
		CertificateURL: "https://example.com/cert.pem",
		PublicKey:      &validCert,
	}

	score := calculator.calculateCertificateValidity(agent)

	assert.Equal(t, 1.0, score, "Valid certificate should get score of 1.0")
}

func TestTrustCalculator_CertificateValidity_ExpiredCertificate(t *testing.T) {
	calculator := &TrustCalculator{}

	expiredCert := generateExpiredCertificate()
	agent := &domain.Agent{
		CertificateURL: "https://example.com/cert.pem",
		PublicKey:      &expiredCert,
	}

	score := calculator.calculateCertificateValidity(agent)

	assert.Equal(t, 0.2, score, "Expired certificate should get score of 0.2")
}

func TestTrustCalculator_CertificateValidity_NoCertificateURL(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{
		CertificateURL: "",
	}

	score := calculator.calculateCertificateValidity(agent)

	assert.Equal(t, 0.0, score, "No certificate URL should get score of 0.0")
}

func TestTrustCalculator_CertificateValidity_NoPublicKey(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{
		CertificateURL: "https://example.com/cert.pem",
		PublicKey:      nil,
	}

	score := calculator.calculateCertificateValidity(agent)

	assert.Equal(t, 0.3, score, "No public key should get score of 0.3")
}

func TestTrustCalculator_CertificateValidity_InvalidPEM(t *testing.T) {
	calculator := &TrustCalculator{}

	invalidPEM := "not a valid PEM"
	agent := &domain.Agent{
		CertificateURL: "https://example.com/cert.pem",
		PublicKey:      &invalidPEM,
	}

	score := calculator.calculateCertificateValidity(agent)

	assert.Equal(t, 0.3, score, "Invalid PEM should get score of 0.3")
}

// ============================================================================
// TEST: calculateRepositoryQuality()
// ============================================================================

func TestTrustCalculator_RepositoryQuality_NoRepositoryURL(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{RepositoryURL: ""}
	score := calculator.calculateRepositoryQuality(agent)

	assert.Equal(t, 0.0, score, "No repository URL should get score of 0.0")
}

func TestTrustCalculator_RepositoryQuality_InvalidURL(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{RepositoryURL: "not a valid url"}
	score := calculator.calculateRepositoryQuality(agent)

	assert.Equal(t, 0.0, score, "Invalid URL should get score of 0.0")
}

func TestTrustCalculator_RepositoryQuality_GitHubURL(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{RepositoryURL: "https://github.com/test/repo"}
	score := calculator.calculateRepositoryQuality(agent)

	// GitHub URL gets 0.5 for known hosting service
	assert.GreaterOrEqual(t, score, 0.5, "GitHub URL should get at least 0.5")
	assert.LessOrEqual(t, score, 1.0)
}

func TestTrustCalculator_RepositoryQuality_GitLabURL(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{RepositoryURL: "https://gitlab.com/test/repo"}
	score := calculator.calculateRepositoryQuality(agent)

	assert.GreaterOrEqual(t, score, 0.5, "GitLab URL should get at least 0.5")
	assert.LessOrEqual(t, score, 1.0)
}

func TestTrustCalculator_RepositoryQuality_BitbucketURL(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{RepositoryURL: "https://bitbucket.org/test/repo"}
	score := calculator.calculateRepositoryQuality(agent)

	assert.GreaterOrEqual(t, score, 0.5, "Bitbucket URL should get at least 0.5")
	assert.LessOrEqual(t, score, 1.0)
}

// ============================================================================
// TEST: calculateDocumentationScore()
// ============================================================================

func TestTrustCalculator_DocumentationScore_NoDocumentation(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{
		Description:      "",
		DocumentationURL: "",
	}

	score := calculator.calculateDocumentationScore(agent)

	assert.Equal(t, 0.0, score, "No documentation should get score of 0.0")
}

func TestTrustCalculator_DocumentationScore_ShortDescription(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{
		Description:      "Short",
		DocumentationURL: "",
	}

	score := calculator.calculateDocumentationScore(agent)

	assert.Equal(t, 0.0, score, "Short description (<50 chars) should get score of 0.0")
}

func TestTrustCalculator_DocumentationScore_LongDescription(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{
		Description:      "This is a comprehensive description that is definitely longer than 50 characters.",
		DocumentationURL: "",
	}

	score := calculator.calculateDocumentationScore(agent)

	assert.Equal(t, 0.3, score, "Long description should get score of 0.3")
}

func TestTrustCalculator_DocumentationScore_WithDocURL(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{
		Description:      "This is a comprehensive description that is definitely longer than 50 characters.",
		DocumentationURL: "https://docs.example.com",
	}

	score := calculator.calculateDocumentationScore(agent)

	// Score should be at least 0.6 (0.3 for description + 0.3 for URL)
	assert.GreaterOrEqual(t, score, 0.6, "Description + doc URL should get at least 0.6")
	assert.LessOrEqual(t, score, 1.0)
}

// ============================================================================
// TEST: calculateUpdateFrequency()
// ============================================================================

func TestTrustCalculator_UpdateFrequency_RecentUpdate(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{
		UpdatedAt: time.Now().Add(-15 * 24 * time.Hour), // 15 days ago
	}

	score := calculator.calculateUpdateFrequency(agent)

	assert.Equal(t, 1.0, score, "Update within 30 days should get score of 1.0")
}

func TestTrustCalculator_UpdateFrequency_30To90Days(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{
		UpdatedAt: time.Now().Add(-60 * 24 * time.Hour), // 60 days ago
	}

	score := calculator.calculateUpdateFrequency(agent)

	assert.Equal(t, 0.7, score, "Update within 90 days should get score of 0.7")
}

func TestTrustCalculator_UpdateFrequency_90To180Days(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{
		UpdatedAt: time.Now().Add(-120 * 24 * time.Hour), // 120 days ago
	}

	score := calculator.calculateUpdateFrequency(agent)

	assert.Equal(t, 0.5, score, "Update within 180 days should get score of 0.5")
}

func TestTrustCalculator_UpdateFrequency_180To365Days(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{
		UpdatedAt: time.Now().Add(-200 * 24 * time.Hour), // 200 days ago
	}

	score := calculator.calculateUpdateFrequency(agent)

	assert.Equal(t, 0.3, score, "Update within 365 days should get score of 0.3")
}

func TestTrustCalculator_UpdateFrequency_Over365Days(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{
		UpdatedAt: time.Now().Add(-400 * 24 * time.Hour), // 400 days ago
	}

	score := calculator.calculateUpdateFrequency(agent)

	assert.Equal(t, 0.1, score, "Update over 365 days ago should get score of 0.1")
}

// ============================================================================
// TEST: calculateAgeScore()
// ============================================================================

func TestTrustCalculator_AgeScore_LessThan7Days(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{
		CreatedAt: time.Now().Add(-5 * 24 * time.Hour), // 5 days old
	}

	score := calculator.calculateAgeScore(agent)

	assert.Equal(t, 0.2, score, "Agent less than 7 days old should get score of 0.2")
}

func TestTrustCalculator_AgeScore_7To30Days(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{
		CreatedAt: time.Now().Add(-20 * 24 * time.Hour), // 20 days old
	}

	score := calculator.calculateAgeScore(agent)

	assert.Equal(t, 0.4, score, "Agent 7-30 days old should get score of 0.4")
}

func TestTrustCalculator_AgeScore_30To90Days(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{
		CreatedAt: time.Now().Add(-60 * 24 * time.Hour), // 60 days old
	}

	score := calculator.calculateAgeScore(agent)

	assert.Equal(t, 0.6, score, "Agent 30-90 days old should get score of 0.6")
}

func TestTrustCalculator_AgeScore_90To180Days(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{
		CreatedAt: time.Now().Add(-120 * 24 * time.Hour), // 120 days old
	}

	score := calculator.calculateAgeScore(agent)

	assert.Equal(t, 0.8, score, "Agent 90-180 days old should get score of 0.8")
}

func TestTrustCalculator_AgeScore_Over180Days(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{
		CreatedAt: time.Now().Add(-200 * 24 * time.Hour), // 200 days old
	}

	score := calculator.calculateAgeScore(agent)

	assert.Equal(t, 1.0, score, "Agent over 180 days old should get score of 1.0")
}

// Test 1: Agent with no capabilities (neutral baseline)
func TestCalculateCapabilityRisk_NoCapabilities(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID:        uuid.New(),
		Name:      "test-agent",
		AgentType: domain.AgentTypeAI,
		Status:    domain.AgentStatusVerified,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Mock: No capabilities
	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil)
	// Mock: No violations
	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return([]*domain.CapabilityViolation{}, 0, nil)

	score := calculator.calculateCapabilityRisk(agent)

	assert.Equal(t, 0.7, score, "Agent with no capabilities should have neutral score of 0.7")
	mockRepo.AssertExpectations(t)
}

// Test 2: Agent with only low-risk capabilities
func TestCalculateCapabilityRisk_LowRiskOnly(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID:        uuid.New(),
		Name:      "low-risk-agent",
		AgentType: domain.AgentTypeAI,
		Status:    domain.AgentStatusVerified,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Mock: Low-risk capabilities (file:read, db:query)
	capabilities := []*domain.AgentCapability{
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilityFileRead,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilityDBQuery,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}
	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(capabilities, nil)
	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return([]*domain.CapabilityViolation{}, 0, nil)

	score := calculator.calculateCapabilityRisk(agent)

	expectedScore := 0.7 - 0.03 - 0.03 // 0.64
	assert.InDelta(t, expectedScore, score, 0.001, "Low-risk capabilities should have minor penalties")
	mockRepo.AssertExpectations(t)
}

// Test 3: Agent with high-risk capabilities
func TestCalculateCapabilityRisk_HighRiskCapabilities(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID:        uuid.New(),
		Name:      "high-risk-agent",
		AgentType: domain.AgentTypeAI,
		Status:    domain.AgentStatusVerified,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Mock: High-risk capabilities (system:admin, user:impersonate)
	capabilities := []*domain.AgentCapability{
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilitySystemAdmin,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilityUserImpersonate,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}
	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(capabilities, nil)
	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return([]*domain.CapabilityViolation{}, 0, nil)

	score := calculator.calculateCapabilityRisk(agent)

	expectedScore := 0.7 - 0.20 - 0.20 // 0.30
	assert.InDelta(t, expectedScore, score, 0.001, "High-risk capabilities should have major penalties")
	mockRepo.AssertExpectations(t)
}

// Test 4: Agent with medium-risk capabilities
func TestCalculateCapabilityRisk_MediumRiskCapabilities(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID:        uuid.New(),
		Name:      "medium-risk-agent",
		AgentType: domain.AgentTypeAI,
		Status:    domain.AgentStatusVerified,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Mock: Medium-risk capabilities (file:write, db:write, api:call)
	capabilities := []*domain.AgentCapability{
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilityFileWrite,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilityDBWrite,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilityAPICall,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}
	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(capabilities, nil)
	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return([]*domain.CapabilityViolation{}, 0, nil)

	score := calculator.calculateCapabilityRisk(agent)

	expectedScore := 0.7 - 0.08 - 0.08 - 0.05 // 0.49
	assert.InDelta(t, expectedScore, score, 0.001, "Medium-risk capabilities should have moderate penalties")
	mockRepo.AssertExpectations(t)
}

// Test 5: Agent with recent CRITICAL violations
func TestCalculateCapabilityRisk_CriticalViolations(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID:        uuid.New(),
		Name:      "violation-agent",
		AgentType: domain.AgentTypeAI,
		Status:    domain.AgentStatusVerified,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Mock: Single low-risk capability
	capabilities := []*domain.AgentCapability{
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilityFileRead,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}

	// Mock: 3 recent CRITICAL violations (last 7 days)
	now := time.Now()
	violations := []*domain.CapabilityViolation{
		{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilitySystemAdmin,
			Severity:            domain.ViolationSeverityCritical,
			TrustScoreImpact:    -15,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -1), // 1 day ago
		},
		{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilitySystemAdmin,
			Severity:            domain.ViolationSeverityCritical,
			TrustScoreImpact:    -15,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -5), // 5 days ago
		},
		{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilitySystemAdmin,
			Severity:            domain.ViolationSeverityCritical,
			TrustScoreImpact:    -15,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -7), // 7 days ago
		},
	}

	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(capabilities, nil)
	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return(violations, 3, nil)

	score := calculator.calculateCapabilityRisk(agent)

	// Expected: 0.7 - 0.03 (file:read) - (3 * 0.15) (critical violations) = 0.22
	expectedScore := 0.7 - 0.03 - (3 * 0.15)
	assert.InDelta(t, expectedScore, score, 0.001, "CRITICAL violations should heavily impact trust")
	mockRepo.AssertExpectations(t)
}

// Test 6: Agent with many violations (volume penalty)
func TestCalculateCapabilityRisk_HighViolationVolume(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID:        uuid.New(),
		Name:      "high-volume-violation-agent",
		AgentType: domain.AgentTypeAI,
		Status:    domain.AgentStatusVerified,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Mock: No capabilities
	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil)

	// Mock: 12 recent LOW violations (triggers volume penalty)
	now := time.Now()
	violations := make([]*domain.CapabilityViolation, 12)
	for i := 0; i < 12; i++ {
		violations[i] = &domain.CapabilityViolation{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilityFileWrite,
			Severity:            domain.ViolationSeverityLow,
			TrustScoreImpact:    -2,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -i-1),
		}
	}

	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return(violations, 12, nil)

	score := calculator.calculateCapabilityRisk(agent)

	// Expected: 0.7 - (12 * 0.02) - 0.20 (volume penalty) = 0.26
	expectedScore := 0.7 - (12 * 0.02) - 0.20
	assert.InDelta(t, expectedScore, score, 0.001, "High violation volume should trigger additional penalty")
	mockRepo.AssertExpectations(t)
}

// Test 7: Score bounds enforcement (cannot go below 0)
func TestCalculateCapabilityRisk_ScoreBoundsMinimum(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID:        uuid.New(),
		Name:      "extreme-risk-agent",
		AgentType: domain.AgentTypeAI,
		Status:    domain.AgentStatusVerified,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Mock: All high-risk capabilities
	capabilities := []*domain.AgentCapability{
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilitySystemAdmin,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilityUserImpersonate,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilityFileDelete,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilityDataExport,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}

	// Mock: Many CRITICAL violations
	now := time.Now()
	violations := make([]*domain.CapabilityViolation, 20)
	for i := 0; i < 20; i++ {
		violations[i] = &domain.CapabilityViolation{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilitySystemAdmin,
			Severity:            domain.ViolationSeverityCritical,
			TrustScoreImpact:    -15,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -i-1),
		}
	}

	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(capabilities, nil)
	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return(violations, 20, nil)

	score := calculator.calculateCapabilityRisk(agent)

	assert.Equal(t, 0.0, score, "Score should never go below 0")
	mockRepo.AssertExpectations(t)
}

// Test 8: Old violations should not impact score
func TestCalculateCapabilityRisk_OldViolationsIgnored(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID:        uuid.New(),
		Name:      "old-violation-agent",
		AgentType: domain.AgentTypeAI,
		Status:    domain.AgentStatusVerified,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Mock: No capabilities
	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil)

	// Mock: Violations older than 30 days
	now := time.Now()
	violations := []*domain.CapabilityViolation{
		{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilitySystemAdmin,
			Severity:            domain.ViolationSeverityCritical,
			TrustScoreImpact:    -15,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -35), // 35 days ago (outside 30-day window)
		},
		{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilitySystemAdmin,
			Severity:            domain.ViolationSeverityCritical,
			TrustScoreImpact:    -15,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -60), // 60 days ago
		},
	}

	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return(violations, 2, nil)

	score := calculator.calculateCapabilityRisk(agent)

	assert.Equal(t, 0.7, score, "Violations older than 30 days should not impact score")
	mockRepo.AssertExpectations(t)
}

// Test 9: Mixed severity violations
func TestCalculateCapabilityRisk_MixedSeverityViolations(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID:        uuid.New(),
		Name:      "mixed-violations-agent",
		AgentType: domain.AgentTypeAI,
		Status:    domain.AgentStatusVerified,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Mock: No capabilities
	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil)

	// Mock: Various severity violations
	now := time.Now()
	violations := []*domain.CapabilityViolation{
		{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilitySystemAdmin,
			Severity:            domain.ViolationSeverityCritical,
			TrustScoreImpact:    -15,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -1), // 1 day ago
		},
		{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilityFileDelete,
			Severity:            domain.ViolationSeverityHigh,
			TrustScoreImpact:    -10,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -2), // 2 days ago
		},
		{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilityFileWrite,
			Severity:            domain.ViolationSeverityMedium,
			TrustScoreImpact:    -5,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -3), // 3 days ago
		},
		{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilityFileRead,
			Severity:            domain.ViolationSeverityLow,
			TrustScoreImpact:    -2,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -4), // 4 days ago
		},
	}

	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return(violations, 4, nil)

	score := calculator.calculateCapabilityRisk(agent)

	// Expected: 0.7 - 0.15 (critical) - 0.10 (high) - 0.05 (medium) - 0.02 (low) = 0.38
	expectedScore := 0.7 - 0.15 - 0.10 - 0.05 - 0.02
	assert.InDelta(t, expectedScore, score, 0.001, "Mixed severity violations should have varying impacts")
	mockRepo.AssertExpectations(t)
}

// Test 10: Error handling - repository error returns baseline
func TestCalculateCapabilityRisk_RepositoryError(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID:        uuid.New(),
		Name:      "error-agent",
		AgentType: domain.AgentTypeAI,
		Status:    domain.AgentStatusVerified,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Mock: Repository error for capabilities
	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(nil, assert.AnError)
	// Mock: No violations when capability fetch fails
	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return([]*domain.CapabilityViolation{}, 0, nil)

	score := calculator.calculateCapabilityRisk(agent)

	assert.Equal(t, 0.7, score, "Repository error should return neutral baseline score")
	mockRepo.AssertExpectations(t)
}

// Test 11: Volume penalty thresholds
func TestCalculateCapabilityRisk_ViolationVolumeThresholds(t *testing.T) {
	now := time.Now()

	// Test 6 violations (should trigger -0.10 penalty)
	t.Run("6 violations", func(t *testing.T) {
		mockRepo := new(MockCapabilityRepository)
		calculator := &TrustCalculator{
			capabilityRepo: mockRepo,
		}

		agent := &domain.Agent{
			ID:        uuid.New(),
			Name:      "6-violations-agent",
			AgentType: domain.AgentTypeAI,
			Status:    domain.AgentStatusVerified,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil)

		violations := make([]*domain.CapabilityViolation, 6)
		for i := 0; i < 6; i++ {
			violations[i] = &domain.CapabilityViolation{
				ID:                  uuid.New(),
				AgentID:             agent.ID,
				AttemptedCapability: domain.CapabilityFileRead,
				Severity:            domain.ViolationSeverityLow,
				TrustScoreImpact:    -2,
				IsBlocked:           true,
				CreatedAt:           now.AddDate(0, 0, -i-1),
			}
		}

		mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return(violations, 6, nil)

		score := calculator.calculateCapabilityRisk(agent)

		// Expected: 0.7 - (6 * 0.02) - 0.10 (volume penalty > 5) = 0.48
		expectedScore := 0.7 - (6 * 0.02) - 0.10
		assert.InDelta(t, expectedScore, score, 0.001, "6 violations should trigger -0.10 volume penalty")
		mockRepo.AssertExpectations(t)
	})

	// Test 11 violations (should trigger -0.20 penalty)
	t.Run("11 violations", func(t *testing.T) {
		mockRepo := new(MockCapabilityRepository)
		calculator := &TrustCalculator{
			capabilityRepo: mockRepo,
		}

		agent := &domain.Agent{
			ID:        uuid.New(),
			Name:      "11-violations-agent",
			AgentType: domain.AgentTypeAI,
			Status:    domain.AgentStatusVerified,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil)

		violations := make([]*domain.CapabilityViolation, 11)
		for i := 0; i < 11; i++ {
			violations[i] = &domain.CapabilityViolation{
				ID:                  uuid.New(),
				AgentID:             agent.ID,
				AttemptedCapability: domain.CapabilityFileRead,
				Severity:            domain.ViolationSeverityLow,
				TrustScoreImpact:    -2,
				IsBlocked:           true,
				CreatedAt:           now.AddDate(0, 0, -i-1),
			}
		}

		mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return(violations, 11, nil)

		score := calculator.calculateCapabilityRisk(agent)

		// Expected: 0.7 - (11 * 0.02) - 0.20 (volume penalty > 10) = 0.28
		expectedScore := 0.7 - (11 * 0.02) - 0.20
		assert.InDelta(t, expectedScore, score, 0.001, "11 violations should trigger -0.20 volume penalty")
		mockRepo.AssertExpectations(t)
	})
}

// ============================================================================
// TEST: Score Boundaries
// ============================================================================

func TestTrustCalculator_ScoreNeverExceedsOne(t *testing.T) {
	mockTrustRepo := new(AgentServiceMockTrustScoreRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockAuditRepo := new(AgentServiceMockAuditLogRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)

	calculator := NewTrustCalculator(mockTrustRepo, mockAPIKeyRepo, mockAuditRepo, mockCapabilityRepo)

	validCert := generateValidCertificate()
	agent := &domain.Agent{
		ID:               uuid.New(),
		Status:           domain.AgentStatusVerified,
		PublicKey:        &validCert,
		CertificateURL:   "https://example.com/cert.pem",
		RepositoryURL:    "https://github.com/test/repo",
		DocumentationURL: "https://docs.example.com",
		Description:      "This is a very comprehensive description with lots of details about the agent.",
		UpdatedAt:        time.Now(),
		CreatedAt:        time.Now().Add(-365 * 24 * time.Hour),
		Version:          "1.0.0",
	}

	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil)
	mockCapabilityRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return([]*domain.CapabilityViolation{}, 0, nil)

	score, err := calculator.Calculate(agent)

	assert.NoError(t, err)
	assert.LessOrEqual(t, score.Score, 1.0, "Score must never exceed 1.0")

	mockCapabilityRepo.AssertExpectations(t)
}

func TestTrustCalculator_ScoreNeverBelowZero(t *testing.T) {
	mockTrustRepo := new(AgentServiceMockTrustScoreRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockAuditRepo := new(AgentServiceMockAuditLogRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)

	calculator := NewTrustCalculator(mockTrustRepo, mockAPIKeyRepo, mockAuditRepo, mockCapabilityRepo)

	agent := &domain.Agent{
		ID:        uuid.New(),
		Status:    domain.AgentStatusRevoked,
		UpdatedAt: time.Now(),
		CreatedAt: time.Now(),
	}

	// Worst case: multiple high-risk capabilities + many critical violations
	capabilities := []*domain.AgentCapability{
		{ID: uuid.New(), AgentID: agent.ID, CapabilityType: domain.CapabilitySystemAdmin},
		{ID: uuid.New(), AgentID: agent.ID, CapabilityType: domain.CapabilityUserImpersonate},
		{ID: uuid.New(), AgentID: agent.ID, CapabilityType: domain.CapabilityFileDelete},
	}

	violations := make([]*domain.CapabilityViolation, 15)
	for i := 0; i < 15; i++ {
		violations[i] = &domain.CapabilityViolation{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilitySystemAdmin,
			Severity:            domain.ViolationSeverityCritical,
			CreatedAt:           time.Now().Add(-time.Duration(i) * time.Hour),
		}
	}

	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(capabilities, nil)
	mockCapabilityRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return(violations, 15, nil)

	score, err := calculator.Calculate(agent)

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, score.Score, 0.0, "Score must never go below 0.0")

	mockCapabilityRepo.AssertExpectations(t)
}

// ============================================================================
// TEST: Confidence Calculation
// ============================================================================

func TestTrustCalculator_Confidence_AllDataPoints(t *testing.T) {
	calculator := &TrustCalculator{}

	validCert := generateValidCertificate()
	agent := &domain.Agent{
		Status:           domain.AgentStatusVerified,
		PublicKey:        &validCert,
		CertificateURL:   "https://example.com/cert.pem",
		RepositoryURL:    "https://github.com/test/repo",
		DocumentationURL: "https://docs.example.com",
		Description:      "Complete description",
		Version:          "1.0.0",
	}

	factors := &domain.TrustScoreFactors{}
	confidence := calculator.calculateConfidence(agent, factors)

	assert.Equal(t, 1.0, confidence, "All data points should yield confidence of 1.0")
}

func TestTrustCalculator_Confidence_NoDataPoints(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{}

	factors := &domain.TrustScoreFactors{}
	confidence := calculator.calculateConfidence(agent, factors)

	assert.Equal(t, 0.0, confidence, "No data points should yield confidence of 0.0")
}

func TestTrustCalculator_Confidence_PartialData(t *testing.T) {
	calculator := &TrustCalculator{}

	agent := &domain.Agent{
		Status:      domain.AgentStatusVerified,
		Description: "Some description",
		Version:     "1.0.0",
	}

	factors := &domain.TrustScoreFactors{}
	confidence := calculator.calculateConfidence(agent, factors)

	expectedConfidence := 3.0 / 7.0 // 3 out of 7 data points
	assert.InDelta(t, expectedConfidence, confidence, 0.01, "Partial data should yield proportional confidence")
}

// ============================================================================
// TEST: Edge Cases
// ============================================================================

func TestTrustCalculator_CapabilityRisk_ErrorFetchingCapabilities(t *testing.T) {
	mockTrustRepo := new(AgentServiceMockTrustScoreRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockAuditRepo := new(AgentServiceMockAuditLogRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)

	calculator := NewTrustCalculator(mockTrustRepo, mockAPIKeyRepo, mockAuditRepo, mockCapabilityRepo)

	agent := &domain.Agent{ID: uuid.New()}

	// Simulate error fetching capabilities
	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(nil, assert.AnError)
	mockCapabilityRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return([]*domain.CapabilityViolation{}, 0, nil)

	score := calculator.calculateCapabilityRisk(agent)

	// Should return baseline score when capabilities can't be fetched
	assert.Equal(t, 0.7, score, "Error fetching capabilities should return baseline score")

	mockCapabilityRepo.AssertExpectations(t)
}

func TestTrustCalculator_CapabilityRisk_ErrorFetchingViolations(t *testing.T) {
	mockTrustRepo := new(AgentServiceMockTrustScoreRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockAuditRepo := new(AgentServiceMockAuditLogRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)

	calculator := NewTrustCalculator(mockTrustRepo, mockAPIKeyRepo, mockAuditRepo, mockCapabilityRepo)

	agent := &domain.Agent{ID: uuid.New()}

	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil)
	mockCapabilityRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return(nil, 0, assert.AnError)

	score := calculator.calculateCapabilityRisk(agent)

	// Should return baseline score when violations can't be fetched
	assert.Equal(t, 0.7, score, "Error fetching violations should return baseline score")

	mockCapabilityRepo.AssertExpectations(t)
}
