package handlers

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// POST /a2a/policies/evaluate evaluates the active policies of the caller's own
// organization, the one its credentials carry. The handler passed the body's
// organizationId to the service as given, so a body naming another organization
// was answered with that organization's decision and the names of its active
// policies, and a body naming none read the policies of the zero UUID (#652).

const activeA2APoliciesQuery = `FROM a2a_policies\s+WHERE organization_id = \$1 AND is_active = TRUE`

var a2aPolicyColumns = []string{"id", "organization_id", "name", "description", "policy_yaml",
	"applies_to_agents", "applies_to_skills", "is_active", "priority",
	"created_by", "updated_by", "created_at", "updated_at"}

// postEvaluatePolicy posts body to EvaluatePolicy on a service that reads
// policies from policyDB, agents from agentDB and trust scores from trustDB, as
// a caller of callerOrg, or as a caller with no organization when callerOrg is
// uuid.Nil.
func postEvaluatePolicy(t *testing.T, policyDB, agentDB, trustDB *sql.DB, callerOrg uuid.UUID, body string) (int, []byte) {
	t.Helper()
	svc := application.NewA2AService(
		nil, nil, nil, nil, nil,
		repository.NewA2ATrustScoreRepository(trustDB),
		nil,
		repository.NewA2APolicyRepository(policyDB),
		nil, nil, nil, nil,
		repository.NewAgentRepository(agentDB),
		nil, nil, nil,
	)
	h := NewA2AHandler(svc, nil, nil)
	app := fiber.New()
	app.Post("/a2a/policies/evaluate", func(c fiber.Ctx) error {
		if callerOrg != uuid.Nil {
			c.Locals("organization_id", callerOrg)
		}
		return h.EvaluatePolicy(c)
	})
	req := httptest.NewRequest("POST", "/a2a/policies/evaluate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, raw
}

func evaluatePolicyBody(clientAgent, remoteAgent uuid.UUID, organizationID string) string {
	body := `{"event":"task.create","clientAgentId":"` + clientAgent.String() +
		`","remoteAgentId":"` + remoteAgent.String() + `"`
	if organizationID != "" {
		body += `,"organizationId":"` + organizationID + `"`
	}
	return body + `}`
}

func newSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

func TestA2AEvaluatePolicyReadsThePoliciesOfTheCallersOrganization(t *testing.T) {
	callerOrg := uuid.New()
	clientAgent := uuid.New()
	remoteAgent := uuid.New()

	for name, bodyOrg := range map[string]string{
		"the body names no organization":           "",
		"the body names the caller's organization": callerOrg.String(),
	} {
		t.Run(name, func(t *testing.T) {
			policyDB, policyMock := newSQLMock(t)
			agentDB, agentMock := newSQLMock(t)
			trustDB, trustMock := newSQLMock(t)

			// One active policy of the caller's organization. It applies only to
			// an agent the request does not name, so the decision is ALLOW and the
			// policy is listed as evaluated.
			now := time.Now()
			policyMock.ExpectQuery(activeA2APoliciesQuery).
				WithArgs(callerOrg).
				WillReturnRows(sqlmock.NewRows(a2aPolicyColumns).AddRow(
					uuid.New(), callerOrg.String(), "caller-policy", "", "",
					[]byte(`["`+uuid.New().String()+`"]`), []byte(`[]`), true, 1, nil, nil, now, now))
			// The service reads both agents' trust scores before it walks the
			// policies, both agents being of the caller's organization.
			expectOrganizationAgents(agentMock, callerOrg, []uuid.UUID{clientAgent, remoteAgent}, clientAgent, remoteAgent)
			trustMock.ExpectQuery(`FROM a2a_trust_scores`).WillReturnError(errors.New("no trust score"))
			trustMock.ExpectQuery(`FROM a2a_trust_scores`).WillReturnError(errors.New("no trust score"))

			status, raw := postEvaluatePolicy(t, policyDB, agentDB, trustDB, callerOrg,
				evaluatePolicyBody(clientAgent, remoteAgent, bodyOrg))

			require.Equal(t, fiber.StatusOK, status, "body: %s", raw)
			var got map[string]any
			require.NoError(t, json.Unmarshal(raw, &got))
			assert.Equal(t, "ALLOW", got["decision"])
			assert.Equal(t, []any{"caller-policy"}, got["policiesEvaluated"])
			assert.NoError(t, policyMock.ExpectationsWereMet(), "the caller's organization's policies are read")
		})
	}
}

func TestA2AEvaluatePolicyRefusesABodyThatNamesAnotherOrganization(t *testing.T) {
	callerOrg := uuid.New()
	otherOrg := uuid.New()
	policyDB, policyMock := newSQLMock(t)
	agentDB, agentMock := newSQLMock(t)
	trustDB, trustMock := newSQLMock(t)

	status, raw := postEvaluatePolicy(t, policyDB, agentDB, trustDB, callerOrg,
		evaluatePolicyBody(uuid.New(), uuid.New(), otherOrg.String()))

	// No query is expected: sqlmock fails any statement it was not told to
	// expect, which the service would answer with 500.
	require.Equal(t, fiber.StatusForbidden, status, "body: %s", raw)
	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, map[string]any{"error": evaluatePolicyOtherOrganizationMessage}, got)
	assert.NotContains(t, string(raw), otherOrg.String(), "the answer does not echo the organization")
	assert.NoError(t, policyMock.ExpectationsWereMet())
	assert.NoError(t, agentMock.ExpectationsWereMet())
	assert.NoError(t, trustMock.ExpectationsWereMet())
}

