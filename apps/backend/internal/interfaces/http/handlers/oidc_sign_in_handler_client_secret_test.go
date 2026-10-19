package handlers

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A body that names the stored provider's issuer and client id may leave
// clientSecret out, as the dashboard's settings page does when the
// administrator changes only the provisioning or the requirement: the stored
// secret is kept, sign-in goes on with it, and the audit row says it was
// kept. A body that names another client id needs its secret, and its
// refusal changes nothing and writes no row.
func TestOIDCSignInAdministrationRouteKeepsTheStoredClientSecret(t *testing.T) {
	f := newOIDCHandlerFixture(t)
	adminA := map[string]string{"X-Test-Org": f.orgA.String(), "X-Test-User": uuid.NewString()}

	status, answer := f.do(t, fiber.MethodPut, "/api/v1/admin/oidc-sign-in", map[string]interface{}{
		"issuer": f.stub.Issuer, "clientId": f.stub.ClientID, "requireProviderSignIn": true,
	}, adminA)
	require.Equal(t, fiber.StatusOK, status, "%v", answer)
	assert.Equal(t, true, answer["clientSecretSet"])
	assert.Equal(t, true, answer["requireProviderSignIn"])
	rows := f.audit.taken()
	require.Len(t, rows, 1)
	assert.Equal(t, true, rows[0].Metadata["clientSecretKept"])
	assert.Equal(t, true, rows[0].Metadata["requireProviderSignIn"])

	// Alice, a member of A, signs in through the stub, whose token endpoint
	// takes only the secret it issued.
	status, signedIn := f.signIn(t, f.orgA, verifiedIdentity("alice@example.com"))
	require.Equal(t, fiber.StatusOK, status, "%v", signedIn)
	_ = f.audit.taken() // the login row of Alice's sign-in

	status, answer = f.do(t, fiber.MethodPut, "/api/v1/admin/oidc-sign-in", map[string]interface{}{
		"issuer": f.stub.Issuer, "clientId": "another-client", "clientSecret": "",
	}, adminA)
	assert.Equal(t, fiber.StatusBadRequest, status)
	assert.Equal(t, "clientSecret is required, at most 2048 characters; it may be left out only to keep the client secret "+
		"stored with the same issuer and clientId", answer["error"])
	assert.Empty(t, f.audit.taken(), "a refused body writes no row")
	status, answer = f.do(t, fiber.MethodGet, "/api/v1/admin/oidc-sign-in", nil, adminA)
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, f.stub.ClientID, answer["clientId"])
	assert.Equal(t, true, answer["requireProviderSignIn"])

	status, answer = f.do(t, fiber.MethodPut, "/api/v1/admin/oidc-sign-in", map[string]interface{}{
		"issuer": f.stub.Issuer, "clientId": f.stub.ClientID, "clientSecret": f.stub.ClientSecret,
	}, adminA)
	require.Equal(t, fiber.StatusOK, status, "%v", answer)
	rows = f.audit.taken()
	require.Len(t, rows, 1)
	assert.Equal(t, false, rows[0].Metadata["clientSecretKept"], "a body with the secret stores it")
}
