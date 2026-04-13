package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	gax "github.com/googleapis/gax-go/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainsecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

// mockGCPSecretManagerClient implements GCPSecretManagerAPI for testing.
type mockGCPSecretManagerClient struct {
	secrets  map[string]*secretmanagerpb.Secret        // name -> secret
	versions map[string]*secretmanagerpb.SecretPayload // name/versions/latest -> payload
}

func newMockGCPSecretManagerClient() *mockGCPSecretManagerClient {
	return &mockGCPSecretManagerClient{
		secrets:  make(map[string]*secretmanagerpb.Secret),
		versions: make(map[string]*secretmanagerpb.SecretPayload),
	}
}

func (m *mockGCPSecretManagerClient) CreateSecret(_ context.Context, req *secretmanagerpb.CreateSecretRequest, _ ...gax.CallOption) (*secretmanagerpb.Secret, error) {
	name := fmt.Sprintf("%s/secrets/%s", req.Parent, req.SecretId)
	if _, exists := m.secrets[name]; exists {
		return nil, fmt.Errorf("secret already exists")
	}
	secret := &secretmanagerpb.Secret{Name: name}
	m.secrets[name] = secret
	return secret, nil
}

func (m *mockGCPSecretManagerClient) AddSecretVersion(_ context.Context, req *secretmanagerpb.AddSecretVersionRequest, _ ...gax.CallOption) (*secretmanagerpb.SecretVersion, error) {
	if _, ok := m.secrets[req.Parent]; !ok {
		return nil, fmt.Errorf("secret not found")
	}
	latestKey := req.Parent + "/versions/latest"
	m.versions[latestKey] = req.Payload
	return &secretmanagerpb.SecretVersion{Name: latestKey}, nil
}

func (m *mockGCPSecretManagerClient) AccessSecretVersion(_ context.Context, req *secretmanagerpb.AccessSecretVersionRequest, _ ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	payload, ok := m.versions[req.Name]
	if !ok {
		return nil, fmt.Errorf("secret version not found")
	}
	return &secretmanagerpb.AccessSecretVersionResponse{
		Name:    req.Name,
		Payload: payload,
	}, nil
}

func (m *mockGCPSecretManagerClient) DeleteSecret(_ context.Context, req *secretmanagerpb.DeleteSecretRequest, _ ...gax.CallOption) error {
	if _, ok := m.secrets[req.Name]; !ok {
		return fmt.Errorf("secret not found")
	}
	latestKey := req.Name + "/versions/latest"
	delete(m.secrets, req.Name)
	delete(m.versions, latestKey)
	return nil
}

func TestGCPSecretConfigFromMap(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg, err := GCPSecretConfigFromMap(map[string]interface{}{
			"projectId": "my-project",
		})
		require.NoError(t, err)
		assert.Equal(t, "my-project", cfg.ProjectID)
	})

	t.Run("missing project ID", func(t *testing.T) {
		_, err := GCPSecretConfigFromMap(map[string]interface{}{})
		assert.Error(t, err)
	})
}

func TestGCPSecretBackendType(t *testing.T) {
	backend := NewGCPSecretBackendFromClient(newMockGCPSecretManagerClient(), "project")
	assert.Equal(t, domainsecrets.BackendTypeGCPSM, backend.Type())
}

func TestGCPSecretBackendCRUD(t *testing.T) {
	client := newMockGCPSecretManagerClient()
	backend := NewGCPSecretBackendFromClient(client, "my-project")
	nsID := uuid.New()

	// Store
	err := backend.Store(nsID, []byte("encrypted-data"), "X25519-ChaCha20")
	require.NoError(t, err)

	// Retrieve
	cred, err := backend.Retrieve(nsID)
	require.NoError(t, err)
	assert.Equal(t, nsID, cred.NamespaceID)
	assert.Equal(t, []byte("encrypted-data"), cred.EncryptedBlob)
	assert.Equal(t, "X25519-ChaCha20", cred.EncryptionAlg)
	assert.Equal(t, 1, cred.Version)

	// Rotate
	err = backend.Rotate(nsID, []byte("rotated-data"), "X25519-ChaCha20")
	require.NoError(t, err)

	cred, err = backend.Retrieve(nsID)
	require.NoError(t, err)
	assert.Equal(t, []byte("rotated-data"), cred.EncryptedBlob)
	assert.Equal(t, 2, cred.Version)

	// Delete
	err = backend.Delete(nsID)
	require.NoError(t, err)
}

func TestGCPSecretBackendRetrieveNotFound(t *testing.T) {
	backend := NewGCPSecretBackendFromClient(newMockGCPSecretManagerClient(), "project")
	_, err := backend.Retrieve(uuid.New())
	assert.Error(t, err)
}

func TestGCPPayloadJSON(t *testing.T) {
	payload := gcpPayload{
		Blob:          []byte("test-blob"),
		EncryptionAlg: "AES-256-GCM",
		Version:       5,
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var decoded gcpPayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, payload.Blob, decoded.Blob)
	assert.Equal(t, payload.EncryptionAlg, decoded.EncryptionAlg)
	assert.Equal(t, payload.Version, decoded.Version)
}
