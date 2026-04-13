package secrets

import (
	"encoding/json"
	"fmt"
	"strconv"

	"os"

	vault "github.com/hashicorp/vault/api"
	"github.com/google/uuid"

	domainsecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

// VaultConfig holds the configuration for HashiCorp Vault backend.
type VaultConfig struct {
	// Address is the Vault server URL (e.g., "https://vault.example.com:8200").
	Address string `json:"address"`

	// MountPath is the KV v2 mount path (default: "secret").
	MountPath string `json:"mountPath"`

	// AuthPath is the JWT/OIDC auth method path (default: "jwt").
	AuthPath string `json:"authPath"`

	// Role is the Vault role for JWT auth.
	Role string `json:"role"`

	// Token is a static token for authentication (alternative to JWT auth).
	// Used in development/testing. In production, prefer JWT auth.
	Token string `json:"token"`

	// Namespace is the Vault namespace (enterprise feature, optional).
	Namespace string `json:"namespace,omitempty"`
}

// VaultConfigFromMap parses a VaultConfig from a generic config map.
func VaultConfigFromMap(m map[string]interface{}) (*VaultConfig, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	var cfg VaultConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse vault config: %w", err)
	}
	if cfg.Address == "" {
		return nil, fmt.Errorf("vault address is required")
	}
	if cfg.MountPath == "" {
		cfg.MountPath = "secret"
	}
	if cfg.AuthPath == "" {
		cfg.AuthPath = "jwt"
	}
	return &cfg, nil
}

// HashiCorpVaultBackend implements SecretsBackend using HashiCorp Vault KV v2.
// No business logic here (CR-009) — just maps CRUD to Vault KV v2 API.
type HashiCorpVaultBackend struct {
	client    *vault.Client
	mountPath string
}

// NewHashiCorpVaultBackend creates a backend from the given config.
func NewHashiCorpVaultBackend(cfg *VaultConfig) (*HashiCorpVaultBackend, error) {
	vaultCfg := vault.DefaultConfig()
	vaultCfg.Address = cfg.Address

	client, err := vault.NewClient(vaultCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault client: %w", err)
	}

	if cfg.Namespace != "" {
		client.SetNamespace(cfg.Namespace)
	}

	// Authenticate: static token or JWT
	if cfg.Token != "" {
		client.SetToken(cfg.Token)
	} else if cfg.Role != "" {
		// JWT auth — read JWT from VAULT_JWT env var or Kubernetes service account token.
		jwt := os.Getenv("VAULT_JWT")
		if jwt == "" {
			// Fall back to Kubernetes-mounted service account token.
			if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token"); err == nil {
				jwt = string(data)
			}
		}
		if jwt == "" {
			return nil, fmt.Errorf("vault JWT auth requires VAULT_JWT env var or mounted service account token")
		}
		loginPath := fmt.Sprintf("auth/%s/login", cfg.AuthPath)
		secret, err := client.Logical().Write(loginPath, map[string]interface{}{
			"role": cfg.Role,
			"jwt":  jwt,
		})
		if err != nil {
			return nil, fmt.Errorf("vault JWT auth failed: %w", err)
		}
		if secret == nil || secret.Auth == nil {
			return nil, fmt.Errorf("vault JWT auth returned no token")
		}
		client.SetToken(secret.Auth.ClientToken)
	} else {
		return nil, fmt.Errorf("vault authentication required: provide either Token or Role")
	}

	return &HashiCorpVaultBackend{
		client:    client,
		mountPath: cfg.MountPath,
	}, nil
}

// NewHashiCorpVaultBackendFromClient creates a backend from an existing Vault client (for testing).
func NewHashiCorpVaultBackendFromClient(client *vault.Client, mountPath string) *HashiCorpVaultBackend {
	if mountPath == "" {
		mountPath = "secret"
	}
	return &HashiCorpVaultBackend{
		client:    client,
		mountPath: mountPath,
	}
}

// secretPath returns the KV v2 path for a namespace.
func (b *HashiCorpVaultBackend) secretPath(namespaceID uuid.UUID) string {
	return fmt.Sprintf("aim-secrets/%s", namespaceID.String())
}

