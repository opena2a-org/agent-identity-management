package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/integrations/siem/hectest"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/handlers"
)

// siemPasswordUsers holds users by email.
type siemPasswordUsers struct {
	domain.UserRepository
	byEmail map[string]*domain.User
}

func (u siemPasswordUsers) GetByEmail(email string) (*domain.User, error) {
	if user, ok := u.byEmail[email]; ok {
		copied := *user
		return &copied, nil
	}
	return nil, errors.New("user not found")
}

// siemProviderSignIn is the requirement of provider sign-in: the organization
// requiring requires it, and the requirement of any other organization cannot
// be read.
type siemProviderSignIn struct{ requiring uuid.UUID }

func (s siemProviderSignIn) RequiresProviderSignIn(_ context.Context, organizationID uuid.UUID) (bool, error) {
	if organizationID == s.requiring {
		return true, nil
	}
	return false, errors.New("the database is unreachable")
}

// A member's own password that each password route answers 403 because the
// member's organization requires provider sign-in, and 500 because that
// requirement cannot be read, reaches the SIEM as aim.auth.signin_failed with
// the reasons provider_sign_in_required and sign_in_refused.
func TestObserveSecurityDecisions_ForwardsTheRefusalsOfTheProviderSignInRequirement(t *testing.T) {
	hecToken := uuid.NewString()
	receiver := hectest.NewReceiver(hecToken)
	t.Cleanup(receiver.Close)
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	forwarder, err := startSIEMForwarding(repository.NewAuditLogRepository(db), siemEnv(map[string]string{
		"SIEM_SPLUNK_HEC_URL":   receiver.URL(),
		"SIEM_SPLUNK_HEC_TOKEN": hecToken,
	}))
	require.NoError(t, err)

	const password = "not-a-real-password"
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	hash := string(hashed)
	member := func(email string) *domain.User {
		return &domain.User{ID: uuid.New(), OrganizationID: uuid.New(), Email: email, Name: "Member",
			Role: domain.RoleMember, Status: domain.UserStatusActive, PasswordHash: &hash}
	}
	required, unreadable := member("required@example.com"), member("unreadable@example.com")
	users := siemPasswordUsers{byEmail: map[string]*domain.User{required.Email: required, unreadable.Email: unreadable}}
	authService := application.NewAuthService(users, nil, nil, nil, nil, nil)
	authService.SetProviderSignInRequirement(siemProviderSignIn{requiring: required.OrganizationID})
	h := &Handlers{
		Auth: handlers.NewAuthHandler(authService, nil, nil, nil, nil),
		PublicRegistration: handlers.NewPublicRegistrationHandler(
			application.NewRegistrationService(siemNoRegistrations{}, users, nil, nil, nil), authService, nil, nil),
	}
	observeSecurityDecisions(forwarder, &Services{FGA: application.NewFGAEngine(nil, nil, nil)}, h)

	app := fiber.New()
	app.Post("/api/v1/public/login", h.PublicRegistration.Login)
	app.Post("/api/v1/auth/login/local", h.Auth.LocalLogin)
	refusals := []struct {
		user   *domain.User
		status int
		reason string
	}{
		{required, fiber.StatusForbidden, "provider_sign_in_required"},
		{unreadable, fiber.StatusInternalServerError, "sign_in_refused"},
	}
	var want []string
	for _, route := range []string{"/api/v1/public/login", "/api/v1/auth/login/local"} {
		for _, refusal := range refusals {
			body, err := json.Marshal(map[string]string{"email": refusal.user.Email, "password": password})
			require.NoError(t, err)
			req := httptest.NewRequest(fiber.MethodPost, route, strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
			require.NoError(t, err)
			_ = resp.Body.Close()
			require.Equal(t, refusal.status, resp.StatusCode, "%s for %s", route, refusal.user.Email)
			want = append(want, route+" "+refusal.reason)
		}
	}

	flushSIEMForwarding(forwarder)

	assert.Empty(t, receiver.Refusals())
	var got []string
	for _, e := range receiver.Accepted() {
		assert.NotContains(t, string(e.Event), "@example.com")
		assert.NotContains(t, string(e.Event), password)
		var event map[string]interface{}
		require.NoError(t, e.Decode(&event))
		require.Equal(t, "aim.auth.signin_failed", event["eventType"])
		assert.Equal(t, "low", event["severity"])
		details := event["details"].(map[string]interface{})
		got = append(got, details["route"].(string)+" "+details["reason"].(string))
	}
	assert.ElementsMatch(t, want, got)
}