func TestA2AEvaluatePolicyWithoutAnOrganizationInTheCredentialsIsRefused(t *testing.T) {
	policyDB, policyMock := newSQLMock(t)
	agentDB, agentMock := newSQLMock(t)
	trustDB, trustMock := newSQLMock(t)

	status, raw := postEvaluatePolicy(t, policyDB, agentDB, trustDB, uuid.Nil,
		evaluatePolicyBody(uuid.New(), uuid.New(), uuid.New().String()))

	assert.Equal(t, fiber.StatusUnauthorized, status, "body: %s", raw)
	assert.NoError(t, policyMock.ExpectationsWereMet())
	assert.NoError(t, agentMock.ExpectationsWereMet())
	assert.NoError(t, trustMock.ExpectationsWereMet())
}

// The decision reads the A2A trust scores of the body's clientAgentId and
// remoteAgentId only for agents of the caller's organization. Another
// organization's agent is answered as an agent with no score, as an id with no
// agent is, so the decision and its reason say nothing about it.

// agentsInOrganizationQuery is AgentRepository.GetByIDs: agents of one
// organization among the ids given.
const agentsInOrganizationQuery = `FROM agents WHERE id IN \(\$1,\$2\) AND organization_id = \$3`

var organizationAgentColumns = []string{"id", "organization_id", "name", "display_name", "status",
	"agent_type", "trust_score", "last_heartbeat", "last_active", "created_at", "updated_at"}

// expectOrganizationAgents expects the read of ids among the agents of org and
// answers it with the agents of found.
func expectOrganizationAgents(mock sqlmock.Sqlmock, org uuid.UUID, ids []uuid.UUID, found ...uuid.UUID) {
	rows := sqlmock.NewRows(organizationAgentColumns)
	now := time.Now()
	for _, id := range found {
		rows.AddRow(id.String(), org.String(), "agent", "Agent", "verified", "ai_agent", 0.9, nil, nil, now, now)
	}
	args := make([]driver.Value, 0, len(ids)+1)
	for _, id := range ids {
		args = append(args, id)
	}
	mock.ExpectQuery(agentsInOrganizationQuery).WithArgs(append(args, org)...).WillReturnRows(rows)
}

var a2aTrustScoreColumns = []string{"agent_id", "total_tasks_as_client", "total_tasks_as_remote",
	"tasks_completed", "tasks_failed", "tasks_cancelled",
	"avg_response_time_ms", "p95_response_time_ms", "avg_task_duration_ms",
	"a2a_trust_score", "peer_trust_average", "unique_peers_count",
	"positive_feedback_count", "negative_feedback_count",
	"computed_at", "data_points", "created_at", "updated_at"}

// expectA2ATrustScore expects the read of agent's A2A trust score and answers score.
func expectA2ATrustScore(mock sqlmock.Sqlmock, agent uuid.UUID, score float64) {
	now := time.Now()
	mock.ExpectQuery(`FROM a2a_trust_scores\s+WHERE agent_id = \$1`).WithArgs(agent).
		WillReturnRows(sqlmock.NewRows(a2aTrustScoreColumns).AddRow(
			agent.String(), 10, 10, 20, 0, 0, nil, nil, nil, score, nil, 2, 0, 0, now, 20, now, now))
}

// expectPolicyForEveryAgent expects the read of org's active policies and
// answers one policy that applies to every agent and skill.
func expectPolicyForEveryAgent(mock sqlmock.Sqlmock, org uuid.UUID) {
	now := time.Now()
	mock.ExpectQuery(activeA2APoliciesQuery).WithArgs(org).
		WillReturnRows(sqlmock.NewRows(a2aPolicyColumns).AddRow(
			uuid.New(), org.String(), "every-agent", "", "",
			[]byte(`[]`), []byte(`[]`), true, 1, nil, nil, now, now))
}

func evaluatePolicyDecision(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	return got
}