func (b *HashiCorpVaultBackend) Store(namespaceID uuid.UUID, blob []byte, encryptionAlg string) error {
	path := b.secretPath(namespaceID)
	data := map[string]interface{}{
		"data": map[string]interface{}{
			"blob":          blob,
			"encryptionAlg": encryptionAlg,
			"version":       1,
		},
	}

	_, err := b.client.Logical().Write(
		fmt.Sprintf("%s/data/%s", b.mountPath, path),
		data,
	)
	if err != nil {
		return fmt.Errorf("vault store failed: %w", err)
	}
	return nil
}

func (b *HashiCorpVaultBackend) Retrieve(namespaceID uuid.UUID) (*domainsecrets.SecretCredential, error) {
	path := b.secretPath(namespaceID)

	secret, err := b.client.Logical().Read(
		fmt.Sprintf("%s/data/%s", b.mountPath, path),
	)
	if err != nil {
		return nil, fmt.Errorf("vault retrieve failed: %w", err)
	}
	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("no credential found in vault for namespace %s", namespaceID)
	}

	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected vault data format for namespace %s", namespaceID)
	}

	blob, err := interfaceToBytes(data["blob"])
	if err != nil {
		return nil, fmt.Errorf("failed to decode blob: %w", err)
	}

	encAlg, _ := data["encryptionAlg"].(string)

	version := 1
	if v, ok := data["version"]; ok {
		version = interfaceToInt(v)
	}

	return &domainsecrets.SecretCredential{
		NamespaceID:   namespaceID,
		EncryptedBlob: blob,
		EncryptionAlg: encAlg,
		Version:       version,
	}, nil
}

func (b *HashiCorpVaultBackend) Rotate(namespaceID uuid.UUID, blob []byte, encryptionAlg string) error {
	// Read current version
	current, err := b.Retrieve(namespaceID)
	nextVersion := 1
	if err == nil && current != nil {
		nextVersion = current.Version + 1
	}

	path := b.secretPath(namespaceID)
	data := map[string]interface{}{
		"data": map[string]interface{}{
			"blob":          blob,
			"encryptionAlg": encryptionAlg,
			"version":       nextVersion,
		},
	}

	_, err = b.client.Logical().Write(
		fmt.Sprintf("%s/data/%s", b.mountPath, path),
		data,
	)
	if err != nil {
		return fmt.Errorf("vault rotate failed: %w", err)
	}
	return nil
}

func (b *HashiCorpVaultBackend) Delete(namespaceID uuid.UUID) error {
	path := b.secretPath(namespaceID)

	// Delete all versions and metadata (permanent delete)
	_, err := b.client.Logical().Delete(
		fmt.Sprintf("%s/metadata/%s", b.mountPath, path),
	)
	if err != nil {
		return fmt.Errorf("vault delete failed: %w", err)
	}
	return nil
}

func (b *HashiCorpVaultBackend) Type() domainsecrets.BackendType {
	return domainsecrets.BackendTypeVault
}

// interfaceToBytes converts various vault response types to []byte.
func interfaceToBytes(v interface{}) ([]byte, error) {
	switch val := v.(type) {
	case []byte:
		return val, nil
	case string:
		// Vault KV v2 stores byte arrays as base64 strings in JSON responses.
		// The vault/api client auto-decodes, but we handle the string case.
		return []byte(val), nil
	case []interface{}:
		result := make([]byte, len(val))
		for i, item := range val {
			switch n := item.(type) {
			case float64:
				result[i] = byte(n)
			case json.Number:
				v, _ := n.Int64()
				result[i] = byte(v)
			default:
				return nil, fmt.Errorf("unexpected array element type at index %d: %T", i, item)
			}
		}
		return result, nil
	case nil:
		return nil, fmt.Errorf("blob is nil")
	default:
		return nil, fmt.Errorf("unexpected blob type: %T", v)
	}
}

// interfaceToInt converts a vault response value to int.
func interfaceToInt(v interface{}) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case json.Number:
		n, _ := val.Int64()
		return int(n)
	case int:
		return val
	case string:
		n, _ := strconv.Atoi(val)
		return n
	default:
		return 1
	}
}
