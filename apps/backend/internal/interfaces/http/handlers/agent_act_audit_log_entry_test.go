package handlers

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// docs/API.md says that this version of the server writes no audit record for a
// verification, a reactivation or an agent delete, only an audit log entry. Each route
// runs here through its handler over the audit service and the audit log repository, on
// a sqlmock connection, with an agent service whose change succeeds: the route writes one
// audit_logs row for the agent.

// auditWriteAttempts is the audit service with the error of each LogAction call kept, so
// a test counts the writes a route attempts and not only those the mock accepts.
type auditWriteAttempts struct {
	*application.AuditService
	errs []error
}

func (a *auditWriteAttempts) LogAction(ctx context.Context, orgID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress, userAgent string, metadata map[string]interface{}) error {
	err := a.AuditService.LogAction(ctx, orgID, userID, action, resourceType, resourceID, ipAddress, userAgent, metadata)
	a.errs = append(a.errs, err)
	return err
}

// serveAgentAct sends method to /agents/:id plus path for an agent in status, through the
// verify, reactivate and delete handlers over a sqlmock connection that expects one
// audit_logs row of action for the agent. The agent service applies verify and reactivate
// to the agent's status and answers a delete with deleteErr. It returns the status code,
// the audit service and the mock.
func serveAgentAct(t *testing.T, method, path string, status domain.AgentStatus, action domain.AuditAction, deleteErr error) (int, *auditWriteAttempts, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	agent := newLifecycleAgent(status)
	userID := uuid.New()
	mock.ExpectExec(`INSERT INTO audit_logs`).
		WithArgs(sqlmock.AnyArg(), agent.OrganizationID, userID, nil, action, "agent", agent.ID,
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	agents := lifecycleService(agent)
	agents.DeleteAgentFunc = func(context.Context, uuid.UUID) error { return deleteErr }
	audit := &auditWriteAttempts{AuditService: application.NewAuditService(repository.NewAuditLogRepository(db))}
	handler := NewAgentHandlerWithInterfaces(
		agents,
		&MockMCPServiceImpl{},
		audit,
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)
	app := createTestAppWithAuth(handler, agent.OrganizationID, userID)
	app.Post("/agents/:id/verify", handler.VerifyAgent)
	app.Post("/agents/:id/reactivate", handler.ReactivateAgent)
	app.Delete("/agents/:id", handler.DeleteAgent)

	resp, err := app.Test(httptest.NewRequest(method, "/agents/"+agent.ID.String()+path, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode, audit, mock
}

func TestAgentVerifyReactivateAndDeleteWriteOneAuditLogEntry(t *testing.T) {
	cases := []struct {
		method, path string
		status       domain.AgentStatus
		code         int
		action       domain.AuditAction
	}{
		{"POST", "/verify", domain.AgentStatusPending, fiber.StatusOK, domain.AuditActionVerify},
		{"POST", "/reactivate", domain.AgentStatusSuspended, fiber.StatusOK, domain.AuditActionUpdate},
		{"DELETE", "", domain.AgentStatusVerified, fiber.StatusNoContent, domain.AuditActionDelete},
	}
	for _, tc := range cases {
		t.Run(tc.method+" /agents/:id"+tc.path, func(t *testing.T) {
			code, audit, mock := serveAgentAct(t, tc.method, tc.path, tc.status, tc.action, nil)

			assert.Equal(t, tc.code, code)
			assert.Equal(t, []error{nil}, audit.errs, "the route attempts one audit log write, and it succeeds")
			assert.NoError(t, mock.ExpectationsWereMet(), "the write is an audit_logs row for the agent")
		})
	}
}

// The negative control: a delete the agent service refuses attempts no audit log write,
// and the audit_logs row the mock expects is not written.
func TestAgentDeleteTheServiceRefusesWritesNoAuditLogEntry(t *testing.T) {
	code, audit, mock := serveAgentAct(t, "DELETE", "", domain.AgentStatusVerified, domain.AuditActionDelete, repositoryFailure)

	assert.Equal(t, fiber.StatusInternalServerError, code)
	assert.Empty(t, audit.errs)
	assert.Error(t, mock.ExpectationsWereMet())
}
