package secrets_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainsecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/secrets"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/secrets/azkvtest"
)

// stubTokenCredential hands the azsecrets client a token built at runtime and
// records what the client asked for after reading the vault's 401 challenge.
// It never calls Microsoft Entra.
type stubTokenCredential struct {
	token string

	mu       sync.Mutex
	requests []policy.TokenRequestOptions
}

func (c *stubTokenCredential) GetToken(_ context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	c.mu.Lock()
	c.requests = append(c.requests, opts)
	c.mu.Unlock()
	return azcore.AccessToken{Token: c.token, ExpiresOn: time.Now().Add(time.Hour)}, nil
}

func (c *stubTokenCredential) tokenRequests() []policy.TokenRequestOptions {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]policy.TokenRequestOptions(nil), c.requests...)
}

// newStubKeyVaultClient builds the real azsecrets client against the stub.
// The stub listens on 127.0.0.1, so the check that the challenge's resource
// (https://vault.azure.net) is a parent domain of the vault host is turned off
// with DisableChallengeResourceVerification.
func newStubKeyVaultClient(t *testing.T, vault *azkvtest.Server, cred azcore.TokenCredential) *azsecrets.Client {
	t.Helper()
	client, err := azsecrets.NewClient(vault.URL, cred, &azsecrets.ClientOptions{
		ClientOptions:                        azcore.ClientOptions{Transport: vault.Client()},
		DisableChallengeResourceVerification: true,
	})
	require.NoError(t, err)
	return client
}

func TestAzureKeyVaultBackendRoundTripThroughAzsecretsClient(t *testing.T) {
	token := azkvtest.NewToken()
	vault := azkvtest.New(t, token)
	cred := &stubTokenCredential{token: token}
	backend := secrets.NewAzureKeyVaultBackendFromClient(newStubKeyVaultClient(t, vault, cred))

	nsID := uuid.New()
	secretPath := "/secrets/aim-secrets-" + nsID.String()
	// Ciphertext is arbitrary bytes, including bytes that are not valid UTF-8.
	stored := []byte{0x00, 0xff, 0xfe, 0xc3, 0x28, 'a', 'i', 'm', 0x80, 0x7f}
	rotated := []byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x01, 0xf0, 0x9f}
	const alg = "X25519-ChaCha20-Poly1305"

	require.NoError(t, backend.Store(nsID, stored, alg))

	got, err := backend.Retrieve(nsID)
	require.NoError(t, err)
	assert.Equal(t, nsID, got.NamespaceID)
	assert.Equal(t, stored, got.EncryptedBlob, "stored bytes must round-trip exactly")
	assert.Equal(t, alg, got.EncryptionAlg)
	assert.Equal(t, 1, got.Version)

	require.NoError(t, backend.Rotate(nsID, rotated, alg))
	got, err = backend.Retrieve(nsID)
	require.NoError(t, err)
	assert.Equal(t, rotated, got.EncryptedBlob, "Retrieve after Rotate must return the new value")
	assert.Equal(t, 2, got.Version)

	require.NoError(t, backend.Delete(nsID))
	_, err = backend.Retrieve(nsID)
	require.Error(t, err)
	assert.NotErrorIs(t, err, domainsecrets.ErrBackendUnavailable,
		"a vault that answers 404 was reached; the error must not say the backend is unavailable")
	assert.EqualError(t, err, "no credential found in azure for namespace "+nsID.String())

	// The wire, as the stub saw it. The first request carries no token and is
	// answered with the 401 challenge; the client retries it with a bearer.
	reqs := vault.Requests()
	require.NotEmpty(t, reqs)
	assert.Equal(t, http.MethodPut, reqs[0].Method)
	assert.False(t, reqs[0].HadAuthorization)
	assert.Equal(t, http.StatusUnauthorized, reqs[0].Status)

	type call struct {
		method string
		path   string
		status int
	}
	var calls []call
	for _, r := range reqs[1:] {
		assert.True(t, r.Authorized, "%s %s after the challenge must carry the bearer", r.Method, r.Path)
		assert.NotEmpty(t, r.APIVersion, "%s %s must carry api-version", r.Method, r.Path)
		calls = append(calls, call{r.Method, strings.TrimSuffix(r.Path, "/"), r.Status})
	}
	assert.Equal(t, []call{
		{http.MethodPut, secretPath, http.StatusOK},       // Store
		{http.MethodGet, secretPath, http.StatusOK},       // Retrieve
		{http.MethodGet, secretPath, http.StatusOK},       // Rotate reads the current version
		{http.MethodPut, secretPath, http.StatusOK},       // Rotate writes a new version
		{http.MethodGet, secretPath, http.StatusOK},       // Retrieve
		{http.MethodDelete, secretPath, http.StatusOK},    // Delete
		{http.MethodGet, secretPath, http.StatusNotFound}, // Retrieve after Delete
	}, calls)

	// The client asked for a token for the resource and tenant the challenge named.
	trs := cred.tokenRequests()
	require.NotEmpty(t, trs)
	for _, tr := range trs {
		assert.Equal(t, []string{azkvtest.Resource + "/.default"}, tr.Scopes)
		assert.Equal(t, vault.TenantID, tr.TenantID)
	}
}