func TestA2AEvaluatePolicyReadsNoTrustScoreOfAnotherOrganizationsAgents(t *testing.T) {
	t.Setenv("A2A_MIN_TRUST_SCORE", "")
	callerOrg := uuid.New()
	clientAgent, remoteAgent := uuid.New(), uuid.New()

	policyDB, policyMock := newSQLMock(t)
	agentDB, agentMock := newSQLMock(t)
	trustDB, trustMock := newSQLMock(t)
	expectPolicyForEveryAgent(policyMock, callerOrg)
	// Neither agent is of the caller's organization.
	expectOrganizationAgents(agentMock, callerOrg, []uuid.UUID{clientAgent, remoteAgent})
	// Both have scores above the threshold, which the decision must not read.
	expectA2ATrustScore(trustMock, clientAgent, 0.93)
	expectA2ATrustScore(trustMock, remoteAgent, 0.87)

	status, raw := postEvaluatePolicy(t, policyDB, agentDB, trustDB, callerOrg,
		evaluatePolicyBody(clientAgent, remoteAgent, ""))

	require.Equal(t, fiber.StatusOK, status, "body: %s", raw)
	got := evaluatePolicyDecision(t, raw)
	assert.Equal(t, "DENY", got["decision"])
	assert.Equal(t, "Trust score below threshold (client: 0.00, remote: 0.00, required: 0.70)", got["reason"])
	assert.NotContains(t, string(raw), "0.93")
	assert.NotContains(t, string(raw), "0.87")
	assert.NoError(t, agentMock.ExpectationsWereMet(), "the agents are looked up in the caller's organization")
	assert.Error(t, trustMock.ExpectationsWereMet(), "no trust score of another organization's agent is read")
}

func TestA2AEvaluatePolicyReadsTheTrustScoresOfTheCallersOwnAgents(t *testing.T) {
	t.Setenv("A2A_MIN_TRUST_SCORE", "")
	callerOrg := uuid.New()
	clientAgent, remoteAgent := uuid.New(), uuid.New()

	t.Run("both agents of the caller's organization", func(t *testing.T) {
		policyDB, policyMock := newSQLMock(t)
		agentDB, agentMock := newSQLMock(t)
		trustDB, trustMock := newSQLMock(t)
		expectPolicyForEveryAgent(policyMock, callerOrg)
		expectOrganizationAgents(agentMock, callerOrg, []uuid.UUID{clientAgent, remoteAgent}, clientAgent, remoteAgent)
		expectA2ATrustScore(trustMock, clientAgent, 0.93)
		expectA2ATrustScore(trustMock, remoteAgent, 0.87)

		status, raw := postEvaluatePolicy(t, policyDB, agentDB, trustDB, callerOrg,
			evaluatePolicyBody(clientAgent, remoteAgent, ""))

		require.Equal(t, fiber.StatusOK, status, "body: %s", raw)
		assert.Equal(t, "ALLOW", evaluatePolicyDecision(t, raw)["decision"])
		assert.NoError(t, trustMock.ExpectationsWereMet())
	})

	t.Run("the remote agent of another organization", func(t *testing.T) {
		policyDB, policyMock := newSQLMock(t)
		agentDB, agentMock := newSQLMock(t)
		trustDB, trustMock := newSQLMock(t)
		expectPolicyForEveryAgent(policyMock, callerOrg)
		expectOrganizationAgents(agentMock, callerOrg, []uuid.UUID{clientAgent, remoteAgent}, clientAgent)
		expectA2ATrustScore(trustMock, clientAgent, 0.93)

		status, raw := postEvaluatePolicy(t, policyDB, agentDB, trustDB, callerOrg,
			evaluatePolicyBody(clientAgent, remoteAgent, ""))

		require.Equal(t, fiber.StatusOK, status, "body: %s", raw)
		got := evaluatePolicyDecision(t, raw)
		assert.Equal(t, "DENY", got["decision"])
		assert.Equal(t, "Trust score below threshold (client: 0.93, remote: 0.00, required: 0.70)", got["reason"])
		assert.NoError(t, trustMock.ExpectationsWereMet(), "only the client's score is read")
	})
}

// A failed read of the agents is not answered as a decision.
func TestA2AEvaluatePolicyWhenTheAgentsCannotBeReadAnswers500(t *testing.T) {
	callerOrg := uuid.New()
	policyDB, policyMock := newSQLMock(t)
	agentDB, agentMock := newSQLMock(t)
	trustDB, trustMock := newSQLMock(t)
	expectPolicyForEveryAgent(policyMock, callerOrg)
	agentMock.ExpectQuery(agentsInOrganizationQuery).WillReturnError(errors.New("pq: connection reset by peer"))

	status, raw := postEvaluatePolicy(t, policyDB, agentDB, trustDB, callerOrg,
		evaluatePolicyBody(uuid.New(), uuid.New(), ""))

	assert.Equal(t, fiber.StatusInternalServerError, status, "body: %s", raw)
	assert.NoError(t, agentMock.ExpectationsWereMet())
	assert.NoError(t, trustMock.ExpectationsWereMet(), "no trust score is read")
}
