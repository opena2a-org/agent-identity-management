package secrets_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/1Password/connect-sdk-go/connect"
	"github.com/1Password/connect-sdk-go/onepassword"
	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainsecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/secrets"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/secrets/onepasswordtest"
)

// newStubOnePasswordBackend builds the backend with its production
// constructor, which builds the Connect client library's REST client, against
// the stub. The client library installs a global Jaeger tracer when none is
// registered, whose sampler and reporter address local agent ports; a no-op
// tracer is registered first so the test reaches only the stub.
func newStubOnePasswordBackend(t *testing.T, srv *onepasswordtest.Server, token string) *secrets.OnePasswordBackend {
	t.Helper()
	opentracing.SetGlobalTracer(opentracing.NoopTracer{})
	backend, err := secrets.NewOnePasswordBackend(&secrets.OnePasswordConfig{
		ConnectHost:  srv.URL,
		ConnectToken: token,
		VaultUUID:    srv.VaultID,
	})
	require.NoError(t, err)
	return backend
}

func TestOnePasswordBackendRoundTripThroughConnectClient(t *testing.T) {
	token := onepasswordtest.NewToken()
	srv := onepasswordtest.New(t, token)
	backend := newStubOnePasswordBackend(t, srv, token)

	nsID := uuid.New()
	title := "aim-secrets/" + nsID.String()
	// Ciphertext is arbitrary bytes, including bytes that are not valid UTF-8.
	stored := []byte{0x00, 0xff, 0xfe, 0xc3, 0x28, 'a', 'i', 'm', 0x80, 0x7f}
	rotated := []byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x01, 0xf0, 0x9f}
	const alg = "X25519-ChaCha20-Poly1305"

	require.NoError(t, backend.Store(nsID, stored, alg))
	// A field value is a JSON string, so the vault holds the ciphertext base64-encoded.
	fields := srv.FieldValues(title)
	assert.Equal(t, base64.StdEncoding.EncodeToString(stored), fields["blob"])
	assert.Equal(t, "base64", fields["blobEncoding"])
	assert.Equal(t, alg, fields["encryptionAlg"])
	assert.Equal(t, "1", fields["version"])

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
	assert.NotErrorIs(t, err, domainsecrets.ErrBackendUnavailable)
	assert.EqualError(t, err, "no credential found in 1password for namespace "+nsID.String())
	assert.Nil(t, srv.FieldValues(title), "the item must be gone from the vault")

	// The wire, as the stub saw it.
	itemsPath := "/v1/vaults/" + srv.VaultID + "/items"
	counts := map[string]int{}
	for _, r := range srv.Requests() {
		assert.True(t, r.Authorized, "%s %s must carry the bearer", r.Method, r.Path)
		switch {
		case r.Method == http.MethodGet && r.Path == itemsPath:
			assert.Equal(t, `title eq "`+title+`"`, r.Filter)
			assert.Equal(t, http.StatusOK, r.Status)
			counts["list"]++
		case r.Method == http.MethodGet:
			assert.Equal(t, http.StatusOK, r.Status)
			counts["get"]++
		case r.Method == http.MethodPost && r.Path == itemsPath:
			assert.Equal(t, http.StatusOK, r.Status)
			counts["create"]++
		case r.Method == http.MethodPut:
			assert.Equal(t, http.StatusOK, r.Status)
			counts["replace"]++
		case r.Method == http.MethodDelete:
			assert.Equal(t, http.StatusNoContent, r.Status)
			counts["delete"]++
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.Path)
		}
	}
	assert.Equal(t, 1, counts["create"], "Store creates one item")
	assert.Equal(t, 1, counts["replace"], "Rotate replaces the item once")
	assert.Equal(t, 1, counts["delete"], "Delete deletes the one item")
	assert.Positive(t, counts["list"])
	assert.Positive(t, counts["get"])
}

func TestOnePasswordBackendRoundTripReadsTheBlobOfEachItemLayout(t *testing.T) {
	token := onepasswordtest.NewToken()
	srv := onepasswordtest.New(t, token)
	backend := newStubOnePasswordBackend(t, srv, token)

	// An item whose blob field holds the bytes as text and that has no
	// blobEncoding field, the layout the backend wrote before the blob was
	// base64-encoded.
	nsID := uuid.New()
	client := connect.NewClient(srv.URL, token)
	_, err := client.CreateItem(&onepassword.Item{
		Title:    "aim-secrets/" + nsID.String(),
		Category: onepassword.Password,
		Fields: []*onepassword.ItemField{
			{Label: "blob", Value: "legacy-ciphertext", Type: "CONCEALED"},
			{Label: "encryptionAlg", Value: "X25519-ChaCha20-Poly1305", Type: "STRING"},
			{Label: "version", Value: "3", Type: "STRING"},
		},
	}, srv.VaultID)
	require.NoError(t, err)

	got, err := backend.Retrieve(nsID)
	require.NoError(t, err)
	assert.Equal(t, []byte("legacy-ciphertext"), got.EncryptedBlob)
	assert.Equal(t, "X25519-ChaCha20-Poly1305", got.EncryptionAlg)
	assert.Equal(t, 3, got.Version)

	// A blobEncoding the backend does not know, and a base64 blob that does
	// not decode, are refused rather than read as text.
	for _, tc := range []struct {
		encoding, value, wantErr string
	}{
		{"hex", "6c6567616379", `unknown 1password blob encoding "hex"`},
		{"base64", "not base64!", "failed to decode 1password blob"},
	} {
		otherNS := uuid.New()
		_, err = client.CreateItem(&onepassword.Item{
			Title:    "aim-secrets/" + otherNS.String(),
			Category: onepassword.Password,
			Fields: []*onepassword.ItemField{
				{Label: "blob", Value: tc.value, Type: "CONCEALED"},
				{Label: "blobEncoding", Value: tc.encoding, Type: "STRING"},
			},
		}, srv.VaultID)
		require.NoError(t, err)
		_, err = backend.Retrieve(otherNS)
		require.Error(t, err, "blobEncoding %q", tc.encoding)
		assert.Contains(t, err.Error(), tc.wantErr)
	}
}

func TestOnePasswordConnectRoundTripStubRefusesRequestWithoutCredentials(t *testing.T) {
	token := onepasswordtest.NewToken()
	srv := onepasswordtest.New(t, token)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/vaults/"+srv.VaultID+"/items", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	var body struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, http.StatusUnauthorized, body.Status)
	assert.Equal(t, "Invalid or missing token", body.Message)

	reqs := srv.Requests()
	require.Len(t, reqs, 1)
	assert.False(t, reqs[0].HadAuthorization)
}

func TestOnePasswordConnectRoundTripStubRefusesAWrongBearer(t *testing.T) {
	srv := onepasswordtest.New(t, onepasswordtest.NewToken())
	backend := newStubOnePasswordBackend(t, srv, onepasswordtest.NewToken())
	nsID := uuid.New()

	err := backend.Store(nsID, []byte{0x01}, "X25519-ChaCha20-Poly1305")
	require.Error(t, err)
	assert.ErrorIs(t, err, domainsecrets.ErrBackendUnavailable)
	assert.Contains(t, err.Error(), "status 401: Invalid or missing token")

	_, err = backend.Retrieve(nsID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domainsecrets.ErrBackendUnavailable)
	assert.Contains(t, err.Error(), "status 401: Invalid or missing token")

	reqs := srv.Requests()
	require.Len(t, reqs, 2)
	for _, r := range reqs {
		assert.True(t, r.HadAuthorization)
		assert.False(t, r.Authorized)
		assert.Equal(t, http.StatusUnauthorized, r.Status)
	}
}
