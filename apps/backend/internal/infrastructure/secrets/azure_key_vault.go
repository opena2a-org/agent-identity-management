package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/google/uuid"

	domainsecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

const azureTimeout = 30 * time.Second

// AzureKeyVaultConfig holds the configuration for Azure Key Vault backend.
type AzureKeyVaultConfig struct {
	// VaultURL is the Azure Key Vault URL (e.g., "https://myvault.vault.azure.net").
	VaultURL string `json:"vaultUrl"`
}

// AzureKeyVaultConfigFromMap parses an AzureKeyVaultConfig from a generic config map.
func AzureKeyVaultConfigFromMap(m map[string]interface{}) (*AzureKeyVaultConfig, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	var cfg AzureKeyVaultConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse azure key vault config: %w", err)
	}
	if cfg.VaultURL == "" {
		return nil, fmt.Errorf("azure key vault URL is required")
	}
	return &cfg, nil
}

// AzureSecretsAPI abstracts the Azure Key Vault secrets client for testability.
type AzureSecretsAPI interface {
	SetSecret(ctx context.Context, name string, params azsecrets.SetSecretParameters, options *azsecrets.SetSecretOptions) (azsecrets.SetSecretResponse, error)
	GetSecret(ctx context.Context, name string, version string, options *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error)
	DeleteSecret(ctx context.Context, name string, options *azsecrets.DeleteSecretOptions) (azsecrets.DeleteSecretResponse, error)
}

// AzureKeyVaultBackend implements SecretsBackend using Azure Key Vault.
// No business logic here (CR-009) — just maps CRUD to Azure Key Vault API.
type AzureKeyVaultBackend struct {
	client AzureSecretsAPI
}

// NewAzureKeyVaultBackend creates a backend using DefaultAzureCredential (workload identity).
func NewAzureKeyVaultBackend(cfg *AzureKeyVaultConfig) (*AzureKeyVaultBackend, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create azure credential: %w", err)
	}

	client, err := azsecrets.NewClient(cfg.VaultURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create azure key vault client: %w", err)
	}

	return &AzureKeyVaultBackend{client: client}, nil
}

// NewAzureKeyVaultBackendFromClient creates a backend from an existing client (for testing).
func NewAzureKeyVaultBackendFromClient(client AzureSecretsAPI) *AzureKeyVaultBackend {
	return &AzureKeyVaultBackend{client: client}
}

// secretName returns an Azure-safe secret name for a namespace.
// Azure Key Vault secret names allow alphanumerics and hyphens only.
func azureSecretName(namespaceID uuid.UUID) string {
	return fmt.Sprintf("aim-secrets-%s", namespaceID.String())
}

// azurePayload is the JSON structure stored as the secret value.
type azurePayload struct {
	Blob          []byte `json:"blob"`
	EncryptionAlg string `json:"encryptionAlg"`
	Version       int    `json:"version"`
}

func (b *AzureKeyVaultBackend) Store(namespaceID uuid.UUID, blob []byte, encryptionAlg string) error {
	ctx, cancel := context.WithTimeout(context.Background(), azureTimeout)
	defer cancel()

	payload := azurePayload{
		Blob:          blob,
		EncryptionAlg: encryptionAlg,
		Version:       1,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	value := string(payloadBytes)
	_, err = b.client.SetSecret(ctx, azureSecretName(namespaceID), azsecrets.SetSecretParameters{
		Value: &value,
	}, nil)
	if err != nil {
		return fmt.Errorf("%w: azure store failed: %v", domainsecrets.ErrBackendUnavailable, err)
	}
	return nil
}

func (b *AzureKeyVaultBackend) Retrieve(namespaceID uuid.UUID) (*domainsecrets.SecretCredential, error) {
	ctx, cancel := context.WithTimeout(context.Background(), azureTimeout)
	defer cancel()

	resp, err := b.client.GetSecret(ctx, azureSecretName(namespaceID), "", nil)
	if err != nil {
		return nil, fmt.Errorf("%w: azure retrieve failed: %v", domainsecrets.ErrBackendUnavailable, err)
	}
	if resp.Value == nil {
		return nil, fmt.Errorf("no credential found in azure for namespace %s", namespaceID)
	}

	var payload azurePayload
	if err := json.Unmarshal([]byte(*resp.Value), &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal azure secret payload: %w", err)
	}

	return &domainsecrets.SecretCredential{
		NamespaceID:   namespaceID,
		EncryptedBlob: payload.Blob,
		EncryptionAlg: payload.EncryptionAlg,
		Version:       payload.Version,
	}, nil
}

func (b *AzureKeyVaultBackend) Rotate(namespaceID uuid.UUID, blob []byte, encryptionAlg string) error {
	ctx, cancel := context.WithTimeout(context.Background(), azureTimeout)
	defer cancel()

	current, err := b.Retrieve(namespaceID)
	nextVersion := 1
	if err == nil && current != nil {
		nextVersion = current.Version + 1
	}

	payload := azurePayload{
		Blob:          blob,
		EncryptionAlg: encryptionAlg,
		Version:       nextVersion,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	value := string(payloadBytes)
	_, err = b.client.SetSecret(ctx, azureSecretName(namespaceID), azsecrets.SetSecretParameters{
		Value: &value,
	}, nil)
	if err != nil {
		return fmt.Errorf("%w: azure rotate failed: %v", domainsecrets.ErrBackendUnavailable, err)
	}
	return nil
}

func (b *AzureKeyVaultBackend) Delete(namespaceID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), azureTimeout)
	defer cancel()

	_, err := b.client.DeleteSecret(ctx, azureSecretName(namespaceID), nil)
	if err != nil {
		return fmt.Errorf("%w: azure delete failed: %v", domainsecrets.ErrBackendUnavailable, err)
	}
	return nil
}

func (b *AzureKeyVaultBackend) Type() domainsecrets.BackendType {
	return domainsecrets.BackendTypeAzureKV
}
