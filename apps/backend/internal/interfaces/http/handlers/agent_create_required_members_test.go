package handlers

// POST /api/v1/agents through the server's own handler and agent service. A
// request that leaves name, displayName or agentType empty, or names an agent
// type the service does not register, is refused without reaching the agents
// table. The answer is 400 with a line that names the members by the JSON
// names CreateAgentRequest binds.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// registeringAgentRepo stands in for the agents table and keeps each agent it is
// asked to insert. No other repository method is set, so a request that
// reaches one fails its test.
type registeringAgentRepo struct {
	domain.AgentRepository
	inserted []*domain.Agent
}

func (r *registeringAgentRepo) Create(agent *domain.Agent) error {
	r.inserted = append(r.inserted, agent)
	return nil
}

// failingTrustCalculator fails every calculation. The service registers an
// agent whose trust score it cannot calculate without one, so a registration
// through it reads and writes no trust score.
type failingTrustCalculator struct{ domain.TrustScoreCalculator }

func (failingTrustCalculator) Calculate(*domain.Agent) (*domain.TrustScore, error) {
	return nil, errors.New("no trust score is calculated in this test")
}

// postCreateAgent posts body to CreateAgent as a signed-in user, with the server's
// agent service over repo, and returns the status and the decoded answer.
func postCreateAgent(t *testing.T, repo *registeringAgentRepo, body string) (int, map[string]any) {
	t.Helper()
	vault, err := crypto.NewKeyVault(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
	require.NoError(t, err)

	agentService := application.NewAgentService(repo, failingTrustCalculator{}, nil, vault, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewAgentHandlerWithInterfaces(
		agentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)
	app := createTestAppWithAuth(handler, uuid.New(), uuid.New())
	app.Post("/api/v1/agents", handler.CreateAgent)

	req := httptest.NewRequest("POST", "/api/v1/agents", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var answer map[string]any
	require.NoError(t, json.Unmarshal(raw, &answer), "the answer is JSON: %s", raw)
	return resp.StatusCode, answer
}

// The bodies of the report, and one without a name. Each was answered 500
// with the server line, which names nothing to change.
func TestCreateAgent_RefusalNamesTheRequiredMembersTheRequestLeftEmpty(t *testing.T) {
	name := jsonMember(t, application.CreateAgentRequest{}, "Name")
	displayName := jsonMember(t, application.CreateAgentRequest{}, "DisplayName")
	agentType := jsonMember(t, application.CreateAgentRequest{}, "AgentType")

	cases := []struct {
		name string
		body string
		want string
	}{
		{
			// The example the dashboard's API reference gave: `type` is not a
			// member the request binds, and `displayName` is missing.
			name: "the API reference's former example",
			body: `{"name":"data-processor","type":"automation_bot","description":"Automated data processing agent","tags":["production","critical"]}`,
			want: displayName + " and " + agentType + " are required",
		},
		{name: "no displayName", body: `{"name":"no-display","agentType":"ai_agent"}`, want: displayName + " is required"},
		{name: "no agentType", body: `{"name":"no-type","displayName":"No Type"}`, want: agentType + " is required"},
		{name: "no name", body: `{"displayName":"No Name","agentType":"ai_agent"}`, want: name + " is required"},
		{name: "an empty object", body: `{}`, want: name + ", " + displayName + " and " + agentType + " are required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &registeringAgentRepo{}
			status, answer := postCreateAgent(t, repo, tc.body)

			assert.Equal(t, fiber.StatusBadRequest, status, "answer: %v", answer)
			assert.Equal(t, map[string]any{"error": tc.want}, answer)
			assert.Empty(t, repo.inserted, "a refused request must not reach the agents table")
		})
	}
}

// An agentType the service does not register, such as the former example's
// type, is answered with the types it does, and the value sent is not echoed.
func TestCreateAgent_RefusalListsTheAgentTypesTheServiceRegisters(t *testing.T) {
	repo := &registeringAgentRepo{}
	status, answer := postCreateAgent(t, repo, `{"name":"data-processor","displayName":"Data Processor","agentType":"automation_bot"}`)

	assert.Equal(t, fiber.StatusBadRequest, status, "answer: %v", answer)
	assert.Equal(t, map[string]any{"error": "agentType must be one of: ai_agent, assistant, autogen, autogpt, babyagi, " +
		"chatbot, claude, cohere, copilot, crewai, custom, demo, gemini, gpt, haystack, langchain, langgraph, llama, " +
		"llamaindex, mistral, semantic_kernel"}, answer)
	assert.Empty(t, repo.inserted, "a refused request must not reach the agents table")
}

// The snake_case spellings the handler also reads still register an agent.
func TestCreateAgent_SnakeCaseMembersRegisterAnAgent(t *testing.T) {
	repo := &registeringAgentRepo{}
	status, answer := postCreateAgent(t, repo, `{"name":"snake-case","display_name":"Snake Case","agent_type":"ai_agent"}`)

	require.Equal(t, fiber.StatusCreated, status, "answer: %v", answer)
	require.Len(t, repo.inserted, 1)
	assert.Equal(t, "Snake Case", repo.inserted[0].DisplayName)
	assert.Equal(t, domain.AgentTypeAI, repo.inserted[0].AgentType)
	assert.Equal(t, "Snake Case", answer["displayName"])
}

// The dashboard's developers page renders apps/web/lib/api-documentation.ts and
// offers its example bodies to copy. Its POST /api/v1/agents entry documented
// `type` and `tags`, which CreateAgentRequest does not bind, and left out
// displayName and agentType, so its example was refused.
func TestDashboardAPIReference_CreateAgentMatchesTheHandler(t *testing.T) {
	const apiPath = "/api/v1/agents"
	entry := dashboardEndpointEntry(t, readBackendFile(t, "../web/lib/api-documentation.ts"), "POST", apiPath)
	documented := documentedRequestIn(t, entry, "POST "+apiPath)
	bound, _ := boundFields(application.CreateAgentRequest{})

	t.Run("every documented member is one the handler binds", func(t *testing.T) {
		assert.Subset(t, bound, documented.properties)
		assert.Subset(t, documented.properties, documented.exampleKeys, "the example sends only documented members")
	})

	t.Run("the members marked required are the ones the service refuses a request without", func(t *testing.T) {
		_, err := (&application.AgentService{}).CreateAgent(context.Background(), &application.CreateAgentRequest{},
			uuid.New(), uuid.New(), nil, nil, "")
		var refusal *application.AgentRequestError
		require.ErrorAs(t, err, &refusal)
		required := append([]string(nil), refusal.Missing...)
		sort.Strings(required)

		assert.Equal(t, required, documented.required)
		assert.Subset(t, documented.exampleKeys, required, "the example sends every required member")
	})

	t.Run("the example registers an agent with the members it sends", func(t *testing.T) {
		repo := &registeringAgentRepo{}
		status, answer := postCreateAgent(t, repo, documented.example)
		require.Equal(t, fiber.StatusCreated, status, "the example is refused: %v", answer["error"])
		require.Len(t, repo.inserted, 1)

		var example map[string]any
		require.NoError(t, json.Unmarshal([]byte(documented.example), &example))
		agent := repo.inserted[0]
		assert.Equal(t, example["name"], agent.Name)
		assert.Equal(t, example["displayName"], agent.DisplayName)
		assert.Equal(t, example["agentType"], string(agent.AgentType))
		assert.Equal(t, example["description"], agent.Description)
	})
}
