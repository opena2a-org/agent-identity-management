package application

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ===========================
// VerificationResult Tests
// ===========================

func TestVerificationResult_Structure(t *testing.T) {
	result := VerificationResult{
		IsValid:      true,
		IsAuthorized: true,
		InScope:      true,
		TrustScore:   0.95,
		Message:      "Action authorized",
	}

	assert.True(t, result.IsValid)
	assert.True(t, result.IsAuthorized)
	assert.True(t, result.InScope)
	assert.Equal(t, 0.95, result.TrustScore)
	assert.Equal(t, "Action authorized", result.Message)
}

func TestVerificationResult_Denied(t *testing.T) {
	result := VerificationResult{
		IsValid:      true,
		IsAuthorized: false,
		InScope:      false,
		TrustScore:   0.65,
		Message:      "Action denied: capability 'file:write' not registered for this agent",
	}

	assert.True(t, result.IsValid)
	assert.False(t, result.IsAuthorized)
	assert.False(t, result.InScope)
	assert.Contains(t, result.Message, "Action denied")
}

func TestVerificationResult_InvalidSignature(t *testing.T) {
	result := VerificationResult{
		IsValid:      false,
		IsAuthorized: false,
		InScope:      false,
		TrustScore:   0.75,
		Message:      "Invalid signature",
	}

	assert.False(t, result.IsValid)
	assert.False(t, result.IsAuthorized)
	assert.False(t, result.InScope)
	assert.Equal(t, "Invalid signature", result.Message)
}

func TestVerificationResult_AgentNotFound(t *testing.T) {
	result := VerificationResult{
		IsValid:      false,
		IsAuthorized: false,
		InScope:      false,
		TrustScore:   0,
		Message:      "Agent not found",
	}

	assert.False(t, result.IsValid)
	assert.Equal(t, "Agent not found", result.Message)
}

// ===========================
// CapabilityDefinition Tests
// ===========================

func TestCapabilityDefinition_Structure(t *testing.T) {
	def := CapabilityDefinition{
		Type:           "file:read",
		Name:           "File Read",
		Description:    "Read files from the filesystem",
		Category:       "file",
		RiskLevel:      "low",
		CapabilityType: "core",
	}

	assert.Equal(t, "file:read", def.Type)
	assert.Equal(t, "File Read", def.Name)
	assert.Equal(t, "Read files from the filesystem", def.Description)
	assert.Equal(t, "file", def.Category)
	assert.Equal(t, "low", def.RiskLevel)
	assert.Equal(t, "core", def.CapabilityType)
}

func TestCapabilityDefinition_CustomCapability(t *testing.T) {
	def := CapabilityDefinition{
		Type:           "myapp:send_email",
		Name:           "Send Email",
		Description:    "Custom capability to send emails",
		Category:       "custom",
		RiskLevel:      "medium",
		CapabilityType: "custom",
	}

	assert.Equal(t, "myapp:send_email", def.Type)
	assert.Equal(t, "custom", def.CapabilityType)
	assert.Equal(t, "custom", def.Category)
}

func TestCapabilityDefinition_AllRiskLevels(t *testing.T) {
	levels := []string{"low", "medium", "high", "critical"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			def := CapabilityDefinition{
				Type:      "test:cap",
				RiskLevel: level,
			}
			assert.Equal(t, level, def.RiskLevel)
		})
	}
}

// ===========================
// ListCapabilitiesResponse Tests
// ===========================

func TestListCapabilitiesResponse_Structure(t *testing.T) {
	response := ListCapabilitiesResponse{
		Capabilities: []CapabilityDefinition{
			{Type: "file:read", Name: "File Read"},
			{Type: "file:write", Name: "File Write"},
		},
		ReservedNamespaces: []string{"file", "db", "api", "system"},
		ValidationPattern:  "^[a-z][a-z0-9_]*:[a-z][a-z0-9_]*$",
	}

	assert.Len(t, response.Capabilities, 2)
	assert.Len(t, response.ReservedNamespaces, 4)
	assert.NotEmpty(t, response.ValidationPattern)
}

func TestListCapabilitiesResponse_Empty(t *testing.T) {
	response := ListCapabilitiesResponse{
		Capabilities:       []CapabilityDefinition{},
		ReservedNamespaces: []string{},
		ValidationPattern:  "",
	}

	assert.Empty(t, response.Capabilities)
	assert.Empty(t, response.ReservedNamespaces)
}

// ===========================
// hasCapability Helper Tests (using CapabilityService)
// ===========================

func TestCapabilityService_HasCapability_Logic(t *testing.T) {
	// Test the hasCapability logic directly
	service := &CapabilityService{}

	capabilities := []*domain.AgentCapability{
		{ID: uuid.New(), CapabilityType: "file:read"},
		{ID: uuid.New(), CapabilityType: "file:write"},
		{ID: uuid.New(), CapabilityType: "api:call"},
	}

	// Test found
	assert.True(t, service.hasCapability(capabilities, "file:read"))
	assert.True(t, service.hasCapability(capabilities, "file:write"))
	assert.True(t, service.hasCapability(capabilities, "api:call"))

	// Test not found
	assert.False(t, service.hasCapability(capabilities, "db:query"))
	assert.False(t, service.hasCapability(capabilities, "system:admin"))

	// Test empty list
	var empty []*domain.AgentCapability
	assert.False(t, service.hasCapability(empty, "file:read"))
}

