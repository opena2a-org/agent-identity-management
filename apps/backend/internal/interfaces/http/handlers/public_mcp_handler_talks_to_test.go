package handlers

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// publicMCPTestAgentRepo is a domain.AgentRepository stub that only
// implements GetByID, the single method VerifyMCPAction reaches through
// AgentService.GetAgent. Any other method panics via the nil embed.
type publicMCPTestAgentRepo struct {
	domain.AgentRepository
	agent *domain.Agent
}

func (r *publicMCPTestAgentRepo) GetByID(id uuid.UUID) (*domain.Agent, error) {
	return r.agent, nil
}

// mcpServerRowColumns mirrors the SELECT list of MCPServerRepository.GetByID.
var mcpServerRowColumns = []string{
	"id", "organization_id", "name", "description", "url", "version",
	"public_key", "status", "is_verified", "last_verified_at", "verification_url",
	"capabilities", "trust_score", "registered_by_agent", "created_by", "created_at", "updated_at",
	"verification_method", "attestation_count", "confidence_score", "last_attested_at",
	"created_by_name", "created_by_email", "created_by_sdk_token_id", "created_by_api_key_id",
	"updated_by", "updated_by_name", "updated_by_email",
}

// verifyMCPActionAs signs a VerifyMCPAction request for an agent with the
// given talks_to list against a server in the agent's own organization,
// sends it through the real handler, and returns the status code and body.
func verifyMCPActionAs(t *testing.T, talksTo []string, serverName string) (int, map[string]interface{}) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	pubB64 := base64.StdEncoding.EncodeToString(pub)

	orgID := uuid.New()
	agentID := uuid.New()
	serverID := uuid.New()

	agent := &domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		PublicKey:      &pubB64,
		TalksTo:        talksTo,
	}

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	now := time.Now()
	mock.ExpectQuery("FROM mcp_servers").
		WithArgs(serverID).
		WillReturnRows(sqlmock.NewRows(mcpServerRowColumns).AddRow(
			serverID.String(), orgID.String(), serverName, nil, "https://mcp.example.com", nil,
			nil, string(domain.MCPServerStatusVerified), true, nil, nil,
			[]byte("[]"), 50.0, nil, uuid.New().String(), now, now,
			"manual", 0, 0.0, nil,
			"", "", nil, nil,
			nil, "", "",
		))

	mcpService := application.NewMCPService(repository.NewMCPServerRepository(db), nil, nil, nil, nil, nil, nil, nil, nil, nil)
	agentService := application.NewAgentService(&publicMCPTestAgentRepo{agent: agent}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewPublicMCPHandler(mcpService, agentService, nil)

	app := fiber.New()
	app.Post("/public/mcp-servers/:id/verify", handler.VerifyMCPAction)

	timestamp := time.Now().Unix()
	message := "verify_mcp_action:" + agentID.String() + ":" + serverID.String() + ":tool_call:" + strconv.FormatInt(timestamp, 10)
	signature := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, []byte(message)))

	body, err := json.Marshal(map[string]interface{}{
		"agentId":    agentID.String(),
		"actionType": "tool_call",
		"timestamp":  timestamp,
		"signature":  signature,
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/public/mcp-servers/"+serverID.String()+"/verify", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var out map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.NoError(t, mock.ExpectationsWereMet())

	return resp.StatusCode, out
}

// An agent's talks_to list is an allowlist: an empty or absent list names
// no MCP servers, so the agent is allowed none of them.
func TestPublicMCPHandler_VerifyMCPAction_EmptyTalksToAllowsNoServer(t *testing.T) {
	cases := []struct {
		name    string
		talksTo []string
	}{
		{name: "nil list", talksTo: nil},
		{name: "empty list", talksTo: []string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := verifyMCPActionAs(t, tc.talksTo, "filesystem")

			assert.Equal(t, fiber.StatusForbidden, status,
				"an agent with no talks_to entries must not be approved for any MCP server")
			assert.Equal(t, "unauthorized_mcp_access", body["violation_type"])
			assert.NotEqual(t, "approved", body["status"])
		})
	}
}

func TestPublicMCPHandler_VerifyMCPAction_TalksToMatching(t *testing.T) {
	t.Run("listed by name is approved", func(t *testing.T) {
		status, body := verifyMCPActionAs(t, []string{"FileSystem"}, "filesystem")

		assert.Equal(t, fiber.StatusOK, status)
		assert.Equal(t, "approved", body["status"])
	})

	t.Run("unlisted server is refused", func(t *testing.T) {
		status, body := verifyMCPActionAs(t, []string{"github"}, "filesystem")

		assert.Equal(t, fiber.StatusForbidden, status)
		assert.Equal(t, "unauthorized_mcp_access", body["violation_type"])
	})
}
