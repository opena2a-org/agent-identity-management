package handlers

import (
	"encoding/json"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// putProvisioning stores the stub provider as A's through the administration
// route, with provisioning, and returns the route's status and answer.
func (f *oidcHandlerFixture) putProvisioning(t *testing.T, provisioning map[string]interface{}) (int, map[string]interface{}) {
	t.Helper()
	adminA := map[string]string{"X-Test-Org": f.orgA.String(), "X-Test-User": f.alice.ID.String()}
	return f.do(t, fiber.MethodPut, "/api/v1/admin/oidc-sign-in", map[string]interface{}{
		"issuer": f.stub.Issuer, "clientId": f.stub.ClientID, "clientSecret": f.stub.ClientSecret,
		"provisioning": provisioning,
	}, adminA)
}

func asJSON(t *testing.T, v interface{}) string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return string(raw)
}

// With provisioning stored through the administration route, the first
// sign-in of a verified email at the configured domain creates a user of A,
// recorded in the audit log, and answers a login pair for it; the next
// sign-in signs the same user in and creates none.
func TestOIDCSignInRoutesProvisionAUserOfTheConfiguredOrganization(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	status, answer := f.putProvisioning(t, map[string]interface{}{
		"enabled": true, "role": "member", "emailDomains": []string{"Example.com"},
	})
	require.Equal(t, fiber.StatusOK, status, "%v", answer)
	want := `{"emailDomains":["example.com"],"enabled":true,"role":"member"}`
	assert.JSONEq(t, want, asJSON(t, answer["provisioning"]))
	rows := f.audit.taken()
	require.Len(t, rows, 1)
	assert.Equal(t, f.stub.Issuer, rows[0].Metadata["issuer"])
	assert.Equal(t, f.stub.ClientID, rows[0].Metadata["clientId"])
	assert.JSONEq(t, want, asJSON(t, rows[0].Metadata["provisioning"]), "the audit row of the change names the provisioning")

	adminA := map[string]string{"X-Test-Org": f.orgA.String(), "X-Test-User": f.alice.ID.String()}
	status, answer = f.do(t, fiber.MethodGet, "/api/v1/admin/oidc-sign-in", nil, adminA)
	require.Equal(t, fiber.StatusOK, status)
	assert.JSONEq(t, want, asJSON(t, answer["provisioning"]))

	before := f.users.all()
	status, signedIn := f.signIn(t, f.orgA, verifiedIdentity("dave@example.com"))
	require.Equal(t, fiber.StatusOK, status, "%v", signedIn)
	user, _ := signedIn["user"].(map[string]interface{})
	require.NotNil(t, user)
	assert.Equal(t, "dave@example.com", user["email"])
	assert.Equal(t, "member", user["role"])
	assert.Equal(t, f.orgA.String(), user["organizationId"])
	daveID, err := uuid.Parse(user["id"].(string))
	require.NoError(t, err)
	access, err := f.jwt.ValidateToken(signedIn["accessToken"].(string))
	require.NoError(t, err)
	assert.Equal(t, daveID.String(), access.UserID)
	assert.Equal(t, f.orgA.String(), access.OrganizationID)

	after := f.users.all()
	require.Len(t, after, len(before)+1)
	assert.Equal(t, f.orgA, after[daveID].OrganizationID)
	for id, u := range before {
		assert.Equal(t, u, after[id], "no other user changed")
	}

	rows = f.audit.taken()
	require.Len(t, rows, 2, "the create of the user, then the login")
	assert.Equal(t, domain.AuditActionCreate, rows[0].Action)
	assert.Equal(t, "user", rows[0].ResourceType)
	assert.Equal(t, daveID, rows[0].ResourceID)
	assert.Equal(t, f.orgA, rows[0].OrganizationID)
	assert.Equal(t, daveID, *rows[0].UserID)
	assert.Equal(t, "oidc_provisioning", rows[0].Metadata["method"])
	assert.Equal(t, domain.AuditActionLogin, rows[1].Action)
	assert.Equal(t, daveID, rows[1].ResourceID)

	status, again := f.signIn(t, f.orgA, verifiedIdentity("dave@example.com"))
	require.Equal(t, fiber.StatusOK, status, "%v", again)
	assert.Equal(t, daveID.String(), again["user"].(map[string]interface{})["id"])
	assert.Len(t, f.users.all(), len(before)+1)
	rows = f.audit.taken()
	require.Len(t, rows, 1, "only the login")
	assert.Equal(t, domain.AuditActionLogin, rows[0].Action)
}

