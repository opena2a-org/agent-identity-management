package handlers

import (
	"database/sql"
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
// policies from policyDB and trust scores from trustDB, as a caller of
// callerOrg, or as a caller with no organization when callerOrg is uuid.Nil.
func postEvaluatePolicy(t *testing.T, policyDB, trustDB *sql.DB, callerOrg uuid.UUID, body string) (int, []byte) {
	t.Helper()
	svc := application.NewA2AService(
		nil, nil, nil, nil, nil,
		repository.NewA2ATrustScoreRepository(trustDB),
		nil,
		repository.NewA2APolicyRepository(policyDB),
		nil, nil, nil, nil, nil, nil, nil, nil,
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
			// The service reads both agents' trust scores before it walks the policies.
			trustMock.ExpectQuery(`FROM a2a_trust_scores`).WillReturnError(errors.New("no trust score"))
			trustMock.ExpectQuery(`FROM a2a_trust_scores`).WillReturnError(errors.New("no trust score"))

			status, raw := postEvaluatePolicy(t, policyDB, trustDB, callerOrg,
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
	trustDB, trustMock := newSQLMock(t)

	status, raw := postEvaluatePolicy(t, policyDB, trustDB, callerOrg,
		evaluatePolicyBody(uuid.New(), uuid.New(), otherOrg.String()))

	// No query is expected: sqlmock fails any statement it was not told to
	// expect, which the service would answer with 500.
	require.Equal(t, fiber.StatusForbidden, status, "body: %s", raw)
	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, map[string]any{"error": evaluatePolicyOtherOrganizationMessage}, got)
	assert.NotContains(t, string(raw), otherOrg.String(), "the answer does not echo the organization")
	assert.NoError(t, policyMock.ExpectationsWereMet())
	assert.NoError(t, trustMock.ExpectationsWereMet())
}

func TestA2AEvaluatePolicyWithoutAnOrganizationInTheCredentialsIsRefused(t *testing.T) {
	policyDB, policyMock := newSQLMock(t)
	trustDB, trustMock := newSQLMock(t)

	status, raw := postEvaluatePolicy(t, policyDB, trustDB, uuid.Nil,
		evaluatePolicyBody(uuid.New(), uuid.New(), uuid.New().String()))

	assert.Equal(t, fiber.StatusUnauthorized, status, "body: %s", raw)
	assert.NoError(t, policyMock.ExpectationsWereMet())
	assert.NoError(t, trustMock.ExpectationsWereMet())
}