func TestCapabilityService_HasCapability_ExactMatch(t *testing.T) {
	service := &CapabilityService{}

	capabilities := []*domain.AgentCapability{
		{ID: uuid.New(), CapabilityType: "file:read"},
	}

	// Must be exact match
	assert.True(t, service.hasCapability(capabilities, "file:read"))
	assert.False(t, service.hasCapability(capabilities, "file:read_all"))
	assert.False(t, service.hasCapability(capabilities, "file:"))
	assert.False(t, service.hasCapability(capabilities, "file"))
}

// ===========================
// capabilitiesToMap Helper Tests
// ===========================

func TestCapabilityService_CapabilitiesToMap(t *testing.T) {
	service := &CapabilityService{}

	capabilities := []*domain.AgentCapability{
		{ID: uuid.New(), CapabilityType: "file:read"},
		{ID: uuid.New(), CapabilityType: "file:write"},
		{ID: uuid.New(), CapabilityType: "api:call"},
	}

	result := service.capabilitiesToMap(capabilities)

	assert.Contains(t, result, "capabilities")
	capList := result["capabilities"].([]string)
	assert.Len(t, capList, 3)
	assert.Contains(t, capList, "file:read")
	assert.Contains(t, capList, "file:write")
	assert.Contains(t, capList, "api:call")
}

func TestCapabilityService_CapabilitiesToMap_Empty(t *testing.T) {
	service := &CapabilityService{}

	var capabilities []*domain.AgentCapability
	result := service.capabilitiesToMap(capabilities)

	capList := result["capabilities"].([]string)
	assert.Empty(t, capList)
}

func TestCapabilityService_CapabilitiesToMap_SingleEntry(t *testing.T) {
	service := &CapabilityService{}

	capabilities := []*domain.AgentCapability{
		{ID: uuid.New(), CapabilityType: "db:query"},
	}

	result := service.capabilitiesToMap(capabilities)
	capList := result["capabilities"].([]string)
	assert.Len(t, capList, 1)
	assert.Equal(t, "db:query", capList[0])
}

// ===========================
// calculateSeverity Helper Tests
// ===========================

func TestCapabilityService_CalculateSeverity(t *testing.T) {
	service := &CapabilityService{}

	tests := []struct {
		name             string
		violationCount   int
		expectedSeverity string
	}{
		{"first violation - low", 0, domain.ViolationSeverityLow},
		{"second violation - medium", 1, domain.ViolationSeverityMedium},
		{"third violation - high", 2, domain.ViolationSeverityHigh},
		{"fourth violation - critical", 3, domain.ViolationSeverityCritical},
		{"many violations - critical", 5, domain.ViolationSeverityCritical},
		{"extreme violations - critical", 100, domain.ViolationSeverityCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &domain.Agent{
				ID:                       uuid.New(),
				CapabilityViolationCount: tt.violationCount,
			}

			severity := service.calculateSeverity(agent)
			assert.Equal(t, tt.expectedSeverity, severity)
		})
	}
}

// ===========================
// mcpToolToCapabilityType Helper Tests
// ===========================

func TestCapabilityService_MCPToolToCapabilityType_KnownTools(t *testing.T) {
	service := &CapabilityService{}

	tests := []struct {
		toolName        string
		expectedCapType string
	}{
		{"read_file", domain.CapabilityFileRead},
		{"write_file", domain.CapabilityFileWrite},
		{"delete_file", domain.CapabilityFileDelete},
		{"execute_command", domain.CapabilitySystemAdmin},
		{"query_database", domain.CapabilityDBQuery},
		{"write_database", domain.CapabilityDBWrite},
		{"call_api", domain.CapabilityAPICall},
		{"export_data", domain.CapabilityDataExport},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			capType := service.mcpToolToCapabilityType(tt.toolName)
			assert.Equal(t, tt.expectedCapType, capType)
		})
	}
}

func TestCapabilityService_MCPToolToCapabilityType_UnknownTools(t *testing.T) {
	service := &CapabilityService{}

	tests := []struct {
		toolName       string
		expectedPrefix string
	}{
		{"custom_tool", domain.CapabilityMCPToolUse},
		{"send_email", domain.CapabilityMCPToolUse},
		{"generate_report", domain.CapabilityMCPToolUse},
		{"my_special_function", domain.CapabilityMCPToolUse},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			capType := service.mcpToolToCapabilityType(tt.toolName)
			assert.Contains(t, capType, tt.expectedPrefix)
			assert.Contains(t, capType, tt.toolName)
		})
	}
}

func TestCapabilityService_MCPToolToCapabilityType_EmptyTool(t *testing.T) {
	service := &CapabilityService{}

	capType := service.mcpToolToCapabilityType("")
	assert.Contains(t, capType, domain.CapabilityMCPToolUse)
}

// ===========================
// verifySignature Helper Tests
// ===========================

