package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// An organization's OIDC sign-in configuration can require its users who are
// not administrators to sign in through the provider. The password routes
// then answer such a user's own password with 403 and the code
// provider_sign_in_required in place of a login pair, and a password change
// issues the user no pair. Administrators, and the users of an organization
// that requires none, sign in with a password as before.

const anotherPassword = "FAKE-Passw0rd2!"

// requirementOf is a requirement of provider sign-in by organization.
type requirementOf map[uuid.UUID]bool

func (r requirementOf) RequiresProviderSignIn(_ context.Context, organizationID uuid.UUID) (bool, error) {
	return r[organizationID], nil
}

// unreadableRequirement answers every read with an error.
type unreadableRequirement struct{}

func (unreadableRequirement) RequiresProviderSignIn(context.Context, uuid.UUID) (bool, error) {
	return false, errors.New("the database is unreachable")
}

// withPassword is a copy of u whose password is issuancePassword, hashed at
// bcrypt's minimum cost (see issuanceUser).
func withPassword(t *testing.T, u domain.User) *domain.User {
	t.Helper()
	hashed, err := bcrypt.GenerateFromPassword([]byte(issuancePassword), bcrypt.MinCost)
	require.NoError(t, err)
	hash := string(hashed)
	u.PasswordHash = &hash
	return &u
}

// passwordRoutes mounts the password routes on an auth service that reads
// the requirement of provider sign-in from requirement, over a user
// repository that holds user alone: POST /api/v1/auth/login/local, POST
// /api/v1/public/login, POST /api/v1/public/change-password, and POST
// /api/v1/auth/change-password signed in as user. The two sign-in routes
// report each refused sign-in to the returned reports.
func passwordRoutes(t *testing.T, jwtService *auth.JWTService, requirement application.ProviderSignInRequirement, user *domain.User) (*fiber.App, *issuanceUserRepo, *signInReports) {
	t.Helper()
	users := &issuanceUserRepo{user: user}
	authService := application.NewAuthService(users, nil, nil, nil, nil, nil)
	authService.SetProviderSignInRequirement(requirement)
	audit := application.NewAuditService(&familyAuditRepo{})
	reports := &signInReports{}
	public := NewPublicRegistrationHandler(application.NewRegistrationService(refusalRegistrationRepo{}, users, nil, nil, nil),
		authService, jwtService, audit)
	public.SetSignInFailureObserver(reports.observe)
	local := NewAuthHandler(authService, jwtService, nil, audit, nil)
	local.SetSignInFailureObserver(reports.observe)

	app := fiber.New()
	app.Post("/api/v1/auth/login/local", local.LocalLogin)
	app.Post("/api/v1/public/login", public.Login)
	app.Post("/api/v1/public/change-password", public.ChangePassword)
	app.Post("/api/v1/auth/change-password", func(c fiber.Ctx) error {
		c.Locals("user_id", user.ID)
		return c.Next()
	}, local.ChangePassword)
	return app, users, reports
}

// postPassword posts body to path and returns the status and the JSON answer.
func postPassword(t *testing.T, app *fiber.App, path string, body map[string]string) (int, map[string]interface{}) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	status, text := postSignIn(t, app, path, string(raw))
	var answer map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(text), &answer), text)
	return status, answer
}

// passwordSignInRoutes are the routes that answer a login pair for an email
// and password.
var passwordSignInRoutes = []string{"/api/v1/auth/login/local", "/api/v1/public/login"}

func signInWith(email, password string) map[string]string {
	return map[string]string{"email": email, "password": password}
}

// assertProviderSignInRequired asserts the 403 that answers a password route
// in place of a login pair for a user of org.
func assertProviderSignInRequired(t *testing.T, org uuid.UUID, status int, answer map[string]interface{}) {
	t.Helper()
	assert.Equal(t, fiber.StatusForbidden, status, "%v", answer)
	assert.Equal(t, providerSignInRequiredMessage, answer["error"])
	assert.Equal(t, application.SignInRefusedProviderSignInRequired, answer["code"])
	assert.Equal(t, org.String(), answer["organizationId"])
	assert.NotContains(t, answer, "accessToken")
	assert.NotContains(t, answer, "refreshToken")
}

