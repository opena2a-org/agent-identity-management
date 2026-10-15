package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/transition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// An agent's talks_to list holds the ID or the name of each MCP server, and
// GET /agents/{id}/mcp-servers lists a server once whichever way it was added.
// AddMCPServers compared the identifiers as strings, so adding one server by
// its ID and then by its name left two entries: total_count said 2 while the
// list showed one server.

func newAddMCPServersService(t *testing.T, agent *domain.Agent, servers []*domain.MCPServer) (*AgentService, *MockAgentRepository) {
	t.Helper()
	agentRepo := new(MockAgentRepository)
	trustCalc := new(AgentServiceMockTrustScoreCalculator)
	trustScoreRepo := new(AgentServiceMockTrustScoreRepository)
	mcpRepo := new(AgentServiceMockMCPServerRepository)

	agentRepo.On("GetByID", agent.ID).Return(agent, nil)
	agentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil).Maybe()
	score := &domain.TrustScore{ID: uuid.New(), AgentID: agent.ID, Score: 0.8}
	trustCalc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(score, nil).Maybe()
	trustScoreRepo.On("Create", score).Return(nil).Maybe()
	mcpRepo.On("GetByOrganization", agent.OrganizationID).Return(servers, nil)

	service := &AgentService{
		agentRepo:      agentRepo,
		trustCalc:      trustCalc,
		trustScoreRepo: trustScoreRepo,
	}
	service.SetMCPServerLookup(mcpRepo)
	return service, agentRepo
}

func TestAgentService_AddMCPServers_OneServerByIDThenByNameIsOneEntry(t *testing.T) {
	agent := createTestAgentForService()
	server := &domain.MCPServer{ID: uuid.New(), OrganizationID: agent.OrganizationID, Name: "filesystem"}
	agent.TalksTo = []string{}
	service, agentRepo := newAddMCPServersService(t, agent, []*domain.MCPServer{server})
	ctx := context.Background()

	_, added, err := service.AddMCPServers(ctx, agent.ID, []string{server.ID.String()})
	require.NoError(t, err)
	assert.Equal(t, []string{server.ID.String()}, added)

	updated, added, err := service.AddMCPServers(ctx, agent.ID, []string{"filesystem"})
	require.NoError(t, err)
	assert.Empty(t, added, "the server is already on the list under its ID")
	assert.Equal(t, []string{server.ID.String()}, updated.TalksTo)
	agentRepo.AssertNumberOfCalls(t, "GetByID", 2)
}

func TestAgentService_AddMCPServers_OneServerByNameThenByIDIsOneEntry(t *testing.T) {
	agent := createTestAgentForService()
	server := &domain.MCPServer{ID: uuid.New(), OrganizationID: agent.OrganizationID, Name: "filesystem"}
	agent.TalksTo = []string{"filesystem"}
	service, _ := newAddMCPServersService(t, agent, []*domain.MCPServer{server})

	// Any spelling of the ID the list reads (uuid.Parse) names the same server.
	for _, spelling := range []string{
		server.ID.String(),
		strings.ToUpper(server.ID.String()),
		"urn:uuid:" + server.ID.String(),
	} {
		updated, added, err := service.AddMCPServers(context.Background(), agent.ID, []string{spelling})
		require.NoError(t, err)
		assert.Empty(t, added, spelling)
		assert.Equal(t, []string{"filesystem"}, updated.TalksTo, spelling)
	}
}

func TestAgentService_AddMCPServers_OneServerTwiceInOneRequestIsOneEntry(t *testing.T) {
	agent := createTestAgentForService()
	server := &domain.MCPServer{ID: uuid.New(), OrganizationID: agent.OrganizationID, Name: "filesystem"}
	agent.TalksTo = []string{}
	service, _ := newAddMCPServersService(t, agent, []*domain.MCPServer{server})

	updated, added, err := service.AddMCPServers(context.Background(), agent.ID, []string{"filesystem", server.ID.String()})
	require.NoError(t, err)
	assert.Equal(t, []string{"filesystem"}, added)
	assert.Equal(t, []string{"filesystem"}, updated.TalksTo)
}

func TestAgentService_AddMCPServers_IdentifiersOfNoServerAreComparedAsGiven(t *testing.T) {
	agent := createTestAgentForService()
	other := &domain.MCPServer{ID: uuid.New(), OrganizationID: agent.OrganizationID, Name: "github"}
	agent.TalksTo = []string{"not-registered-yet"}
	service, _ := newAddMCPServersService(t, agent, []*domain.MCPServer{other})
	unknownID := uuid.New().String()

	updated, added, err := service.AddMCPServers(context.Background(), agent.ID,
		[]string{"not-registered-yet", "filesystem", unknownID, "github"})
	require.NoError(t, err)
	assert.Equal(t, []string{"filesystem", unknownID, "github"}, added)
	assert.Equal(t, []string{"not-registered-yet", "filesystem", unknownID, "github"}, updated.TalksTo)
}

func TestAgentService_AddMCPServers_WithATransitionRecorderOneServerIsOneEntry(t *testing.T) {
	// With a recorder set, AddMCPServers writes through one talks_to_added
	// transition. A server already on the list under its ID or its name is no
	// change there either, so nothing is recorded and nothing is written; the
	// zero recorder below is never reached.
	agent := createTestAgentForService()
	server := &domain.MCPServer{ID: uuid.New(), OrganizationID: agent.OrganizationID, Name: "filesystem"}

	for _, tc := range []struct{ held, given string }{
		{server.ID.String(), "filesystem"},
		{"filesystem", server.ID.String()},
		{"filesystem", "urn:uuid:" + server.ID.String()},
	} {
		agent.TalksTo = []string{tc.held}
		service, agentRepo := newAddMCPServersService(t, agent, []*domain.MCPServer{server})
		service.transitions = &transition.Recorder{}

		updated, added, err := service.AddMCPServers(context.Background(), agent.ID, []string{tc.given})
		require.NoError(t, err, tc.given)
		assert.Empty(t, added, tc.given)
		assert.Equal(t, []string{tc.held}, updated.TalksTo, tc.given)
		agentRepo.AssertNotCalled(t, "Update", mock.Anything)
	}
}

func TestAgentService_AddMCPServers_AFailedServerLookupComparesAsGiven(t *testing.T) {
	agent := createTestAgentForService()
	serverID := uuid.New().String()
	agent.TalksTo = []string{serverID}

	agentRepo := new(MockAgentRepository)
	mcpRepo := new(AgentServiceMockMCPServerRepository)
	agentRepo.On("GetByID", agent.ID).Return(agent, nil)
	mcpRepo.On("GetByOrganization", agent.OrganizationID).Return(nil, errors.New("database unavailable"))
	service := &AgentService{agentRepo: agentRepo}
	service.SetMCPServerLookup(mcpRepo)

	updated, added, err := service.AddMCPServers(context.Background(), agent.ID, []string{serverID})
	require.NoError(t, err)
	assert.Empty(t, added)
	assert.Equal(t, []string{serverID}, updated.TalksTo)
}
