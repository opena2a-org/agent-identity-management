package handlers

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The TypeScript and Java SDKs evaluate a policy as the request pinned in
// sdk/typescript/tests/fixtures/sdk_requests/a2a_evaluate_policy.json;
// evaluatePolicy.wire.test.ts and A2AEvaluatePolicyWireTest.java hold the
// clients to it. The test below holds the server to it: POST
// /a2a/policies/evaluate binds every member and evaluates the policy that names
// the request's client agent and skill. The clients sent targetAgentId, action,
// resource and context, none of which the route reads, so every evaluation was
// of the zero client agent with no skill (#652).

func TestSDKEvaluatePolicyRequestIsReadByEvaluatePolicy(t *testing.T) {
	fx := readTSRequestFixture(t, "a2a_evaluate_policy")
	require.Equal(t, fiber.MethodPost, fx.Method)
	require.Equal(t, "/api/v1/a2a/policies/evaluate", fx.Path)

	var body struct {
		Event         string    `json:"event"`
		ClientAgentID uuid.UUID `json:"clientAgentId"`
		RemoteAgentID uuid.UUID `json:"remoteAgentId"`
		SkillID       string    `json:"skillId"`
	}
	decodeTSBody(t, fx, &body)
	require.NotEmpty(t, body.Event)
	require.NotEmpty(t, body.SkillID)

	callerOrg := uuid.New()
	policyDB, policyMock := newSQLMock(t)
	agentDB, agentMock := newSQLMock(t)
	trustDB, trustMock := newSQLMock(t)

	// One active policy, for the request's client agent and skill only. Neither
	// agent has a trust score, so the policy denies when, and only when, the
	// server read the client agent and skill from the body.
	now := time.Now()
	policyMock.ExpectQuery(activeA2APoliciesQuery).
		WithArgs(callerOrg).
		WillReturnRows(sqlmock.NewRows(a2aPolicyColumns).AddRow(
			uuid.New(), callerOrg.String(), "skill-policy", "", "",
			[]byte(`["`+body.ClientAgentID.String()+`"]`), []byte(`["`+body.SkillID+`"]`), true, 1, nil, nil, now, now))
	expectOrganizationAgents(agentMock, callerOrg, []uuid.UUID{body.ClientAgentID, body.RemoteAgentID},
		body.ClientAgentID, body.RemoteAgentID)
	trustMock.ExpectQuery(`FROM a2a_trust_scores`).WillReturnError(errors.New("no trust score"))
	trustMock.ExpectQuery(`FROM a2a_trust_scores`).WillReturnError(errors.New("no trust score"))

	status, raw := postEvaluatePolicy(t, policyDB, agentDB, trustDB, callerOrg, string(fx.Body))

	require.Equal(t, fiber.StatusOK, status, "answer: %s", raw)
	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "DENY", got["decision"], "the policy for the client agent and skill applies")
	assert.Equal(t, []any{"skill-policy"}, got["policiesEvaluated"])
	assert.NoError(t, agentMock.ExpectationsWereMet(), "the body's client and remote agents are read")
}