// putRequirement stores the stub provider as A's through the administration
// route, with requireProviderSignIn set to require when require is not nil,
// and returns the route's status and answer.
func (f *oidcHandlerFixture) putRequirement(t *testing.T, require *bool) (int, map[string]interface{}) {
	t.Helper()
	adminA := map[string]string{"X-Test-Org": f.orgA.String(), "X-Test-User": f.alice.ID.String()}
	body := map[string]interface{}{"issuer": f.stub.Issuer, "clientId": f.stub.ClientID, "clientSecret": f.stub.ClientSecret}
	if require != nil {
		body["requireProviderSignIn"] = *require
	}
	return f.do(t, fiber.MethodPut, "/api/v1/admin/oidc-sign-in", body, adminA)
}

// The administration routes store, answer and audit whether the organization
// requires sign-in through its provider. A body without the member stores a
// configuration that requires none.
func TestOIDCSignInAdministrationRoutesStoreTheRequirementOfProviderSignIn(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	adminA := map[string]string{"X-Test-Org": f.orgA.String(), "X-Test-User": f.alice.ID.String()}
	yes, no := true, false

	status, answer := f.do(t, fiber.MethodGet, "/api/v1/admin/oidc-sign-in", nil, adminA)
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, false, answer["requireProviderSignIn"], "the fixture's configuration requires none")

	status, answer = f.putRequirement(t, &yes)
	require.Equal(t, fiber.StatusOK, status, "%v", answer)
	assert.Equal(t, true, answer["requireProviderSignIn"])
	status, answer = f.do(t, fiber.MethodGet, "/api/v1/admin/oidc-sign-in", nil, adminA)
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, true, answer["requireProviderSignIn"])

	rows := f.audit.taken()
	require.Len(t, rows, 1)
	assert.Equal(t, domain.AuditActionUpdate, rows[0].Action)
	assert.Equal(t, "oidc_sign_in_config", rows[0].ResourceType)
	assert.Equal(t, true, rows[0].Metadata["requireProviderSignIn"], "the audit row names the requirement")

	status, answer = f.putRequirement(t, nil)
	require.Equal(t, fiber.StatusOK, status, "%v", answer)
	assert.Equal(t, false, answer["requireProviderSignIn"], "a body without the member requires none")
	status, answer = f.putRequirement(t, &yes)
	require.Equal(t, fiber.StatusOK, status, "%v", answer)
	status, answer = f.putRequirement(t, &no)
	require.Equal(t, fiber.StatusOK, status, "%v", answer)
	assert.Equal(t, false, answer["requireProviderSignIn"])
	rows = f.audit.taken()
	require.Len(t, rows, 3, "one row for each of the three stores since the rows were last taken")
	assert.Equal(t, false, rows[0].Metadata["requireProviderSignIn"])
	assert.Equal(t, true, rows[1].Metadata["requireProviderSignIn"])
	assert.Equal(t, false, rows[2].Metadata["requireProviderSignIn"])

	// B has no provider, so B requires nothing, and the answer is the one an
	// organization without a provider was given before the requirement.
	adminB := map[string]string{"X-Test-Org": f.orgB.String(), "X-Test-User": f.carolB.ID.String()}
	status, answer = f.do(t, fiber.MethodGet, "/api/v1/admin/oidc-sign-in", nil, adminB)
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, map[string]interface{}{"configured": false, "redirectUrl": oidcTestDashboard + "/auth/oidc/callback"}, answer)
}