func TestCapabilityService_VerifySignature_ValidEd25519(t *testing.T) {
	service := &CapabilityService{}

	// Generate a real Ed25519 key pair
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	assert.NoError(t, err)

	payload := []byte("test payload to sign")
	signature := ed25519.Sign(privateKey, payload)

	publicKeyBase64 := base64.StdEncoding.EncodeToString(publicKey)

	result := service.verifySignature(publicKeyBase64, "Ed25519", signature, payload)
	assert.True(t, result)
}

func TestCapabilityService_VerifySignature_InvalidSignature(t *testing.T) {
	service := &CapabilityService{}

	publicKey, _, err := ed25519.GenerateKey(nil)
	assert.NoError(t, err)

	payload := []byte("test payload")
	invalidSignature := make([]byte, ed25519.SignatureSize)

	publicKeyBase64 := base64.StdEncoding.EncodeToString(publicKey)

	result := service.verifySignature(publicKeyBase64, "Ed25519", invalidSignature, payload)
	assert.False(t, result)
}

func TestCapabilityService_VerifySignature_WrongPayload(t *testing.T) {
	service := &CapabilityService{}

	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	assert.NoError(t, err)

	originalPayload := []byte("original payload")
	differentPayload := []byte("different payload")
	signature := ed25519.Sign(privateKey, originalPayload)

	publicKeyBase64 := base64.StdEncoding.EncodeToString(publicKey)

	result := service.verifySignature(publicKeyBase64, "Ed25519", signature, differentPayload)
	assert.False(t, result)
}

func TestCapabilityService_VerifySignature_InvalidPublicKeyEncoding(t *testing.T) {
	service := &CapabilityService{}

	result := service.verifySignature("not-valid-base64!@#$", "Ed25519", []byte("sig"), []byte("payload"))
	assert.False(t, result)
}

func TestCapabilityService_VerifySignature_WrongKeySize(t *testing.T) {
	service := &CapabilityService{}

	// Create a key that's not the right size for Ed25519 (should be 32 bytes)
	wrongSizeKey := base64.StdEncoding.EncodeToString([]byte("short key"))

	result := service.verifySignature(wrongSizeKey, "Ed25519", []byte("sig"), []byte("payload"))
	assert.False(t, result)
}

func TestCapabilityService_VerifySignature_UnsupportedAlgorithm(t *testing.T) {
	service := &CapabilityService{}

	publicKey := base64.StdEncoding.EncodeToString(make([]byte, 32))

	result := service.verifySignature(publicKey, "RSA", []byte("sig"), []byte("payload"))
	assert.False(t, result)
}

func TestCapabilityService_VerifySignature_EmptyAlgorithm(t *testing.T) {
	service := &CapabilityService{}

	// Generate a real Ed25519 key pair
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	assert.NoError(t, err)

	payload := []byte("test payload")
	signature := ed25519.Sign(privateKey, payload)

	publicKeyBase64 := base64.StdEncoding.EncodeToString(publicKey)

	// Empty algorithm should default to Ed25519
	result := service.verifySignature(publicKeyBase64, "", signature, payload)
	assert.True(t, result)
}

func TestCapabilityService_VerifySignature_DifferentKeys(t *testing.T) {
	service := &CapabilityService{}

	// Generate two different key pairs
	publicKey1, _, _ := ed25519.GenerateKey(nil)
	_, privateKey2, _ := ed25519.GenerateKey(nil)

	payload := []byte("test payload")
	// Sign with key2's private key
	signature := ed25519.Sign(privateKey2, payload)

	// Verify with key1's public key - should fail
	publicKeyBase64 := base64.StdEncoding.EncodeToString(publicKey1)

	result := service.verifySignature(publicKeyBase64, "Ed25519", signature, payload)
	assert.False(t, result)
}

// ===========================
// Trust Score Impact Tests (Logic only)
// ===========================

func TestTrustScoreDecreaseLogic(t *testing.T) {
	tests := []struct {
		name             string
		currentScore     float64
		decrease         float64
		expectedNewScore float64
	}{
		{"normal decrease", 0.75, 0.10, 0.65},
		{"decrease to zero", 0.05, 0.10, 0.0},
		{"large decrease", 0.50, 0.50, 0.0},
		{"max score decrease", 1.0, 0.10, 0.90},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newScore := tt.currentScore - tt.decrease
			if newScore < 0 {
				newScore = 0
			}
			assert.Equal(t, tt.expectedNewScore, newScore)
		})
	}
}

func TestCompromisedAgentThresholds(t *testing.T) {
	tests := []struct {
		name             string
		violationCount   int
		trustScore       float64
		shouldCompromise bool
	}{
		{"3 violations - compromised", 3, 0.50, true},
		{"4 violations - compromised", 4, 0.60, true},
		{"2 violations high trust - safe", 2, 0.80, false},
		{"1 violation low trust - compromised", 1, 0.25, true},
		{"0 violations low trust - compromised", 0, 0.20, true},
		{"2 violations boundary - safe", 2, 0.35, false},
		{"2 violations below boundary - compromised", 2, 0.29, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Logic from VerifyAction: compromised if violationCount >= 3 OR trustScore < 0.30
			isCompromised := tt.violationCount >= 3 || tt.trustScore < 0.30
			assert.Equal(t, tt.shouldCompromise, isCompromised)
		})
	}
}

