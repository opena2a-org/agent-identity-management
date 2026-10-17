package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"golang.org/x/crypto/bcrypt"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// rejectedRegistrationRepo holds one rejected registration request.
type rejectedRegistrationRepo struct {
	application.RegistrationRepository
	request *domain.UserRegistrationRequest
}

func (r rejectedRegistrationRepo) GetRegistrationRequestByEmailAnyStatus(_ context.Context, email string) (*domain.UserRegistrationRequest, error) {
	if r.request != nil && strings.EqualFold(r.request.Email, email) {
		return r.request, nil
	}
	return refusalRegistrationRepo{}.GetRegistrationRequestByEmailAnyStatus(context.Background(), email)
}

func signInBody(email, password string) string {
	b, _ := json.Marshal(map[string]string{"email": email, "password": password})
	return string(b)
}

func postSignIn(t *testing.T, app *fiber.App, path, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(fiber.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "sign-in-report-test/1.0")
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(raw)
}

type signInReports struct{ got []SignInFailure }

func (r *signInReports) observe(f SignInFailure) { r.got = append(r.got, f) }

func assertReport(t *testing.T, f SignInFailure, route, reason string, user *domain.User) {
	t.Helper()
	assert.Equal(t, route, f.Route)
	assert.Equal(t, "password", f.Method)
	assert.Equal(t, reason, f.Reason)
	assert.NotEmpty(t, f.IPAddress)
	assert.Equal(t, "sign-in-report-test/1.0", f.UserAgent)
	assert.False(t, f.Time.IsZero())
	if user != nil {
		assert.Equal(t, user.ID, f.UserID)
		assert.Equal(t, user.OrganizationID, f.OrganizationID)
	} else {
		assert.Equal(t, uuid.Nil, f.UserID)
		assert.Equal(t, uuid.Nil, f.OrganizationID)
	}
}

func publicSignInApp(t *testing.T, user *domain.User, registrations application.RegistrationRepository, observe func(SignInFailure)) *fiber.App {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests-32")
	users := &issuanceUserRepo{user: user}
	authService := application.NewAuthService(users, nil, nil, nil, nil, nil)
	h := NewPublicRegistrationHandler(application.NewRegistrationService(registrations, users, nil, nil, nil),
		authService, auth.NewJWTService(), application.NewAuditService(&familyAuditRepo{}))
	h.SetSignInFailureObserver(observe)
	app := fiber.New()
	app.Post("/public/login", h.Login)
	return app
}

// POST /public/login reports each 401 with the check that answered it, and
// names the user only when the email matched one. The answers are the ones
// the route gives without an observer.
func TestPublicLogin_ReportsEachRefusedSignIn(t *testing.T) {
	user := issuanceUser(t, false)
	deactivated := issuanceUser(t, false)
	deactivated.Status = domain.UserStatusDeactivated
	rejectedHash, err := bcrypt.GenerateFromPassword([]byte(issuancePassword), bcrypt.MinCost)
	require.NoError(t, err)
	rejectedHashText := string(rejectedHash)
	rejected := rejectedRegistrationRepo{request: &domain.UserRegistrationRequest{
		ID: uuid.New(), Email: "rejected@example.com", Status: domain.RegistrationStatusRejected, PasswordHash: &rejectedHashText,
	}}

	cases := []struct {
		name          string
		user          *domain.User
		registrations application.RegistrationRepository
		body          string
		reason        string
		namesUser     bool
	}{
		{"a wrong password", user, refusalRegistrationRepo{}, signInBody(user.Email, "Wrong-Passw0rd!"), SignInFailedEmailOrPasswordMismatch, true},
		{"an unknown email", user, refusalRegistrationRepo{}, signInBody("unknown@example.com", issuancePassword), SignInFailedEmailOrPasswordMismatch, false},
		{"a deactivated user", deactivated, refusalRegistrationRepo{}, signInBody(deactivated.Email, issuancePassword), SignInFailedAccountDeactivated, true},
		{"a rejected registration", user, rejected, signInBody("rejected@example.com", issuancePassword), SignInFailedRegistrationRejected, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reports := &signInReports{}
			status, body := postSignIn(t, publicSignInApp(t, tc.user, tc.registrations, reports.observe), "/public/login", tc.body)
			unobservedStatus, unobservedBody := postSignIn(t, publicSignInApp(t, tc.user, tc.registrations, nil), "/public/login", tc.body)

			assert.Equal(t, fiber.StatusUnauthorized, status)
			assert.Equal(t, unobservedStatus, status)
			assert.Equal(t, unobservedBody, body, "the observer does not change the answer")
			require.Len(t, reports.got, 1)
			var named *domain.User
			if tc.namesUser {
				named = tc.user
			}
			assertReport(t, reports.got[0], "/public/login", tc.reason, named)
			for _, f := range reports.got {
				encoded, _ := json.Marshal(f)
				assert.NotContains(t, string(encoded), "@example.com", "a report carries no submitted email")
				assert.NotContains(t, string(encoded), issuancePassword, "a report carries no password")
			}
		})
	}

	t.Run("an unknown email is answered as a wrong password is", func(t *testing.T) {
		app := publicSignInApp(t, user, refusalRegistrationRepo{}, (&signInReports{}).observe)
		wrongStatus, wrongBody := postSignIn(t, app, "/public/login", signInBody(user.Email, "Wrong-Passw0rd!"))
		unknownStatus, unknownBody := postSignIn(t, app, "/public/login", signInBody("unknown@example.com", "Wrong-Passw0rd!"))
		assert.Equal(t, wrongStatus, unknownStatus)
		assert.Equal(t, wrongBody, unknownBody)
		assert.JSONEq(t, `{"success":false,"error":"Invalid email or password"}`, wrongBody)
	})

	t.Run("a successful sign-in is not reported", func(t *testing.T) {
		reports := &signInReports{}
		status, body := postSignIn(t, publicSignInApp(t, user, refusalRegistrationRepo{}, reports.observe), "/public/login", signInBody(user.Email, issuancePassword))
		require.Equal(t, fiber.StatusOK, status, body)
		assert.Empty(t, reports.got)
	})
}