// With A's requirement stored through the administration route, both
// password sign-in routes answer Alice, a member of A, her own password with
// the 403, record no sign-in and report the refusal; a wrong password is
// answered as it was, so a caller without the password learns nothing of the
// requirement. Alice then signs in through A's provider.
func TestPasswordSignInIsRefusedToAUserWhoMustSignInThroughTheProvider(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	yes := true
	status, answer := f.putRequirement(t, &yes)
	require.Equal(t, fiber.StatusOK, status, "%v", answer)

	alice := withPassword(t, f.alice)
	app, users, reports := passwordRoutes(t, f.jwt, f.service, alice)
	for _, route := range passwordSignInRoutes {
		status, answer := postPassword(t, app, route, signInWith(alice.Email, issuancePassword))
		assertProviderSignInRequired(t, f.orgA, status, answer)

		status, answer = postPassword(t, app, route, signInWith(alice.Email, "Wrong-Passw0rd!"))
		assert.Equal(t, fiber.StatusUnauthorized, status, route)
		assert.Equal(t, "Invalid email or password", answer["error"], route)
		assert.NotContains(t, answer, "code", route)
		assert.NotContains(t, answer, "organizationId", route)
	}
	_, answer = postPassword(t, app, "/api/v1/public/login", signInWith(alice.Email, issuancePassword))
	assert.Equal(t, false, answer["success"], "the public route's refusal carries success false")
	assert.Nil(t, users.user.LastLoginAt, "no sign-in is recorded")

	require.Len(t, reports.got, 5)
	assertReport(t, reports.got[0], "/api/v1/auth/login/local", SignInFailedProviderSignInRequired, alice)
	assertReport(t, reports.got[1], "/api/v1/auth/login/local", application.SignInRefusedPasswordMismatch, alice)
	assertReport(t, reports.got[2], "/api/v1/public/login", SignInFailedProviderSignInRequired, alice)
	assertReport(t, reports.got[3], "/api/v1/public/login", SignInFailedEmailOrPasswordMismatch, alice)

	status, signedIn := f.signIn(t, f.orgA, verifiedIdentity(alice.Email))
	require.Equal(t, fiber.StatusOK, status, "%v", signedIn)
	access, err := f.jwt.ValidateToken(signedIn["accessToken"].(string))
	require.NoError(t, err)
	assert.Equal(t, alice.ID.String(), access.UserID)
	assert.Equal(t, f.orgA.String(), access.OrganizationID)
}

// The requirement reaches the users of A who are not administrators, and no
// one once it is lifted or A's provider is removed.
func TestPasswordSignInOfAUserTheRequirementDoesNotReach(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	yes := true
	status, answer := f.putRequirement(t, &yes)
	require.Equal(t, fiber.StatusOK, status, "%v", answer)
	adminA := f.alice
	adminA.ID, adminA.Email, adminA.Role = uuid.New(), "admin-a@example.com", domain.RoleAdmin

	signsIn := func(t *testing.T, user *domain.User) {
		t.Helper()
		app, _, reports := passwordRoutes(t, f.jwt, f.service, user)
		for _, route := range passwordSignInRoutes {
			status, answer := postPassword(t, app, route, signInWith(user.Email, issuancePassword))
			require.Equal(t, fiber.StatusOK, status, "%s: %v", route, answer)
			assert.NotEmpty(t, answer["accessToken"], route)
		}
		assert.Empty(t, reports.got)
	}
	t.Run("an administrator of A", func(t *testing.T) { signsIn(t, withPassword(t, adminA)) })
	t.Run("a member of B, which has no provider", func(t *testing.T) { signsIn(t, withPassword(t, f.bob)) })

	t.Run("a member of A once the requirement is lifted", func(t *testing.T) {
		status, answer := f.putRequirement(t, nil)
		require.Equal(t, fiber.StatusOK, status, "%v", answer)
		signsIn(t, withPassword(t, f.alice))
	})
	t.Run("a member of A once A's provider is removed", func(t *testing.T) {
		status, answer := f.putRequirement(t, &yes)
		require.Equal(t, fiber.StatusOK, status, "%v", answer)
		status, _ = f.do(t, fiber.MethodDelete, "/api/v1/admin/oidc-sign-in", nil,
			map[string]string{"X-Test-Org": f.orgA.String(), "X-Test-User": adminA.ID.String()})
		require.Equal(t, fiber.StatusNoContent, status)
		signsIn(t, withPassword(t, f.alice))
	})
}