// ===========================
// Capability Scope Tests
// ===========================

func TestAgentCapability_Structure(t *testing.T) {
	now := time.Now()
	agentID := uuid.New()
	grantedBy := uuid.New()

	capability := domain.AgentCapability{
		ID:              uuid.New(),
		AgentID:         agentID,
		CapabilityType:  "file:read",
		CapabilityScope: map[string]interface{}{"path": "/tmp", "recursive": true},
		GrantedBy:       &grantedBy,
		GrantedAt:       now,
		RevokedAt:       nil,
	}

	assert.NotEqual(t, uuid.Nil, capability.ID)
	assert.Equal(t, agentID, capability.AgentID)
	assert.Equal(t, "file:read", capability.CapabilityType)
	assert.Equal(t, "/tmp", capability.CapabilityScope["path"])
	assert.Equal(t, true, capability.CapabilityScope["recursive"])
	assert.NotNil(t, capability.GrantedBy)
	assert.Nil(t, capability.RevokedAt)
}

func TestAgentCapability_Revoked(t *testing.T) {
	now := time.Now()
	revokedAt := now.Add(time.Hour)

	capability := domain.AgentCapability{
		ID:             uuid.New(),
		AgentID:        uuid.New(),
		CapabilityType: "db:write",
		GrantedAt:      now,
		RevokedAt:      &revokedAt,
	}

	assert.NotNil(t, capability.RevokedAt)
	assert.True(t, capability.RevokedAt.After(capability.GrantedAt))
}

// ===========================
// CapabilityViolation Tests
// ===========================

func TestCapabilityViolation_Structure(t *testing.T) {
	now := time.Now()
	agentID := uuid.New()
	sourceIP := "192.168.1.100"

	violation := domain.CapabilityViolation{
		ID:                  uuid.New(),
		AgentID:             agentID,
		AttemptedCapability: "system:admin",
		RegisteredCapabilities: map[string]interface{}{
			"capabilities": []string{"file:read", "file:write"},
		},
		Severity:         domain.ViolationSeverityCritical,
		TrustScoreImpact: -15,
		IsBlocked:        true,
		SourceIP:         &sourceIP,
		RequestMetadata: map[string]interface{}{
			"endpoint": "/admin/config",
			"method":   "POST",
		},
		CreatedAt: now,
	}

	assert.NotEqual(t, uuid.Nil, violation.ID)
	assert.Equal(t, agentID, violation.AgentID)
	assert.Equal(t, "system:admin", violation.AttemptedCapability)
	assert.Equal(t, domain.ViolationSeverityCritical, violation.Severity)
	assert.Equal(t, -15, violation.TrustScoreImpact)
	assert.True(t, violation.IsBlocked)
	assert.Equal(t, "192.168.1.100", *violation.SourceIP)
}

func TestCapabilityViolation_AllSeverities(t *testing.T) {
	severities := []string{
		domain.ViolationSeverityLow,
		domain.ViolationSeverityMedium,
		domain.ViolationSeverityHigh,
		domain.ViolationSeverityCritical,
	}

	for _, sev := range severities {
		t.Run(sev, func(t *testing.T) {
			violation := domain.CapabilityViolation{
				ID:       uuid.New(),
				Severity: sev,
			}
			assert.Equal(t, sev, violation.Severity)
		})
	}
}

// ===========================
// Constructor Tests
// ===========================

func TestNewCapabilityService(t *testing.T) {
	service := NewCapabilityService(nil, nil, nil, nil, nil, nil)

	assert.NotNil(t, service)
	assert.Nil(t, service.capabilityRepo)
	assert.Nil(t, service.agentRepo)
	assert.Nil(t, service.auditRepo)
	assert.Nil(t, service.trustCalc)
	assert.Nil(t, service.trustScoreRepo)
}

func TestNewCapabilityService_WithMocks(t *testing.T) {
	mockAgentRepo := new(MockAgentRepoForCapability)

	service := NewCapabilityService(nil, mockAgentRepo, nil, nil, nil, nil)

	assert.NotNil(t, service)
	assert.NotNil(t, service.agentRepo)
}

// ========================================
// Mock Repositories for Service Tests
// ========================================

type MockCapabilityRepoForService struct {
	mock.Mock
}

func (m *MockCapabilityRepoForService) CreateCapability(capability *domain.AgentCapability) error {
	args := m.Called(capability)
	return args.Error(0)
}

func (m *MockCapabilityRepoForService) GetCapabilityByID(id uuid.UUID) (*domain.AgentCapability, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AgentCapability), args.Error(1)
}

func (m *MockCapabilityRepoForService) GetCapabilitiesByAgentID(agentID uuid.UUID) ([]*domain.AgentCapability, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AgentCapability), args.Error(1)
}

func (m *MockCapabilityRepoForService) GetActiveCapabilitiesByAgentID(agentID uuid.UUID) ([]*domain.AgentCapability, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AgentCapability), args.Error(1)
}

