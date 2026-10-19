package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// POST /api/v1/auth/delete-account: a signed-in user presents their password
// again and asks for the deletion of their own account. The password is
// verified as at sign-in; the account is deactivated at once, which ends its
// sessions, and the answer gives the time from which the purge deletes it.
// The route runs behind the real AuthMiddleware here, as in main.go, with the
// user record read on every request.

// accountDeletionUsers holds one user in memory and serves the three reads
// and writes the route makes on it: the user record, the record the auth
// middleware reads on every request, and the deletion request.
type accountDeletionUsers struct {
	issuanceUserRepo
	requests   int
	requestErr error
}

func (r *accountDeletionUsers) GetSessionState(_ context.Context, id uuid.UUID) (*domain.UserSessionState, error) {
	if r.user == nil || r.user.ID != id {
		return nil, domain.ErrUserNotFound
	}
	return &domain.UserSessionState{
		ForcePasswordChange: r.user.ForcePasswordChange,
		Deactivated:         r.user.Status == domain.UserStatusDeactivated || r.user.DeletedAt != nil,
	}, nil
}

func (r *accountDeletionUsers) RequestDeletion(_ context.Context, userID uuid.UUID, at time.Time) (domain.AccountDeletion, error) {
	r.requests++
	if r.requestErr != nil {
		return domain.AccountDeletion{}, r.requestErr
	}
	r.user.Status = domain.UserStatusDeactivated
	return domain.AccountDeletion{UserID: userID, OrganizationID: r.user.OrganizationID, RequestedAt: at.Truncate(time.Microsecond)}, nil
}

func (r *accountDeletionUsers) PurgeDeletionRequests(context.Context, time.Time, int) ([]domain.AccountDeletion, error) {
	return nil, nil
}

type accountDeletionFixture struct {
	app      *fiber.App
	users    *accountDeletionUsers
	failures *reauthFailureRepo
	audit    *familyAuditRepo
	auth     *application.AuthService
	user     *domain.User
	access   string
}

func newAccountDeletionFixture(t *testing.T) *accountDeletionFixture {
	t.Helper()
	t.Setenv("JWT_SECRET", sessionAgeTestSecret)
	svc := auth.NewJWTService()
	user := issuanceUser(t, false)
	users := &accountDeletionUsers{issuanceUserRepo: issuanceUserRepo{user: user}}
	svc.SetSessionStateReader(users)
	failures := &reauthFailureRepo{}
	auditRepo := &familyAuditRepo{}
	authService := application.NewAuthService(users, nil, nil, failures, nil, nil)
	deletions := application.NewAccountDeletionService(users, application.NewAuditService(auditRepo))

	h := NewAccountDeletionHandler(users, authService, deletions)
	app := fiber.New()
	app.Post("/api/v1/auth/delete-account", middleware.AuthMiddleware(svc), h.RequestDeletion)
	app.Get("/api/v1/auth/me", middleware.AuthMiddleware(svc), func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	access, _, err := svc.GenerateTokenPair(user.ID.String(), user.OrganizationID.String(), user.Email, string(user.Role))
	require.NoError(t, err)
	return &accountDeletionFixture{
		app: app, users: users, failures: failures, audit: auditRepo, auth: authService, user: user, access: access,
	}
}

func (f *accountDeletionFixture) call(t *testing.T, method, path, bearer string, body []byte) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)
	resp, err := f.app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	require.NoError(t, err)
	defer resp.Body.Close()
	out := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (f *accountDeletionFixture) requestDeletion(t *testing.T, password string) (int, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(map[string]string{"password": password})
	require.NoError(t, err)
	return f.call(t, "POST", "/api/v1/auth/delete-account", f.access, raw)
}

func TestAccountDeletion_DeactivatesTheAccountAndEndsItsSessions(t *testing.T) {
	f := newAccountDeletionFixture(t)
	status, _ := f.call(t, "GET", "/api/v1/auth/me", f.access, nil)
	require.Equal(t, fiber.StatusOK, status, "control: the session acts before the request")

	before := time.Now().UTC().Add(-time.Second)
	status, body := f.requestDeletion(t, issuancePassword)
	require.Equal(t, fiber.StatusAccepted, status, "body: %v", body)
	assert.Equal(t, "deactivated", body["status"])

	requestedAt, err := time.Parse(time.RFC3339Nano, body["deletionRequestedAt"].(string))
	require.NoError(t, err)
	dueAt, err := time.Parse(time.RFC3339Nano, body["deletionDueAt"].(string))
	require.NoError(t, err)
	assert.True(t, requestedAt.After(before) && requestedAt.Before(time.Now().Add(time.Second)), "requested at %s", requestedAt)
	assert.Equal(t, 29*24*time.Hour, dueAt.Sub(requestedAt), "the account is due 29 days after the request")

	assert.Equal(t, 1, f.users.requests)
	assert.Equal(t, domain.UserStatusDeactivated, f.users.user.Status)

	rows := auditRowsOf(f.audit.rows, domain.AuditActionAccountDeletionRequested)
	require.Len(t, rows, 1)
	assert.Equal(t, f.user.OrganizationID, rows[0].OrganizationID)
	require.NotNil(t, rows[0].UserID)
	assert.Equal(t, f.user.ID, *rows[0].UserID)
	assert.Equal(t, f.user.ID, rows[0].ResourceID)
	assert.Equal(t, dueAt.UTC().Format(time.RFC3339), rows[0].Metadata["deletionDueAt"])

	// The session that made the request, like any other of the account, is
	// refused from its next request on.
	status, body = f.call(t, "GET", "/api/v1/auth/me", f.access, nil)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, "Token has been revoked: the account is deactivated", body["error"])
	status, _ = f.requestDeletion(t, issuancePassword)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, 1, f.users.requests, "a deactivated account's session cannot ask again")

	// Sign-in refuses the deactivated account.
	_, err = f.auth.LoginWithPasswordExtended(context.Background(), f.user.Email, issuancePassword, "198.51.100.7", "test")
	assert.Error(t, err, "a deactivated account cannot sign in")
}