// A password change by a user who must sign in through the provider stands,
// and issues no pair. The forced change's sign-in is refused as any other.
func TestPasswordChangeIssuesNoPairToAUserWhoMustSignInThroughTheProvider(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	yes := true
	status, answer := f.putRequirement(t, &yes)
	require.Equal(t, fiber.StatusOK, status, "%v", answer)

	t.Run("a forced change", func(t *testing.T) {
		alice := withPassword(t, f.alice)
		alice.ForcePasswordChange = true
		app, users, _ := passwordRoutes(t, f.jwt, f.service, alice)

		status, answer := postPassword(t, app, "/api/v1/public/login", signInWith(alice.Email, issuancePassword))
		assertProviderSignInRequired(t, f.orgA, status, answer)
		assert.NotContains(t, answer, "requiresPasswordChange")

		status, answer = postPassword(t, app, "/api/v1/public/change-password", map[string]string{
			"email": alice.Email, "oldPassword": issuancePassword, "newPassword": anotherPassword,
		})
		require.Equal(t, fiber.StatusOK, status, "%v", answer)
		assert.Equal(t, map[string]interface{}{
			"success":        true,
			"message":        "Password changed successfully. " + providerSignInRequiredMessage,
			"code":           application.SignInRefusedProviderSignInRequired,
			"organizationId": f.orgA.String(),
		}, answer)
		assert.False(t, users.user.ForcePasswordChange, "the change stands")
		require.NotNil(t, users.user.PasswordHash)
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(*users.user.PasswordHash), []byte(anotherPassword)))

		status, answer = postPassword(t, app, "/api/v1/public/login", signInWith(alice.Email, anotherPassword))
		assertProviderSignInRequired(t, f.orgA, status, answer)
	})

	t.Run("a signed-in change", func(t *testing.T) {
		alice := withPassword(t, f.alice)
		app, users, _ := passwordRoutes(t, f.jwt, f.service, alice)
		status, answer := postPassword(t, app, "/api/v1/auth/change-password", map[string]string{
			"currentPassword": issuancePassword, "newPassword": anotherPassword,
		})
		require.Equal(t, fiber.StatusOK, status, "%v", answer)
		assert.Equal(t, map[string]interface{}{
			"message":        "Password changed successfully",
			"code":           application.SignInRefusedProviderSignInRequired,
			"organizationId": f.orgA.String(),
		}, answer)
		assert.NotNil(t, users.user.PasswordChangedAt, "the change stands")
	})

	t.Run("an administrator's signed-in change", func(t *testing.T) {
		adminA := f.alice
		adminA.ID, adminA.Email, adminA.Role = uuid.New(), "admin-a@example.com", domain.RoleAdmin
		app, _, _ := passwordRoutes(t, f.jwt, f.service, withPassword(t, adminA))
		status, answer := postPassword(t, app, "/api/v1/auth/change-password", map[string]string{
			"currentPassword": issuancePassword, "newPassword": anotherPassword,
		})
		require.Equal(t, fiber.StatusOK, status, "%v", answer)
		assert.NotEmpty(t, answer["accessToken"])
		assert.NotContains(t, answer, "code")
	})
}