func (m *MockCapabilityRepoForService) RevokeCapability(id uuid.UUID, revokedAt time.Time) error {
	args := m.Called(id, revokedAt)
	return args.Error(0)
}

func (m *MockCapabilityRepoForService) SetHoneytoken(id uuid.UUID, honeytoken bool) error {
	args := m.Called(id, honeytoken)
	return args.Error(0)
}

func (m *MockCapabilityRepoForService) DeleteCapability(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockCapabilityRepoForService) CreateViolation(violation *domain.CapabilityViolation) error {
	args := m.Called(violation)
	return args.Error(0)
}

func (m *MockCapabilityRepoForService) GetViolationByID(id uuid.UUID) (*domain.CapabilityViolation, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.CapabilityViolation), args.Error(1)
}

func (m *MockCapabilityRepoForService) GetViolationsByAgentID(agentID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error) {
	args := m.Called(agentID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.CapabilityViolation), args.Int(1), args.Error(2)
}

func (m *MockCapabilityRepoForService) GetRecentViolations(orgID uuid.UUID, minutes int) ([]*domain.CapabilityViolation, error) {
	args := m.Called(orgID, minutes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.CapabilityViolation), args.Error(1)
}

func (m *MockCapabilityRepoForService) GetViolationsByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error) {
	args := m.Called(orgID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.CapabilityViolation), args.Int(1), args.Error(2)
}

func (m *MockCapabilityRepoForService) ListCapabilityDefinitions(orgID *uuid.UUID) ([]*domain.CapabilityDefinition, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.CapabilityDefinition), args.Error(1)
}

func (m *MockCapabilityRepoForService) GetCapabilityDefinition(namespace, action string, orgID *uuid.UUID) (*domain.CapabilityDefinition, error) {
	args := m.Called(namespace, action, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.CapabilityDefinition), args.Error(1)
}

func (m *MockCapabilityRepoForService) CreateCapabilityDefinition(def *domain.CapabilityDefinition) error {
	args := m.Called(def)
	return args.Error(0)
}

func (m *MockCapabilityRepoForService) UpdateCapabilityDefinition(def *domain.CapabilityDefinition) error {
	args := m.Called(def)
	return args.Error(0)
}

type MockAgentRepoForCapability struct {
	mock.Mock
}

func (m *MockAgentRepoForCapability) Create(agent *domain.Agent) error {
	args := m.Called(agent)
	return args.Error(0)
}

func (m *MockAgentRepoForCapability) GetByID(id uuid.UUID) (*domain.Agent, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForCapability) GetByName(orgID uuid.UUID, name string) (*domain.Agent, error) {
	args := m.Called(orgID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForCapability) GetByOrganization(orgID uuid.UUID) ([]*domain.Agent, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForCapability) Update(agent *domain.Agent) error {
	args := m.Called(agent)
	return args.Error(0)
}

func (m *MockAgentRepoForCapability) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAgentRepoForCapability) List(limit, offset int) ([]*domain.Agent, error) {
	args := m.Called(limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForCapability) ListRevokedIDs(limit, offset int) ([]uuid.UUID, error) {
	args := m.Called(limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]uuid.UUID), args.Error(1)
}

func (m *MockAgentRepoForCapability) UpdateTrustScore(id uuid.UUID, score float64) error {
	args := m.Called(id, score)
	return args.Error(0)
}

func (m *MockAgentRepoForCapability) IncrementViolationCount(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAgentRepoForCapability) MarkAsCompromised(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAgentRepoForCapability) UpdateLastActive(ctx context.Context, agentID uuid.UUID) error {
	args := m.Called(ctx, agentID)
	return args.Error(0)
}

func (m *MockAgentRepoForCapability) GetStaleAgents(ctx context.Context, staleSince time.Time) ([]*domain.Agent, error) {
	args := m.Called(ctx, staleSince)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForCapability) GetByIDs(ctx context.Context, callerOrgID uuid.UUID, ids []uuid.UUID) ([]*domain.Agent, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

type MockAuditRepoForCapability struct {
	mock.Mock
}

func (m *MockAuditRepoForCapability) Create(log *domain.AuditLog) error {
	args := m.Called(log)
	return args.Error(0)
}

func (m *MockAuditRepoForCapability) GetByID(id uuid.UUID) (*domain.AuditLog, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AuditLog), args.Error(1)
}