// With provisioning on at example.com, A's provider asserting the email of
// Bob, a user of B, signs in no one and creates no user, and an address at
// another domain is refused as well.
func TestOIDCSignInRoutesNeverProvisionTheEmailOfAnotherOrganizationsUser(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	status, answer := f.putProvisioning(t, map[string]interface{}{
		"enabled": true, "role": "manager", "emailDomains": []string{"example.com"},
	})
	require.Equal(t, fiber.StatusOK, status, "%v", answer)
	_ = f.audit.taken()
	before := f.users.all()

	for email, code := range map[string]string{
		"bob@example.com":    application.OIDCSignInRefusedEmailInOtherOrganization,
		"mallory@other.test": application.OIDCSignInRefusedDomainNotProvisioned,
	} {
		status, answer := f.signIn(t, f.orgA, verifiedIdentity(email))
		assert.Equal(t, fiber.StatusUnauthorized, status, email)
		assert.Equal(t, code, answer["code"], email)
		assert.NotContains(t, answer, "accessToken", email)
	}
	assert.Equal(t, before, f.users.all(), "no user was created, changed or moved")
	assert.Zero(t, f.users.lastLogins[f.bob.ID])
	assert.Empty(t, f.audit.taken())
}

func TestOIDCSignInAdministrationRoutesRefuseAProvisioningTheyCannotStore(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	domainsRule := "provisioning.emailDomains must hold 1 to 20 domain names, such as example.com, when provisioning is enabled"
	for _, tc := range []struct {
		provisioning map[string]interface{}
		message      string
	}{
		{map[string]interface{}{"enabled": true, "role": "admin", "emailDomains": []string{"example.com"}},
			"provisioning.role must be viewer, member or manager"},
		{map[string]interface{}{"enabled": true, "emailDomains": []string{}}, domainsRule},
		{map[string]interface{}{"enabled": true}, domainsRule},
		{map[string]interface{}{"enabled": true, "emailDomains": []string{"*.example.com"}}, domainsRule},
	} {
		status, answer := f.putProvisioning(t, tc.provisioning)
		assert.Equal(t, fiber.StatusBadRequest, status, "%v", tc.provisioning)
		assert.Equal(t, tc.message, answer["error"], "%v", tc.provisioning)
	}
	assert.Empty(t, f.audit.taken(), "a refused configuration writes no audit row")

	// The stored configuration is still the fixture's, which provisions no user.
	adminA := map[string]string{"X-Test-Org": f.orgA.String(), "X-Test-User": f.alice.ID.String()}
	status, answer := f.do(t, fiber.MethodGet, "/api/v1/admin/oidc-sign-in", nil, adminA)
	require.Equal(t, fiber.StatusOK, status)
	assert.JSONEq(t, `{"enabled":false}`, asJSON(t, answer["provisioning"]))

	// A body without provisioning stores a configuration that provisions none.
	status, answer = f.do(t, fiber.MethodPut, "/api/v1/admin/oidc-sign-in", map[string]interface{}{
		"issuer": f.stub.Issuer, "clientId": f.stub.ClientID, "clientSecret": f.stub.ClientSecret,
	}, adminA)
	require.Equal(t, fiber.StatusOK, status, "%v", answer)
	assert.JSONEq(t, `{"enabled":false}`, asJSON(t, answer["provisioning"]))
}