func TestAccountDeletion_WrongPasswordIsRefusedAndRecorded(t *testing.T) {
	f := newAccountDeletionFixture(t)

	status, body := f.requestDeletion(t, reauthWrongPassword)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, "Invalid password", body["error"])
	assert.Len(t, f.failures.failures, 1, "the wrong password counts toward the lockout")

	assert.Equal(t, 0, f.users.requests)
	assert.Equal(t, domain.UserStatusActive, f.users.user.Status)
	assert.Empty(t, auditRowsOf(f.audit.rows, domain.AuditActionAccountDeletionRequested))
	status, _ = f.call(t, "GET", "/api/v1/auth/me", f.access, nil)
	assert.Equal(t, fiber.StatusOK, status, "the session still acts")
}

func TestAccountDeletion_LockedAccountIsRefused(t *testing.T) {
	f := newAccountDeletionFixture(t)
	f.failures.lockout = &domain.AuthLockout{Email: f.user.Email, LockedUntil: time.Now().Add(10 * time.Minute)}

	status, body := f.requestDeletion(t, issuancePassword)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, "Invalid password", body["error"])
	assert.Equal(t, 0, f.users.requests)
	assert.Equal(t, domain.UserStatusActive, f.users.user.Status)
}

func TestAccountDeletion_AccountWithoutAPasswordIsRefused(t *testing.T) {
	f := newAccountDeletionFixture(t)
	f.users.user.PasswordHash = nil

	status, body := f.requestDeletion(t, issuancePassword)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, "Invalid password", body["error"])
	assert.Equal(t, 0, f.users.requests)
}

func TestAccountDeletion_LastActiveAdminIsRefused(t *testing.T) {
	f := newAccountDeletionFixture(t)
	f.users.requestErr = domain.ErrLastActiveAdmin

	status, body := f.requestDeletion(t, issuancePassword)
	assert.Equal(t, fiber.StatusConflict, status)
	assert.Equal(t, LastActiveAdminCode, body["code"])
	assert.Contains(t, body["error"], "PUT /api/v1/admin/users/:id/role", "the answer names the route that resolves it")

	assert.Equal(t, 1, f.users.requests)
	assert.Equal(t, domain.UserStatusActive, f.users.user.Status)
	assert.Empty(t, auditRowsOf(f.audit.rows, domain.AuditActionAccountDeletionRequested))
	status, _ = f.call(t, "GET", "/api/v1/auth/me", f.access, nil)
	assert.Equal(t, fiber.StatusOK, status, "the session still acts")
}

func TestAccountDeletion_RequiresThePassword(t *testing.T) {
	f := newAccountDeletionFixture(t)

	for name, raw := range map[string][]byte{
		"no password":  []byte(`{}`),
		"empty":        []byte(`{"password": ""}`),
		"invalid JSON": []byte(`{"password": `),
	} {
		status, _ := f.call(t, "POST", "/api/v1/auth/delete-account", f.access, raw)
		assert.Equal(t, fiber.StatusBadRequest, status, name)
	}
	assert.Equal(t, 0, f.users.requests)
	assert.Empty(t, f.failures.failures, "no password was checked")
}

func TestAccountDeletion_TokenOfAnotherOrganizationIsRefused(t *testing.T) {
	f := newAccountDeletionFixture(t)
	svc := auth.NewJWTService()
	svc.SetSessionStateReader(f.users)
	other, _, err := svc.GenerateTokenPair(f.user.ID.String(), uuid.New().String(), f.user.Email, string(f.user.Role))
	require.NoError(t, err)

	raw, err := json.Marshal(map[string]string{"password": issuancePassword})
	require.NoError(t, err)
	status, body := f.call(t, "POST", "/api/v1/auth/delete-account", other, raw)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, "Invalid or expired token", body["error"])
	assert.Equal(t, 0, f.users.requests)
	assert.Empty(t, f.failures.failures, "no password was checked")
}