func (m *MockAuditRepoForCapability) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	args := m.Called(orgID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *MockAuditRepoForCapability) GetByUser(orgID, userID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	args := m.Called(orgID, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *MockAuditRepoForCapability) GetByResource(orgID uuid.UUID, resourceType string, resourceID uuid.UUID) ([]*domain.AuditLog, error) {
	args := m.Called(orgID, resourceType, resourceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *MockAuditRepoForCapability) Search(query string, limit, offset int) ([]*domain.AuditLog, error) {
	args := m.Called(query, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *MockAuditRepoForCapability) GetByAgent(orgID, agentID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	args := m.Called(orgID, agentID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *MockAuditRepoForCapability) CountActionsByAgentInTimeWindow(agentID uuid.UUID, action domain.AuditAction, windowMinutes int) (int, error) {
	args := m.Called(agentID, action, windowMinutes)
	return args.Int(0), args.Error(1)
}

func (m *MockAuditRepoForCapability) GetRecentActionsByAgent(agentID uuid.UUID, limit int) ([]*domain.AuditLog, error) {
	args := m.Called(agentID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *MockAuditRepoForCapability) GetAgentActionsByIPAddress(agentID uuid.UUID, ipAddress string, limit int) ([]*domain.AuditLog, error) {
	args := m.Called(agentID, ipAddress, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

type MockTrustCalculatorForCapability struct {
	mock.Mock
}

func (m *MockTrustCalculatorForCapability) Calculate(agent *domain.Agent) (*domain.TrustScore, error) {
	args := m.Called(agent)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TrustScore), args.Error(1)
}

func (m *MockTrustCalculatorForCapability) CalculateFactors(agent *domain.Agent) (*domain.TrustScoreFactors, error) {
	args := m.Called(agent)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TrustScoreFactors), args.Error(1)
}

type MockTrustScoreRepoForCapability struct {
	mock.Mock
}

func (m *MockTrustScoreRepoForCapability) Create(score *domain.TrustScore) error {
	args := m.Called(score)
	return args.Error(0)
}

func (m *MockTrustScoreRepoForCapability) GetByAgent(agentID uuid.UUID) (*domain.TrustScore, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TrustScore), args.Error(1)
}

func (m *MockTrustScoreRepoForCapability) GetLatest(agentID uuid.UUID) (*domain.TrustScore, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TrustScore), args.Error(1)
}

func (m *MockTrustScoreRepoForCapability) GetHistory(agentID uuid.UUID, limit int) ([]*domain.TrustScore, error) {
	args := m.Called(agentID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TrustScore), args.Error(1)
}

func (m *MockTrustScoreRepoForCapability) GetHistoryAuditTrail(agentID uuid.UUID, limit int) ([]*domain.TrustScoreHistoryEntry, error) {
	args := m.Called(agentID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TrustScoreHistoryEntry), args.Error(1)
}

func (m *MockTrustScoreRepoForCapability) UpdateScore(agentID uuid.UUID, newScore float64) error {
	args := m.Called(agentID, newScore)
	return args.Error(0)
}

// ========================================
// Service Method Tests with Mocks
// ========================================

func TestCapabilityService_GetAgentCapabilities_ActiveOnly(t *testing.T) {
	mockCapRepo := new(MockCapabilityRepoForService)
	service := NewCapabilityService(mockCapRepo, nil, nil, nil, nil, nil)

	agentID := uuid.New()
	expectedCaps := []*domain.AgentCapability{
		{ID: uuid.New(), AgentID: agentID, CapabilityType: "file:read"},
		{ID: uuid.New(), AgentID: agentID, CapabilityType: "file:write"},
	}

	mockCapRepo.On("GetActiveCapabilitiesByAgentID", agentID).Return(expectedCaps, nil)

	result, err := service.GetAgentCapabilities(context.Background(), agentID, true)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	mockCapRepo.AssertExpectations(t)
}

func TestCapabilityService_GetAgentCapabilities_All(t *testing.T) {
	mockCapRepo := new(MockCapabilityRepoForService)
	service := NewCapabilityService(mockCapRepo, nil, nil, nil, nil, nil)

	agentID := uuid.New()
	now := time.Now()
	expectedCaps := []*domain.AgentCapability{
		{ID: uuid.New(), AgentID: agentID, CapabilityType: "file:read"},
		{ID: uuid.New(), AgentID: agentID, CapabilityType: "file:write", RevokedAt: &now},
	}

	mockCapRepo.On("GetCapabilitiesByAgentID", agentID).Return(expectedCaps, nil)

	result, err := service.GetAgentCapabilities(context.Background(), agentID, false)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	mockCapRepo.AssertExpectations(t)
}

func TestCapabilityService_GetViolationsByAgent_Success(t *testing.T) {
	mockCapRepo := new(MockCapabilityRepoForService)
	service := NewCapabilityService(mockCapRepo, nil, nil, nil, nil, nil)

	agentID := uuid.New()
	expectedViolations := []*domain.CapabilityViolation{
		{ID: uuid.New(), AgentID: agentID, AttemptedCapability: "system:admin"},
	}

	mockCapRepo.On("GetViolationsByAgentID", agentID, 10, 0).Return(expectedViolations, 1, nil)

	result, total, err := service.GetViolationsByAgent(context.Background(), agentID, 10, 0)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, 1, total)
	mockCapRepo.AssertExpectations(t)
}

func TestCapabilityService_GetViolationsByOrganization_Success(t *testing.T) {
	mockCapRepo := new(MockCapabilityRepoForService)
	service := NewCapabilityService(mockCapRepo, nil, nil, nil, nil, nil)

	orgID := uuid.New()
	expectedViolations := []*domain.CapabilityViolation{
		{ID: uuid.New(), AttemptedCapability: "system:admin"},
		{ID: uuid.New(), AttemptedCapability: "db:delete"},
	}

	mockCapRepo.On("GetViolationsByOrganization", orgID, 10, 0).Return(expectedViolations, 2, nil)

	result, total, err := service.GetViolationsByOrganization(context.Background(), orgID, 10, 0)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, 2, total)
	mockCapRepo.AssertExpectations(t)
}

func TestCapabilityService_GetRecentViolations_Success(t *testing.T) {
	mockCapRepo := new(MockCapabilityRepoForService)
	service := NewCapabilityService(mockCapRepo, nil, nil, nil, nil, nil)

	orgID := uuid.New()
	expectedViolations := []*domain.CapabilityViolation{
		{ID: uuid.New(), AttemptedCapability: "system:exec"},
	}

	mockCapRepo.On("GetRecentViolations", orgID, 60).Return(expectedViolations, nil)

	result, err := service.GetRecentViolations(context.Background(), orgID, 60)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	mockCapRepo.AssertExpectations(t)
}

func TestCapabilityService_ListCapabilities_Success(t *testing.T) {
	mockCapRepo := new(MockCapabilityRepoForService)
	service := NewCapabilityService(mockCapRepo, nil, nil, nil, nil, nil)

	orgID := uuid.New()
	expectedDefs := []*domain.CapabilityDefinition{
		{Namespace: "file", Action: "read", DisplayName: "File Read", Description: "Read files", Category: "file", RiskLevel: "low", CapabilityType: "core"},
		{Namespace: "file", Action: "write", DisplayName: "File Write", Description: "Write files", Category: "file", RiskLevel: "medium", CapabilityType: "core"},
	}

	mockCapRepo.On("ListCapabilityDefinitions", &orgID).Return(expectedDefs, nil)

	result, err := service.ListCapabilities(context.Background(), orgID)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "file:read", result[0].Type)
	assert.Equal(t, "File Read", result[0].Name)
	mockCapRepo.AssertExpectations(t)
}

