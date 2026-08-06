package application

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mock repositories for registry bridge tests ---

type mockOrgRepoForBridge struct {
	mock.Mock
}

func (m *mockOrgRepoForBridge) Create(org *domain.Organization) error {
	return m.Called(org).Error(0)
}
func (m *mockOrgRepoForBridge) GetByID(id uuid.UUID) (*domain.Organization, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Organization), args.Error(1)
}
func (m *mockOrgRepoForBridge) GetByDomain(d string) (*domain.Organization, error) {
	args := m.Called(d)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Organization), args.Error(1)
}
func (m *mockOrgRepoForBridge) ListAll() ([]*domain.Organization, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Organization), args.Error(1)
}
func (m *mockOrgRepoForBridge) Update(org *domain.Organization) error {
	return m.Called(org).Error(0)
}
func (m *mockOrgRepoForBridge) Delete(id uuid.UUID) error {
	return m.Called(id).Error(0)
}

type mockAgentRepoForBridge struct {
	mock.Mock
}

func (m *mockAgentRepoForBridge) Create(a *domain.Agent) error { return nil }
func (m *mockAgentRepoForBridge) GetByID(id uuid.UUID) (*domain.Agent, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}
func (m *mockAgentRepoForBridge) GetByName(orgID uuid.UUID, name string) (*domain.Agent, error) {
	return nil, nil
}
func (m *mockAgentRepoForBridge) GetByOrganization(orgID uuid.UUID) ([]*domain.Agent, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}
func (m *mockAgentRepoForBridge) Update(a *domain.Agent) error                    { return nil }
func (m *mockAgentRepoForBridge) Delete(id uuid.UUID) error                       { return nil }
func (m *mockAgentRepoForBridge) List(limit, offset int) ([]*domain.Agent, error) { return nil, nil }

func (m *mockAgentRepoForBridge) ListRevokedIDs(limit, offset int) ([]uuid.UUID, error) {
	return nil, nil
}
func (m *mockAgentRepoForBridge) UpdateTrustScore(id uuid.UUID, s float64) error { return nil }
func (m *mockAgentRepoForBridge) IncrementViolationCount(id uuid.UUID) error     { return nil }
func (m *mockAgentRepoForBridge) MarkAsCompromised(id uuid.UUID) error           { return nil }
func (m *mockAgentRepoForBridge) UpdateLastActive(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (m *mockAgentRepoForBridge) GetStaleAgents(ctx context.Context, since time.Time) ([]*domain.Agent, error) {
	return nil, nil
}
func (m *mockAgentRepoForBridge) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*domain.Agent, error) {
	return nil, nil
}

type mockAttestationRepoForBridge struct {
	mock.Mock
}

func (m *mockAttestationRepoForBridge) CreateAttestation(a *domain.MCPAttestation) error { return nil }
func (m *mockAttestationRepoForBridge) GetAttestationByID(id uuid.UUID) (*domain.MCPAttestation, error) {
	return nil, nil
}
func (m *mockAttestationRepoForBridge) GetAttestationsByMCP(id uuid.UUID) ([]*domain.MCPAttestation, error) {
	return nil, nil
}
func (m *mockAttestationRepoForBridge) GetValidAttestationsByMCP(id uuid.UUID) ([]*domain.MCPAttestation, error) {
	return nil, nil
}
func (m *mockAttestationRepoForBridge) GetAttestationsByAgent(agentID uuid.UUID) ([]*domain.MCPAttestation, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.MCPAttestation), args.Error(1)
}
func (m *mockAttestationRepoForBridge) GetAllAttestationsByOrganization(orgID uuid.UUID, limit int) ([]*domain.MCPAttestation, error) {
	return nil, nil
}
func (m *mockAttestationRepoForBridge) InvalidateAttestation(id uuid.UUID) error { return nil }
func (m *mockAttestationRepoForBridge) InvalidateExpiredAttestations() error     { return nil }
func (m *mockAttestationRepoForBridge) CreateConnection(c *domain.AgentMCPConnection) error {
	return nil
}
func (m *mockAttestationRepoForBridge) GetConnectionByID(id uuid.UUID) (*domain.AgentMCPConnection, error) {
	return nil, nil
}
func (m *mockAttestationRepoForBridge) GetConnectionByAgentAndMCP(agentID, mcpID uuid.UUID) (*domain.AgentMCPConnection, error) {
	return nil, nil
}
func (m *mockAttestationRepoForBridge) GetConnectionsByAgent(agentID uuid.UUID) ([]*domain.AgentMCPConnection, error) {
	return nil, nil
}
func (m *mockAttestationRepoForBridge) GetConnectionsByMCP(mcpID uuid.UUID) ([]*domain.AgentMCPConnection, error) {
	return nil, nil
}
func (m *mockAttestationRepoForBridge) UpdateConnection(c *domain.AgentMCPConnection) error {
	return nil
}
func (m *mockAttestationRepoForBridge) DeleteConnection(id uuid.UUID) error { return nil }
func (m *mockAttestationRepoForBridge) UpdateMCPConfidenceScore(mcpID uuid.UUID, score float64, count int, lastAttested time.Time) error {
	return nil
}

