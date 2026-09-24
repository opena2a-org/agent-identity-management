package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// A sign-in answers with the token pair in the body and sets no session cookie. The API
// accepts a session only from the Authorization header, and a cookie the browser keeps is
// a second copy of the sign-in that the dashboard cannot see or clear.

const issuancePassword = "Issuance-Cell-Passw0rd!"

// issuanceUserRepo holds one user in memory. Every other method is left to the embedded nil
// interface, so a call the sign-in paths do not make panics the test.
type issuanceUserRepo struct {
	domain.UserRepository
	user *domain.User
}

func (r *issuanceUserRepo) GetByEmail(email string) (*domain.User, error) {
	if r.user != nil && strings.EqualFold(r.user.Email, email) {
		copied := *r.user
		return &copied, nil
	}
	return nil, http.ErrNoCookie
}

func (r *issuanceUserRepo) GetByID(id uuid.UUID) (*domain.User, error) {
	if r.user != nil && r.user.ID == id {
		copied := *r.user
		return &copied, nil
	}
	return nil, http.ErrNoCookie
}

func (r *issuanceUserRepo) Update(u *domain.User) error {
	copied := *u
	r.user = &copied
	return nil
}

func issuanceUser(t *testing.T, forceChange bool) *domain.User {
	t.Helper()
	hash, err := auth.NewPasswordHasher().HashPassword(issuancePassword)
	require.NoError(t, err)
	return &domain.User{
		ID:                  uuid.New(),
		OrganizationID:      uuid.New(),
		Email:               "issuance@example.com",
		Name:                "Issuance Cell",
		Role:                domain.RoleAdmin,
		Status:              domain.UserStatusActive,
		PasswordHash:        &hash,
		ForcePasswordChange: forceChange,
	}
}

type issuanceRow struct {
	name        string
	forceChange bool
	route       string
	contentType string
	body        func() string
}

func TestSignIn_IssuesNoSessionCookie(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-only-jwt-secret-not-a-real-value-0123456789")
	jsonLogin := func() string {
		b, _ := json.Marshal(map[string]string{"email": "issuance@example.com", "password": issuancePassword})
		return string(b)
	}
	rows := []issuanceRow{
		{"public login, JSON", false, "/api/v1/public/login", "application/json", jsonLogin},
		{"public login, form", false, "/api/v1/public/login", "application/x-www-form-urlencoded", func() string {
			return url.Values{"email": {"issuance@example.com"}, "password": {issuancePassword}}.Encode()
		}},
		{"public login, password change required", true, "/api/v1/public/login", "application/json", jsonLogin},
		{"public change-password", true, "/api/v1/public/change-password", "application/json", func() string {
			b, _ := json.Marshal(map[string]string{"email": "issuance@example.com", "oldPassword": issuancePassword, "newPassword": "Another-Cell-Passw0rd!"})
			return string(b)
		}},
		{"local login", false, "/api/v1/auth/login/local", "application/json", jsonLogin},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			users := &issuanceUserRepo{user: issuanceUser(t, row.forceChange)}
			authService := application.NewAuthService(users, nil, nil, nil, nil, nil)
			jwtService := auth.NewJWTService()

			app := fiber.New()
			public := NewPublicRegistrationHandler(application.NewRegistrationService(refusalRegistrationRepo{}, users, nil, nil, nil), authService, jwtService)
			app.Post("/api/v1/public/login", public.Login)
			app.Post("/api/v1/public/change-password", public.ChangePassword)
			local := NewAuthHandler(authService, jwtService, nil, application.NewAuditService(&familyAuditRepo{}))
			app.Post("/api/v1/auth/login/local", local.LocalLogin)

			req := httptest.NewRequest("POST", row.route, strings.NewReader(row.body()))
			req.Header.Set("Content-Type", row.contentType)
			resp, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
			require.NoError(t, err)
			defer resp.Body.Close()
			raw, _ := io.ReadAll(resp.Body)

			require.Equal(t, fiber.StatusOK, resp.StatusCode, "the sign-in succeeds: %s", string(raw))
			var body map[string]any
			require.NoError(t, json.Unmarshal(raw, &body))
			access, _ := body["accessToken"].(string)
			assert.NotEmpty(t, access, "the access token is in the body")

			for _, c := range resp.Cookies() {
				if c.Name == "access_token" || c.Name == "refresh_token" {
					// Never print the value: report its length only.
					assert.True(t, c.Value == "", "the answer sets a non-empty %s cookie (%d bytes)", c.Name, len(c.Value))
				}
			}
		})
	}
}