// When the requirement cannot be read, a user it could reach is refused a
// pair: the sign-in routes answer 500 and report the refusal, and a password
// change stands without one. An administrator signs in as before.
func TestPasswordRoutesWhenTheRequirementCannotBeRead(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-only-jwt-secret-not-a-real-value-0123456789")
	jwtService := auth.NewJWTService()
	// The member must change the password, so the forced change route below
	// takes it.
	member := withPassword(t, domain.User{ID: uuid.New(), OrganizationID: uuid.New(), Email: "member@example.com",
		Name: "Member", Role: domain.RoleMember, Status: domain.UserStatusActive, ForcePasswordChange: true})

	app, users, reports := passwordRoutes(t, jwtService, unreadableRequirement{}, member)
	for _, route := range passwordSignInRoutes {
		status, answer := postPassword(t, app, route, signInWith(member.Email, issuancePassword))
		assert.Equal(t, fiber.StatusInternalServerError, status, route)
		assert.Equal(t, ServerErrorMessage, answer["error"], route)
		assert.NotContains(t, answer, "accessToken", route)
	}
	require.Len(t, reports.got, 2)
	assertReport(t, reports.got[0], "/api/v1/auth/login/local", signInFailedUnclassified, nil)
	assertReport(t, reports.got[1], "/api/v1/public/login", signInFailedUnclassified, member)
	assert.Nil(t, users.user.LastLoginAt)

	status, answer := postPassword(t, app, "/api/v1/public/change-password", map[string]string{
		"email": member.Email, "oldPassword": issuancePassword, "newPassword": anotherPassword,
	})
	assert.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, map[string]interface{}{"success": true, "message": "Password changed successfully. Sign in again to continue."}, answer)

	status, answer = postPassword(t, app, "/api/v1/auth/change-password", map[string]string{
		"currentPassword": anotherPassword, "newPassword": issuancePassword,
	})
	assert.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, map[string]interface{}{"message": "Password changed successfully"}, answer)

	admin := withPassword(t, domain.User{ID: uuid.New(), OrganizationID: member.OrganizationID, Email: "admin@example.com",
		Name: "Admin", Role: domain.RoleAdmin, Status: domain.UserStatusActive})
	app, _, _ = passwordRoutes(t, jwtService, unreadableRequirement{}, admin)
	for _, route := range passwordSignInRoutes {
		status, answer := postPassword(t, app, route, signInWith(admin.Email, issuancePassword))
		require.Equal(t, fiber.StatusOK, status, "%s: %v", route, answer)
		assert.NotEmpty(t, answer["accessToken"], route)
	}
}

// A signed-in user who must sign in through the provider cannot
// re-authenticate with a password: 403, nothing rotated, retired or
// recorded. A wrong password is answered as before, and an administrator
// re-authenticates.
func TestReauth_UserWhoMustSignInThroughTheProviderIsRefused(t *testing.T) {
	store := &rotationStore{}
	f := newReauthFixture(t, store, nil)
	f.user.Role = domain.RoleMember
	f.authService.SetProviderSignInRequirement(requirementOf{f.user.OrganizationID: true})

	status, body := postReauth(t, f.app, f.access, reauthWrongPassword, f.refresh)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, map[string]any{"error": "Invalid password"}, body)

	status, body = postReauth(t, f.app, f.access, issuancePassword, f.refresh)
	assert.Equal(t, fiber.StatusForbidden, status)
	assert.Equal(t, map[string]any{
		"error":          providerSignInRequiredMessage,
		"code":           application.SignInRefusedProviderSignInRequired,
		"organizationId": f.user.OrganizationID.String(),
	}, body)
	f.assertNothingIssued(t, body)
	assert.Zero(t, store.writes, "nothing retired")
	assert.Len(t, f.failures.failures, 1, "the wrong password alone is a failed attempt")

	f.user.Role = domain.RoleAdmin
	status, body = postReauth(t, f.app, f.access, issuancePassword, f.refresh)
	assert.Equal(t, fiber.StatusOK, status, "%v", body)
	assert.NotEmpty(t, body["accessToken"])
}

// When the requirement cannot be read, a user it could reach is not
// re-authenticated: 500, nothing rotated, retired or recorded.
func TestReauth_WhenTheRequirementCannotBeRead(t *testing.T) {
	store := &rotationStore{}
	f := newReauthFixture(t, store, nil)
	f.user.Role = domain.RoleMember
	f.authService.SetProviderSignInRequirement(unreadableRequirement{})

	status, body := postReauth(t, f.app, f.access, issuancePassword, f.refresh)
	assert.Equal(t, fiber.StatusInternalServerError, status)
	assert.Equal(t, map[string]any{"error": ServerErrorMessage}, body)
	f.assertNothingIssued(t, body)
	assert.Zero(t, store.writes, "nothing retired")
}