func TestAzureKeyVaultRoundTripStubRefusesRequestWithoutCredentials(t *testing.T) {
	token := azkvtest.NewToken()
	vault := azkvtest.New(t, token)

	for _, tc := range []struct {
		name          string
		authorization string
	}{
		{"no Authorization header", ""},
		{"a bearer the vault does not accept", "Bearer " + azkvtest.NewToken()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, vault.URL+"/secrets/any-name?api-version=7.6", nil)
			require.NoError(t, err)
			req.Header.Set("Accept", "application/json")
			if tc.authorization != "" {
				req.Header.Set("Authorization", tc.authorization)
			}
			resp, err := vault.Client().Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			challenge := resp.Header.Get("WWW-Authenticate")
			assert.True(t, strings.HasPrefix(challenge, "Bearer "), "challenge %q", challenge)
			assert.Contains(t, challenge, `authorization="`+azkvtest.AuthorityHost+"/"+vault.TenantID+`"`)
			assert.Contains(t, challenge, `resource="`+azkvtest.Resource+`"`)

			var body struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
			assert.NotEmpty(t, body.Error.Code)
			assert.NotEmpty(t, body.Error.Message)
		})
	}

	// The backend surfaces the refusal when its credential's token is not accepted.
	wrong := &stubTokenCredential{token: azkvtest.NewToken()}
	backend := secrets.NewAzureKeyVaultBackendFromClient(newStubKeyVaultClient(t, vault, wrong))
	err := backend.Store(uuid.New(), []byte{0x01}, "X25519-ChaCha20-Poly1305")
	require.Error(t, err)
	assert.ErrorIs(t, err, domainsecrets.ErrBackendUnavailable)
	assert.Contains(t, err.Error(), "401")
}

func TestAzureKeyVaultRoundTripFailsWhenTheClientDoesNotTrustTheVault(t *testing.T) {
	token := azkvtest.NewToken()
	vault := azkvtest.New(t, token)
	cred := &stubTokenCredential{token: token}

	// A TLS client with an empty root pool does not trust the stub's certificate.
	untrusting := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: x509.NewCertPool(), MinVersion: tls.VersionTLS12},
	}}
	client, err := azsecrets.NewClient(vault.URL, cred, &azsecrets.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Transport: untrusting,
			Retry:     policy.RetryOptions{MaxRetries: -1},
		},
		DisableChallengeResourceVerification: true,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = client.GetSecret(ctx, "aim-secrets-"+uuid.NewString(), "", nil)
	require.Error(t, err)
	var verifyErr *tls.CertificateVerificationError
	assert.True(t, errors.As(err, &verifyErr), "want a certificate verification error, got %v", err)

	backend := secrets.NewAzureKeyVaultBackendFromClient(client)
	err = backend.Store(uuid.New(), []byte{0x01}, "X25519-ChaCha20-Poly1305")
	require.Error(t, err)
	assert.ErrorIs(t, err, domainsecrets.ErrBackendUnavailable)

	assert.Empty(t, vault.Requests(), "no HTTP request may reach a vault the client does not trust")
	assert.Empty(t, cred.tokenRequests(), "no token may be requested for a vault the client does not trust")
}

func TestAzureKeyVaultRoundTripChallengeResourceIsVerifiedByDefault(t *testing.T) {
	token := azkvtest.NewToken()
	vault := azkvtest.New(t, token)
	cred := &stubTokenCredential{token: token}

	// Without DisableChallengeResourceVerification the client refuses a
	// challenge whose resource is not a parent domain of the vault host, and
	// sends no token.
	client, err := azsecrets.NewClient(vault.URL, cred, &azsecrets.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Transport: vault.Client(),
			Retry:     policy.RetryOptions{MaxRetries: -1},
		},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = client.GetSecret(ctx, "aim-secrets-"+uuid.NewString(), "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), azkvtest.Resource)
	assert.Contains(t, err.Error(), "doesn't match the requested domain")

	reqs := vault.Requests()
	require.Len(t, reqs, 1)
	assert.False(t, reqs[0].HadAuthorization)
	assert.Equal(t, http.StatusUnauthorized, reqs[0].Status)
	assert.Empty(t, cred.tokenRequests())
}