type mockMCPServerRepoForBridge struct {
	mock.Mock
}

func (m *mockMCPServerRepoForBridge) Create(s *domain.MCPServer) error { return nil }
func (m *mockMCPServerRepoForBridge) GetByID(id uuid.UUID) (*domain.MCPServer, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MCPServer), args.Error(1)
}
func (m *mockMCPServerRepoForBridge) GetByOrganization(orgID uuid.UUID) ([]*domain.MCPServer, error) {
	return nil, nil
}
func (m *mockMCPServerRepoForBridge) GetByURL(url string, orgID uuid.UUID) (*domain.MCPServer, error) {
	return nil, nil
}
func (m *mockMCPServerRepoForBridge) Update(s *domain.MCPServer) error { return nil }
func (m *mockMCPServerRepoForBridge) Delete(id uuid.UUID) error        { return nil }
func (m *mockMCPServerRepoForBridge) List(limit, offset int) ([]*domain.MCPServer, error) {
	return nil, nil
}
func (m *mockMCPServerRepoForBridge) GetVerificationStatus(id uuid.UUID) (*domain.MCPServerVerificationStatus, error) {
	return nil, nil
}

type mockAuditLogRepoForBridge struct {
	mock.Mock
}

func (m *mockAuditLogRepoForBridge) Create(l *domain.AuditLog) error { return nil }
func (m *mockAuditLogRepoForBridge) GetByID(id uuid.UUID) (*domain.AuditLog, error) {
	return nil, nil
}
func (m *mockAuditLogRepoForBridge) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	return nil, nil
}
func (m *mockAuditLogRepoForBridge) GetByUser(orgID, userID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	return nil, nil
}
func (m *mockAuditLogRepoForBridge) GetByResource(orgID uuid.UUID, resourceType string, resourceID uuid.UUID) ([]*domain.AuditLog, error) {
	return nil, nil
}
func (m *mockAuditLogRepoForBridge) Search(query string, limit, offset int) ([]*domain.AuditLog, error) {
	return nil, nil
}
func (m *mockAuditLogRepoForBridge) GetByAgent(orgID, agentID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	args := m.Called(orgID, agentID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}
func (m *mockAuditLogRepoForBridge) CountActionsByAgentInTimeWindow(agentID uuid.UUID, action domain.AuditAction, windowMinutes int) (int, error) {
	return 0, nil
}
func (m *mockAuditLogRepoForBridge) GetRecentActionsByAgent(agentID uuid.UUID, limit int) ([]*domain.AuditLog, error) {
	return nil, nil
}
func (m *mockAuditLogRepoForBridge) GetAgentActionsByIPAddress(agentID uuid.UUID, ipAddress string, limit int) ([]*domain.AuditLog, error) {
	return nil, nil
}

// --- Tests ---

func TestGenerateContributorToken(t *testing.T) {
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	secret := "test-secret-value"

	token1 := generateContributorToken(orgID, secret)
	token2 := generateContributorToken(orgID, secret)

	// Deterministic: same input produces same token
	assert.Equal(t, token1, token2)
	assert.Len(t, token1, 64) // SHA-256 hex is 64 chars

	// Different org ID produces different token
	otherOrgID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	token3 := generateContributorToken(otherOrgID, secret)
	assert.NotEqual(t, token1, token3)

	// Different secret produces different token
	token4 := generateContributorToken(orgID, "other-secret")
	assert.NotEqual(t, token1, token4)
}

func TestAggregateAndPush_NoSecret(t *testing.T) {
	svc := NewRegistryBridgeService(
		&mockOrgRepoForBridge{},
		&mockAgentRepoForBridge{},
		&mockAttestationRepoForBridge{},
		&mockMCPServerRepoForBridge{},
		&mockAuditLogRepoForBridge{},
		"http://localhost:9999",
	)

	// Ensure env var is not set
	os.Unsetenv("REGISTRY_BRIDGE_SECRET")

	orgRepo := &mockOrgRepoForBridge{}
	orgRepo.On("ListAll").Return([]*domain.Organization{
		{ID: uuid.New(), IsActive: true},
	}, nil)
	svc.orgRepo = orgRepo

	err := svc.AggregateAndPush(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "REGISTRY_BRIDGE_SECRET")
}

func TestAggregateAndPush_NoOrgs(t *testing.T) {
	orgRepo := &mockOrgRepoForBridge{}
	orgRepo.On("ListAll").Return([]*domain.Organization{}, nil)

	svc := NewRegistryBridgeService(
		orgRepo,
		&mockAgentRepoForBridge{},
		&mockAttestationRepoForBridge{},
		&mockMCPServerRepoForBridge{},
		&mockAuditLogRepoForBridge{},
		"http://localhost:9999",
	)

	os.Setenv("REGISTRY_BRIDGE_SECRET", "test-secret")
	defer os.Unsetenv("REGISTRY_BRIDGE_SECRET")

	err := svc.AggregateAndPush(context.Background())
	assert.NoError(t, err)
}

func TestAggregateAndPush_FullFlow(t *testing.T) {
	orgID := uuid.New()
	agentID := uuid.New()
	mcpServerID := uuid.New()

	// Set up a test HTTP server to receive the contribution
	var receivedBatch domain.ContributionBatch
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/contribute", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&receivedBatch)
		assert.NoError(t, err)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// Set up mocks
	orgRepo := &mockOrgRepoForBridge{}
	orgRepo.On("ListAll").Return([]*domain.Organization{
		{ID: orgID, IsActive: true, Name: "Test Org"},
	}, nil)

	agentRepo := &mockAgentRepoForBridge{}
	agentRepo.On("GetByOrganization", orgID).Return([]*domain.Agent{
		{
			ID:                       agentID,
			OrganizationID:           orgID,
			Name:                     "test-agent",
			TrustScore:               0.85,
			CapabilityViolationCount: 2,
		},
	}, nil)

	attestationRepo := &mockAttestationRepoForBridge{}
	attestationRepo.On("GetAttestationsByAgent", agentID).Return([]*domain.MCPAttestation{
		{
			ID:                uuid.New(),
			MCPServerID:       mcpServerID,
			AgentID:           &agentID,
			SignatureVerified: true,
			IsValid:           true,
		},
		{
			ID:                uuid.New(),
			MCPServerID:       mcpServerID,
			AgentID:           &agentID,
			SignatureVerified: true,
			IsValid:           true,
		},
		{
			ID:                uuid.New(),
			MCPServerID:       mcpServerID,
			AgentID:           &agentID,
			SignatureVerified: false,
			IsValid:           false,
		},
	}, nil)

	mcpServerRepo := &mockMCPServerRepoForBridge{}
	mcpServerRepo.On("GetByID", mcpServerID).Return(&domain.MCPServer{
		ID:      mcpServerID,
		Name:    "mcp-server-github",
		Version: "1.2.0",
		URL:     "https://github.com/modelcontextprotocol/servers/tree/main/src/github",
	}, nil)

	auditLogRepo := &mockAuditLogRepoForBridge{}
	// orgID is the new first arg per the audit-log query IDOR fix
	// (todo/2026-05-21-a3d-viii-followup-audit-log-query-idor.md).
	auditLogRepo.On("GetByAgent", orgID, agentID, 1000, 0).Return([]*domain.AuditLog{
		{ID: uuid.New()},
		{ID: uuid.New()},
		{ID: uuid.New()},
	}, nil)

	os.Setenv("REGISTRY_BRIDGE_SECRET", "test-secret-for-bridge")
	defer os.Unsetenv("REGISTRY_BRIDGE_SECRET")

	svc := NewRegistryBridgeService(
		orgRepo,
		agentRepo,
		attestationRepo,
		mcpServerRepo,
		auditLogRepo,
		server.URL,
	)

	err := svc.AggregateAndPush(context.Background())
	assert.NoError(t, err)

	// Verify the batch was received
	assert.NotEmpty(t, receivedBatch.ContributorToken)
	assert.Len(t, receivedBatch.ContributorToken, 64)
	assert.NotEmpty(t, receivedBatch.SubmittedAt)
	assert.Len(t, receivedBatch.Events, 1)

	event := receivedBatch.Events[0]
	assert.Equal(t, "interaction", event.Type)
	assert.Equal(t, "aim-server", event.Tool)
	assert.Equal(t, "1.5.0", event.ToolVersion)

	// Verify package info (privacy: only MCP name, no org/agent names)
	assert.Equal(t, "mcp-server-github", event.Package.Name)
	assert.Equal(t, "1.2.0", event.Package.Version)

	// Verify behavior summary
	assert.NotNil(t, event.BehaviorSummary)
	assert.Equal(t, 3, event.BehaviorSummary.Interactions)
	assert.InDelta(t, 0.85, event.BehaviorSummary.SuccessRate, 0.001)
	assert.Equal(t, 2, event.BehaviorSummary.Anomalies)

	// Verify scan summary (3 attestations >= 2 threshold)
	assert.NotNil(t, event.ScanSummary)
	assert.Equal(t, 3, event.ScanSummary.TotalChecks)
	assert.Equal(t, 2, event.ScanSummary.Passed)
	assert.Equal(t, 66, event.ScanSummary.Score) // 2/3 * 100 = 66
	assert.Equal(t, "warn", event.ScanSummary.Verdict)

	// Verify privacy: no org name, no agent name in the batch
	batchJSON, _ := json.Marshal(receivedBatch)
	batchStr := string(batchJSON)
	assert.NotContains(t, batchStr, "Test Org")
	assert.NotContains(t, batchStr, "test-agent")
}

func TestBuildOrgContribution_NoAgents(t *testing.T) {
	orgID := uuid.New()

	agentRepo := &mockAgentRepoForBridge{}
	agentRepo.On("GetByOrganization", orgID).Return([]*domain.Agent{}, nil)

	svc := NewRegistryBridgeService(
		&mockOrgRepoForBridge{},
		agentRepo,
		&mockAttestationRepoForBridge{},
		&mockMCPServerRepoForBridge{},
		&mockAuditLogRepoForBridge{},
		"http://localhost:9999",
	)

	events, err := svc.buildOrgContribution(context.Background(), orgID)
	assert.NoError(t, err)
	assert.Nil(t, events)
}

func TestBuildOrgContribution_NoAttestations(t *testing.T) {
	orgID := uuid.New()
	agentID := uuid.New()

	agentRepo := &mockAgentRepoForBridge{}
	agentRepo.On("GetByOrganization", orgID).Return([]*domain.Agent{
		{ID: agentID, OrganizationID: orgID},
	}, nil)

	attestationRepo := &mockAttestationRepoForBridge{}
	attestationRepo.On("GetAttestationsByAgent", agentID).Return([]*domain.MCPAttestation{}, nil)

	svc := NewRegistryBridgeService(
		&mockOrgRepoForBridge{},
		agentRepo,
		attestationRepo,
		&mockMCPServerRepoForBridge{},
		&mockAuditLogRepoForBridge{},
		"http://localhost:9999",
	)

	events, err := svc.buildOrgContribution(context.Background(), orgID)
	assert.NoError(t, err)
	assert.NotNil(t, events, "must be non-nil empty slice (jsonarray-lint invariant)")
	assert.Equal(t, 0, len(events))
}

func TestInferEcosystem(t *testing.T) {
	assert.Equal(t, "npm", inferEcosystem("https://example.com/mcp-server"))
	assert.Equal(t, "", inferEcosystem(""))
}

// GetByOrganizationPaged pages over GetByOrganization for tests.
func (m *mockAgentRepoForBridge) GetByOrganizationPaged(orgID uuid.UUID, limit, offset int) ([]*domain.Agent, int, error) {
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
