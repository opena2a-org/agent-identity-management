package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/oauth"
)

// An administrator who changes the provisioning or the requirement of the
// stored provider, naming the same issuer and client id, may leave the client
// secret out: the stored secret is kept as it was sealed, and sign-in goes on
// with it.
func TestOIDCSignInConfigureKeepsTheStoredSecretForTheSameIssuerAndClient(t *testing.T) {
	f := newOIDCSignInFixture(t, func(a, _ uuid.UUID) []domain.User {
		return []domain.User{oidcTestUser(a, "alice@example.com")}
	})
	ctx := context.Background()
	before, err := f.service.Configuration(ctx, f.orgA)
	require.NoError(t, err)

	cfg, err := f.service.Configure(ctx, f.orgA, uuid.New(), OIDCSignInConfigInput{
		Issuer:                f.stub.Issuer,
		ClientID:              f.stub.ClientID,
		Provisioning:          OIDCProvisioningInput{Enabled: true, Role: "member", EmailDomains: []string{"example.com"}},
		RequireProviderSignIn: true,
	})
	require.NoError(t, err)
	assert.Equal(t, before.SealedClientSecret, cfg.SealedClientSecret, "the stored secret is kept as it was sealed")
	assert.True(t, cfg.RequireProviderSignIn)
	assert.Equal(t, domain.OIDCProvisioning{Enabled: true, Role: domain.RoleMember, EmailDomains: []string{"example.com"}}, cfg.Provisioning)

	// The stub's token endpoint takes only its own client secret, so a
	// completed sign-in shows the kept secret is the one it issued.
	user, err := f.signIn(t, f.orgA, verified("alice@example.com"))
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", user.Email)

	// A client secret of spaces is no secret, and is left out the same way.
	cfg, err = f.service.Configure(ctx, f.orgA, uuid.New(), OIDCSignInConfigInput{
		Issuer: f.stub.Issuer, ClientID: f.stub.ClientID, ClientSecret: "   ",
	})
	require.NoError(t, err)
	assert.Equal(t, before.SealedClientSecret, cfg.SealedClientSecret)
	assert.False(t, cfg.RequireProviderSignIn, "the body is stored in place of the earlier one")
	assert.Equal(t, domain.OIDCProvisioning{}, cfg.Provisioning)
}

// A client secret belongs to its issuer and client: a configuration that
// names another issuer or another client id, or an organization that has no
// stored provider, needs the secret, and a refused configuration changes
// nothing. A secret that is given replaces the stored one.
func TestOIDCSignInConfigureNeedsTheSecretOfAnotherIssuerOrClient(t *testing.T) {
	f := newOIDCSignInFixture(t, func(_, _ uuid.UUID) []domain.User { return nil })
	ctx := context.Background()
	before, err := f.service.Configuration(ctx, f.orgA)
	require.NoError(t, err)

	// The stub's client sends a request for an example.com host to the stub,
	// whose configuration document names the stub's own issuer: had the
	// issuer been asked first, this would be refused as another issuer's
	// document rather than for the missing secret.
	for name, in := range map[string]OIDCSignInConfigInput{
		"another client id": {Issuer: f.stub.Issuer, ClientID: "another-client"},
		"another issuer":    {Issuer: "https://other-idp.example.com", ClientID: f.stub.ClientID},
	} {
		_, err := f.service.Configure(ctx, f.orgA, uuid.New(), in)
		assert.ErrorIs(t, err, ErrOIDCClientSecretInvalid, name)
	}
	after, err := f.service.Configuration(ctx, f.orgA)
	require.NoError(t, err)
	assert.Equal(t, before, after, "the refused configurations changed nothing")
	_, err = f.service.Configure(ctx, f.orgA, uuid.New(), OIDCSignInConfigInput{
		Issuer: "https://other-idp.example.com", ClientID: f.stub.ClientID, ClientSecret: "not-a-real-client-secret",
	})
	assert.ErrorIs(t, err, oauth.ErrOIDCIssuerMismatch, "with a secret, that issuer is asked and refused for its document")

	// B has no stored provider, so it has no secret to keep, and A's is not B's.
	_, err = f.service.Configure(ctx, f.orgB, uuid.New(), OIDCSignInConfigInput{Issuer: f.stub.Issuer, ClientID: f.stub.ClientID})
	assert.ErrorIs(t, err, ErrOIDCClientSecretInvalid)
	_, err = f.service.Configuration(ctx, f.orgB)
	assert.ErrorIs(t, err, domain.ErrOIDCSignInNotConfigured)

	replaced, err := f.service.Configure(ctx, f.orgA, uuid.New(), OIDCSignInConfigInput{
		Issuer: f.stub.Issuer, ClientID: f.stub.ClientID, ClientSecret: "not-a-real-replacement-secret",
	})
	require.NoError(t, err)
	opened, err := f.service.sealer.OpenOIDCClientSecret(f.orgA, replaced.SealedClientSecret)
	require.NoError(t, err)
	assert.Equal(t, "not-a-real-replacement-secret", opened)
}
