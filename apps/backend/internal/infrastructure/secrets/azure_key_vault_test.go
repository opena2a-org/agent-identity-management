package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainsecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

// mockAzureSecretsClient implements AzureSecretsAPI for testing.
type mockAzureSecretsClient struct {
	secrets map[string]string // name -> value
}

func newMockAzureSecretsClient() *mockAzureSecretsClient {
	return &mockAzureSecretsClient{secrets: make(map[string]string)}
}

func (m *mockAzureSecretsClient) SetSecret(_ context.Context, name string, params azsecrets.SetSecretParameters, _ *azsecrets.SetSecretOptions) (azsecrets.SetSecretResponse, error) {
	if params.Value == nil {
		return azsecrets.SetSecretResponse{}, fmt.Errorf("value is nil")
	}
	m.secrets[name] = *params.Value
	return azsecrets.SetSecretResponse{}, nil
}

func (m *mockAzureSecretsClient) GetSecret(_ context.Context, name string, _ string, _ *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error) {
	val, ok := m.secrets[name]
	if !ok {
		return azsecrets.GetSecretResponse{}, fmt.Errorf("secret not found")
	}
	return azsecrets.GetSecretResponse{
		Secret: azsecrets.Secret{
			Value: &val,
		},
	}, nil
}

func (m *mockAzureSecretsClient) DeleteSecret(_ context.Context, name string, _ *azsecrets.DeleteSecretOptions) (azsecrets.DeleteSecretResponse, error) {
	if _, ok := m.secrets[name]; !ok {
		return azsecrets.DeleteSecretResponse{}, fmt.Errorf("secret not found")
	}
	delete(m.secrets, name)
	return azsecrets.DeleteSecretResponse{}, nil
}

func TestAzureKeyVaultConfigFromMap(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg, err := AzureKeyVaultConfigFromMap(map[string]interface{}{
			"vaultUrl": "https://myvault.vault.azure.net",
		})
		require.NoError(t, err)
		assert.Equal(t, "https://myvault.vault.azure.net", cfg.VaultURL)
	})

	t.Run("missing vault URL", func(t *testing.T) {
		_, err := AzureKeyVaultConfigFromMap(map[string]interface{}{})
		assert.Error(t, err)
	})
}

func TestAzureKeyVaultBackendType(t *testing.T) {
	backend := NewAzureKeyVaultBackendFromClient(newMockAzureSecretsClient())
	assert.Equal(t, domainsecrets.BackendTypeAzureKV, backend.Type())
}

func TestAzureKeyVaultBackendCRUD(t *testing.T) {
	client := newMockAzureSecretsClient()
	backend := NewAzureKeyVaultBackendFromClient(client)
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

func TestAzureKeyVaultBackendRetrieveNotFound(t *testing.T) {
	backend := NewAzureKeyVaultBackendFromClient(newMockAzureSecretsClient())
	_, err := backend.Retrieve(uuid.New())
	assert.Error(t, err)
}

func TestAzurePayloadJSON(t *testing.T) {
	payload := azurePayload{
		Blob:          []byte("test-blob"),
		EncryptionAlg: "AES-256-GCM",
		Version:       3,
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var decoded azurePayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, payload.Blob, decoded.Blob)
	assert.Equal(t, payload.EncryptionAlg, decoded.EncryptionAlg)
	assert.Equal(t, payload.Version, decoded.Version)
}