func TestCapabilityService_ListCapabilitiesWithMetadata_Success(t *testing.T) {
	mockCapRepo := new(MockCapabilityRepoForService)
	service := NewCapabilityService(mockCapRepo, nil, nil, nil, nil, nil)

	orgID := uuid.New()
	expectedDefs := []*domain.CapabilityDefinition{
		{Namespace: "file", Action: "read", DisplayName: "File Read", Description: "Read files", Category: "file", RiskLevel: "low", CapabilityType: "core"},
	}

	mockCapRepo.On("ListCapabilityDefinitions", &orgID).Return(expectedDefs, nil)

	result, err := service.ListCapabilitiesWithMetadata(context.Background(), orgID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Capabilities, 1)
	assert.NotEmpty(t, result.ReservedNamespaces)
	assert.NotEmpty(t, result.ValidationPattern)
	mockCapRepo.AssertExpectations(t)
}

func TestCapabilityService_GrantCapability_Success(t *testing.T) {
	mockCapRepo := new(MockCapabilityRepoForService)
	mockAgentRepo := new(MockAgentRepoForCapability)
	mockAuditRepo := new(MockAuditRepoForCapability)
	mockTrustCalc := new(MockTrustCalculatorForCapability)
	mockTrustScoreRepo := new(MockTrustScoreRepoForCapability)

	service := NewCapabilityService(mockCapRepo, mockAgentRepo, mockAuditRepo, nil, mockTrustCalc, mockTrustScoreRepo)

	agentID := uuid.New()
	orgID := uuid.New()
	grantedBy := uuid.New()
	agent := &domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		DisplayName:    "Test Agent",
		TrustScore:     0.85,
	}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)
	mockCapRepo.On("GetCapabilityDefinition", "file", "read", mock.AnythingOfType("*uuid.UUID")).Return(&domain.CapabilityDefinition{RiskLevel: domain.RiskLevelLow}, nil)
	mockCapRepo.On("CreateCapability", mock.AnythingOfType("*domain.AgentCapability")).Return(nil)
	mockAuditRepo.On("Create", mock.AnythingOfType("*domain.AuditLog")).Return(nil)
	mockTrustCalc.On("Calculate", agent).Return(&domain.TrustScore{Score: 0.90}, nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)
	mockTrustScoreRepo.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil)

	result, err := service.GrantCapability(context.Background(), agentID, "file:read", nil, &grantedBy, "")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, agentID, result.AgentID)
	assert.Equal(t, "file:read", result.CapabilityType)
	mockCapRepo.AssertExpectations(t)
	mockAgentRepo.AssertExpectations(t)
}

func TestCapabilityService_GrantCapability_AgentNotFound(t *testing.T) {
	mockCapRepo := new(MockCapabilityRepoForService)
	mockAgentRepo := new(MockAgentRepoForCapability)

	service := NewCapabilityService(mockCapRepo, mockAgentRepo, nil, nil, nil, nil)

	agentID := uuid.New()
	grantedBy := uuid.New()

	mockAgentRepo.On("GetByID", agentID).Return(nil, assert.AnError)

	result, err := service.GrantCapability(context.Background(), agentID, "file:read", nil, &grantedBy, "")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "agent not found")
	mockAgentRepo.AssertExpectations(t)
}