func localSignInApp(t *testing.T, user *domain.User, observe func(SignInFailure)) *fiber.App {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests-32")
	h := NewAuthHandler(application.NewAuthService(&issuanceUserRepo{user: user}, nil, nil, nil, nil, nil),
		auth.NewJWTService(), nil, application.NewAuditService(&familyAuditRepo{}), nil)
	h.SetSignInFailureObserver(observe)
	app := fiber.New()
	app.Post("/auth/login/local", h.LocalLogin)
	return app
}

// POST /auth/login/local reports each 401 with the reason the service's
// refusal names. Every refusal keeps the one answer the route gives.
func TestLocalLogin_ReportsEachRefusedSignIn(t *testing.T) {
	user := issuanceUser(t, false)
	deactivated := issuanceUser(t, false)
	deactivated.Status = domain.UserStatusDeactivated
	noPassword := issuanceUser(t, false)
	noPassword.PasswordHash = nil

	cases := []struct {
		name      string
		user      *domain.User
		body      string
		reason    string
		namesUser bool
	}{
		{"a wrong password", user, signInBody(user.Email, "Wrong-Passw0rd!"), application.SignInRefusedPasswordMismatch, true},
		{"an unknown email", user, signInBody("unknown@example.com", issuancePassword), application.SignInRefusedUnknownEmail, false},
		{"a deactivated user", deactivated, signInBody(deactivated.Email, issuancePassword), application.SignInRefusedAccountDeactivated, true},
		{"a user without a password", noPassword, signInBody(noPassword.Email, issuancePassword), application.SignInRefusedPasswordNotSet, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reports := &signInReports{}
			status, body := postSignIn(t, localSignInApp(t, tc.user, reports.observe), "/auth/login/local", tc.body)
			unobservedStatus, unobservedBody := postSignIn(t, localSignInApp(t, tc.user, nil), "/auth/login/local", tc.body)

			assert.Equal(t, fiber.StatusUnauthorized, status)
			assert.Equal(t, unobservedStatus, status)
			assert.Equal(t, unobservedBody, body, "the observer does not change the answer")
			assert.JSONEq(t, `{"error":"Invalid email or password"}`, body)
			require.Len(t, reports.got, 1)
			var named *domain.User
			if tc.namesUser {
				named = tc.user
			}
			assertReport(t, reports.got[0], "/auth/login/local", tc.reason, named)
		})
	}

	t.Run("a successful sign-in is not reported", func(t *testing.T) {
		reports := &signInReports{}
		status, body := postSignIn(t, localSignInApp(t, user, reports.observe), "/auth/login/local", signInBody(user.Email, issuancePassword))
		require.Equal(t, fiber.StatusOK, status, body)
		assert.Empty(t, reports.got)
	})
}

// The report an observer receives keeps the request's own address and user
// agent after the request has ended. c.IP behind a trusted proxy and c.Get
// hand the handler views of the server's request buffer, which the server
// rewrites in place when it reads the next request, and the SIEM forwarder
// serializes the report's event after the answer has been sent.
func TestReportSignInFailure_KeepsTheRequestsOwnAddressAndUserAgent(t *testing.T) {
	app := fiber.New(fiber.Config{
		ProxyHeader:      fiber.HeaderXForwardedFor,
		TrustProxy:       true,
		TrustProxyConfig: fiber.TrustProxyConfig{Loopback: true},
	})
	fctx := &fasthttp.RequestCtx{}
	fctx.SetRemoteAddr(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 4321})
	fctx.Request.Header.Set(fiber.HeaderXForwardedFor, "203.0.113.70")
	fctx.Request.Header.Set(fiber.HeaderUserAgent, "sign-in-report-test/1.0")
	c := app.AcquireCtx(fctx)
	defer app.ReleaseCtx(c)
	addressView, userAgentView := c.IP(), c.Get(fiber.HeaderUserAgent)
	require.Equal(t, "203.0.113.70", addressView)
	require.Equal(t, "sign-in-report-test/1.0", userAgentView)

	reports := &signInReports{}
	userID, orgID := uuid.New(), uuid.New()
	reportSignInFailure(c, reports.observe, SignInFailedEmailOrPasswordMismatch, userID, orgID)
	require.Len(t, reports.got, 1)

	// The next request's headers, read into the same buffer: the views change.
	fctx.Request.Header.Set(fiber.HeaderXForwardedFor, "198.51.100.9")
	fctx.Request.Header.Set(fiber.HeaderUserAgent, "later-request-agent/9.9")
	require.Equal(t, "198.51.100.9", addressView, "c.IP no longer hands out a view of the request buffer; this test no longer models the hazard")
	require.Equal(t, "later-request-agent/9.9", userAgentView, "c.Get no longer hands out a view of the request buffer; this test no longer models the hazard")

	got := reports.got[0]
	assert.Equal(t, "203.0.113.70", got.IPAddress)
	assert.Equal(t, "sign-in-report-test/1.0", got.UserAgent)
	assert.Equal(t, "password", got.Method)
	assert.Equal(t, SignInFailedEmailOrPasswordMismatch, got.Reason)
	assert.Equal(t, userID, got.UserID)
	assert.Equal(t, orgID, got.OrganizationID)
}
