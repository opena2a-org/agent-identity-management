package secrets_test

import (
	"context"
	"testing"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	domainsecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/secrets"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/secrets/gcpsmtest"
)

// stubBearer sends "authorization: Bearer <token>" on every call. The stub is
// plaintext on 127.0.0.1, so it does not require transport security.
type stubBearer struct{ token string }

func (b stubBearer) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + b.token}, nil
}

func (stubBearer) RequireTransportSecurity() bool { return false }

// newStubSecretManagerClient builds the real Secret Manager client against the
// stub: its endpoint, no Google credentials, a plaintext connection, and, when
// token is not empty, a bearer on every call.
func newStubSecretManagerClient(t *testing.T, srv *gcpsmtest.Server, token string) *secretmanager.Client {
	t.Helper()
	opts := []option.ClientOption{
		option.WithEndpoint(srv.Addr),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}
	if token != "" {
		opts = append(opts, option.WithGRPCDialOption(grpc.WithPerRPCCredentials(stubBearer{token: token})))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := secretmanager.NewClient(ctx, opts...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestGCPSecretBackendRoundTripThroughSecretManagerClient(t *testing.T) {
	token := gcpsmtest.NewToken()
	srv := gcpsmtest.New(t, token)
	const project = "aim-stub-project"
	backend := secrets.NewGCPSecretBackendFromClient(newStubSecretManagerClient(t, srv, token), project)

	nsID := uuid.New()
	secretName := "projects/" + project + "/secrets/aim-secrets-" + nsID.String()
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
		"a service that answers NOT_FOUND was reached; the error must not say the backend is unavailable")
	assert.EqualError(t, err, "no credential found in gcp for namespace "+nsID.String())

	type call struct {
		rpc      string
		resource string
		code     codes.Code
	}
	var calls []call
	for _, r := range srv.Requests() {
		assert.True(t, r.Authorized, "%s must carry the bearer", r.RPC())
		calls = append(calls, call{r.RPC(), r.Resource, r.Code})
	}
	latest := secretName + "/versions/latest"
	assert.Equal(t, []call{
		{"CreateSecret", "projects/" + project, codes.OK}, // Store creates the secret
		{"AddSecretVersion", secretName, codes.OK},        // Store adds version 1
		{"AccessSecretVersion", latest, codes.OK},         // Retrieve
		{"AccessSecretVersion", latest, codes.OK},         // Rotate reads the current version
		{"AddSecretVersion", secretName, codes.OK},        // Rotate adds version 2
		{"AccessSecretVersion", latest, codes.OK},         // Retrieve
		{"DeleteSecret", secretName, codes.OK},            // Delete
		{"AccessSecretVersion", latest, codes.NotFound},   // Retrieve after Delete
	}, calls)
}

func TestGCPSecretManagerRoundTripStubRefusesRequestWithoutCredentials(t *testing.T) {
	token := gcpsmtest.NewToken()
	srv := gcpsmtest.New(t, token)
	const project = "aim-stub-project"

	for _, tc := range []struct {
		name  string
		token string
	}{
		{"no authorization metadata", ""},
		{"a bearer the service does not accept", gcpsmtest.NewToken()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := newStubSecretManagerClient(t, srv, tc.token)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
				Name: "projects/" + project + "/secrets/any/versions/latest",
			})
			require.Error(t, err)
			assert.Equal(t, codes.Unauthenticated, status.Code(err), "got %v", err)

			backend := secrets.NewGCPSecretBackendFromClient(client, project)
			err = backend.Store(uuid.New(), []byte{0x01}, "X25519-ChaCha20-Poly1305")
			require.Error(t, err)
			assert.ErrorIs(t, err, domainsecrets.ErrBackendUnavailable)
			assert.Contains(t, err.Error(), "Unauthenticated")
		})
	}

	reqs := srv.Requests()
	require.NotEmpty(t, reqs)
	for _, r := range reqs {
		assert.False(t, r.Authorized)
		assert.Equal(t, codes.Unauthenticated, r.Code, "%s", r.RPC())
	}
}