func TestCapabilityService_RevokeCapability_Success(t *testing.T) {
	mockCapRepo := new(MockCapabilityRepoForService)
	mockAgentRepo := new(MockAgentRepoForCapability)
	mockAuditRepo := new(MockAuditRepoForCapability)
	mockTrustCalc := new(MockTrustCalculatorForCapability)
	mockTrustScoreRepo := new(MockTrustScoreRepoForCapability)

	service := NewCapabilityService(mockCapRepo, mockAgentRepo, mockAuditRepo, nil, mockTrustCalc, mockTrustScoreRepo)

	capID := uuid.New()
	agentID := uuid.New()
	orgID := uuid.New()
	revokedBy := uuid.New()

	capability := &domain.AgentCapability{
		ID:             capID,
		AgentID:        agentID,
		CapabilityType: "file:write",
	}
	agent := &domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		DisplayName:    "Test Agent",
		TrustScore:     0.85,
	}

	mockCapRepo.On("GetCapabilityByID", capID).Return(capability, nil)
	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)
	mockCapRepo.On("RevokeCapability", capID, mock.AnythingOfType("time.Time")).Return(nil)
	mockAuditRepo.On("Create", mock.AnythingOfType("*domain.AuditLog")).Return(nil)
	mockTrustCalc.On("Calculate", agent).Return(&domain.TrustScore{Score: 0.80}, nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)
	mockTrustScoreRepo.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil)

	err := service.RevokeCapability(context.Background(), capID, &revokedBy)

	assert.NoError(t, err)
	mockCapRepo.AssertExpectations(t)
	mockAgentRepo.AssertExpectations(t)
}

func TestCapabilityService_RevokeCapability_NotFound(t *testing.T) {
	mockCapRepo := new(MockCapabilityRepoForService)

	service := NewCapabilityService(mockCapRepo, nil, nil, nil, nil, nil)

	capID := uuid.New()
	revokedBy := uuid.New()

	mockCapRepo.On("GetCapabilityByID", capID).Return(nil, assert.AnError)

	err := service.RevokeCapability(context.Background(), capID, &revokedBy)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "capability not found")
	mockCapRepo.AssertExpectations(t)
}

func TestCapabilityService_AutoDetectCapabilities_WithTools(t *testing.T) {
	mockCapRepo := new(MockCapabilityRepoForService)
	service := NewCapabilityService(mockCapRepo, nil, nil, nil, nil, nil)

	agentID := uuid.New()
	mcpMetadata := map[string]interface{}{
		"tools": []interface{}{
			map[string]interface{}{
				"name":        "read_file",
				"description": "Read a file from the filesystem",
			},
			map[string]interface{}{
				"name":        "write_file",
				"description": "Write to a file",
			},
		},
	}

	mockCapRepo.On("CreateCapability", mock.AnythingOfType("*domain.AgentCapability")).Return(nil).Times(2)

	err := service.AutoDetectCapabilities(context.Background(), agentID, mcpMetadata)

	assert.NoError(t, err)
	mockCapRepo.AssertExpectations(t)
}

func TestCapabilityService_AutoDetectCapabilities_NoTools(t *testing.T) {
	mockCapRepo := new(MockCapabilityRepoForService)
	service := NewCapabilityService(mockCapRepo, nil, nil, nil, nil, nil)

	agentID := uuid.New()
	mcpMetadata := map[string]interface{}{
		"version": "1.0",
	}

	err := service.AutoDetectCapabilities(context.Background(), agentID, mcpMetadata)

	assert.NoError(t, err)
	// No capabilities should be created
	mockCapRepo.AssertNotCalled(t, "CreateCapability", mock.Anything)
}

func TestCapabilityService_ValidateAndRegisterCapability_CoreCapability(t *testing.T) {
	mockCapRepo := new(MockCapabilityRepoForService)
	service := NewCapabilityService(mockCapRepo, nil, nil, nil, nil, nil)

	orgID := uuid.New()
	capability := "file:read"

	mockCapRepo.On("GetCapabilityDefinition", "file", "read", (*uuid.UUID)(nil)).Return(&domain.CapabilityDefinition{}, nil)

	err := service.ValidateAndRegisterCapability(context.Background(), capability, orgID)

	assert.NoError(t, err)
	mockCapRepo.AssertExpectations(t)
}

func TestCapabilityService_ValidateAndRegisterCapability_InvalidFormat(t *testing.T) {
	mockCapRepo := new(MockCapabilityRepoForService)
	service := NewCapabilityService(mockCapRepo, nil, nil, nil, nil, nil)

	orgID := uuid.New()
	capability := "invalid" // No colon

	err := service.ValidateAndRegisterCapability(context.Background(), capability, orgID)

	assert.Error(t, err)
}

// GetByOrganizationPaged pages over GetByOrganization for tests.
func (m *MockAgentRepoForCapability) GetByOrganizationPaged(orgID uuid.UUID, limit, offset int) ([]*domain.Agent, int, error) {
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
func (m *MockCapabilityRepoForService) GetCapabilitiesByAgentIDs(agentIDs []uuid.UUID, activeOnly bool) (map[uuid.UUID][]*domain.AgentCapability, error) {
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
