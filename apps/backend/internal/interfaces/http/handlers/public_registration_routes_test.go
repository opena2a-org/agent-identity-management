package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// Minimal repositories for the refusal path: the routes read the user and the request by
// email (the access route also checks any status), then the service counts approvers. Nothing else is reachable before the refusal, so
// every other method is left to the embedded nil interface (a call would panic the test).
type refusalUserRepo struct{ domain.UserRepository }

func (refusalUserRepo) GetByEmail(string) (*domain.User, error) { return nil, errors.New("not found") }
func (refusalUserRepo) CountByRoleAndStatus(domain.UserRole, domain.UserStatus) (int, error) {
	return 0, nil
}

type refusalRegistrationRepo struct {
	application.RegistrationRepository
}

func (refusalRegistrationRepo) GetRegistrationRequestByEmail(context.Context, string) (*domain.UserRegistrationRequest, error) {
	return nil, errors.New("not found")
}
func (refusalRegistrationRepo) GetRegistrationRequestByEmailAnyStatus(context.Context, string) (*domain.UserRegistrationRequest, error) {
	return nil, errors.New("not found")
}

// Both public creators must map the bare sentinel to 503 with the code and the message that
// names the variable. A helper-level test cannot see a route that stops calling the helper.
func TestPublicRegistrationRoutes_RefuseWith503AndCodeWhenNoAdministratorExists(t *testing.T) {
	t.Setenv("AIM_PLATFORM_ADMINS", "ops@example.com") // set but unclaimed: predicate B, not A
	service := application.NewRegistrationService(refusalRegistrationRepo{}, refusalUserRepo{}, nil, nil, nil)
	// The access-request route asks the auth service for the user first; it only forwards to the repo.
	authService := application.NewAuthService(refusalUserRepo{}, nil, nil, nil, nil, nil)
	handler := NewPublicRegistrationHandler(service, authService, nil)

	app := fiber.New()
	app.Post("/register", handler.RegisterUser)
	app.Post("/request-access", handler.RequestAccess)

	cases := []struct {
		route string
		body  map[string]any
	}{
		{"/register", map[string]any{"email": "first@example.com", "firstName": "First", "lastName": "User", "password": "Str0ng!Passw0rd"}},
		{"/request-access", map[string]any{"email": "access@example.com", "fullName": "Access User", "reason": "needs a dashboard account"}},
	}
	for _, tc := range cases {
		t.Run(tc.route, func(t *testing.T) {
			payload, _ := json.Marshal(tc.body)
			req := httptest.NewRequest("POST", tc.route, bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			// Fiber's default test timeout is one second; a loaded CI runner needs more.
			resp, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			if resp.StatusCode != fiber.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503", resp.StatusCode)
			}
			var body map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body["code"] != NoAdministratorsCode {
				t.Fatalf("code = %v, want %q", body["code"], NoAdministratorsCode)
			}
			msg, _ := body["error"].(string)
			if !strings.Contains(msg, "AIM_PLATFORM_ADMINS") {
				t.Fatalf("message does not name the variable: %q", msg)
			}
		})
	}
}
