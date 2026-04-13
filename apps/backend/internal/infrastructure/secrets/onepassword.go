package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/1Password/connect-sdk-go/connect"
	"github.com/1Password/connect-sdk-go/onepassword"
	"github.com/google/uuid"

	domainsecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

const onePasswordTimeout = 30 * time.Second

// OnePasswordConfig holds the configuration for 1Password Secrets Automation backend.
type OnePasswordConfig struct {
	// ConnectHost is the 1Password Connect server URL.
	ConnectHost string `json:"connectHost"`

	// ConnectToken is the API token for 1Password Connect.
	ConnectToken string `json:"connectToken"`

	// VaultUUID is the 1Password vault to store secrets in.
	VaultUUID string `json:"vaultUuid"`
}

// OnePasswordConfigFromMap parses a OnePasswordConfig from a generic config map.
func OnePasswordConfigFromMap(m map[string]interface{}) (*OnePasswordConfig, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	var cfg OnePasswordConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse 1password config: %w", err)
	}
	if cfg.ConnectHost == "" {
		return nil, fmt.Errorf("1password connect host is required")
	}
	if cfg.ConnectToken == "" {
		return nil, fmt.Errorf("1password connect token is required")
	}
	if cfg.VaultUUID == "" {
		return nil, fmt.Errorf("1password vault UUID is required")
	}
	return &cfg, nil
}

// OnePasswordAPI abstracts the 1Password Connect client for testability.
type OnePasswordAPI interface {
	GetItem(itemUUID, vaultUUID string) (*onepassword.Item, error)
	CreateItem(item *onepassword.Item, vaultUUID string) (*onepassword.Item, error)
	UpdateItem(item *onepassword.Item, vaultUUID string) (*onepassword.Item, error)
	DeleteItem(item *onepassword.Item, vaultUUID string) error
	GetItemsByTitle(title string, vaultUUID string) ([]onepassword.Item, error)
}

// OnePasswordBackend implements SecretsBackend using 1Password Secrets Automation.
// No business logic here (CR-009) — just maps CRUD to 1Password Connect API.
type OnePasswordBackend struct {
	client    OnePasswordAPI
	vaultUUID string
}

// NewOnePasswordBackend creates a backend from the given config.
func NewOnePasswordBackend(cfg *OnePasswordConfig) (*OnePasswordBackend, error) {
	client := connect.NewClientWithUserAgent(cfg.ConnectHost, cfg.ConnectToken, "aim-secrets/1.0")
	return &OnePasswordBackend{
		client:    client,
		vaultUUID: cfg.VaultUUID,
	}, nil
}

// NewOnePasswordBackendFromClient creates a backend from an existing client (for testing).
func NewOnePasswordBackendFromClient(client OnePasswordAPI, vaultUUID string) *OnePasswordBackend {
	return &OnePasswordBackend{
		client:    client,
		vaultUUID: vaultUUID,
	}
}

// itemTitle returns a deterministic 1Password item title for a namespace.
func itemTitle(namespaceID uuid.UUID) string {
	return fmt.Sprintf("aim-secrets/%s", namespaceID.String())
}

func (b *OnePasswordBackend) Store(namespaceID uuid.UUID, blob []byte, encryptionAlg string) error {
	_, cancel := context.WithTimeout(context.Background(), onePasswordTimeout)
	defer cancel()

	item := &onepassword.Item{
		Title:    itemTitle(namespaceID),
		Category: onepassword.Password,
		Fields: []*onepassword.ItemField{
			{Label: "blob", Value: string(blob), Type: "CONCEALED"},
			{Label: "encryptionAlg", Value: encryptionAlg, Type: "STRING"},
			{Label: "version", Value: "1", Type: "STRING"},
		},
	}

	_, err := b.client.CreateItem(item, b.vaultUUID)
	if err != nil {
		return fmt.Errorf("%w: 1password store failed: %v", domainsecrets.ErrBackendUnavailable, err)
	}
	return nil
}

func (b *OnePasswordBackend) Retrieve(namespaceID uuid.UUID) (*domainsecrets.SecretCredential, error) {
	_, cancel := context.WithTimeout(context.Background(), onePasswordTimeout)
	defer cancel()

	items, err := b.client.GetItemsByTitle(itemTitle(namespaceID), b.vaultUUID)
	if err != nil {
		return nil, fmt.Errorf("%w: 1password retrieve failed: %v", domainsecrets.ErrBackendUnavailable, err)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no credential found in 1password for namespace %s", namespaceID)
	}

	item, err := b.client.GetItem(items[0].ID, b.vaultUUID)
	if err != nil {
		return nil, fmt.Errorf("%w: 1password get item failed: %v", domainsecrets.ErrBackendUnavailable, err)
	}

	var blobVal, encAlg string
	version := 1
	for _, f := range item.Fields {
		switch f.Label {
		case "blob":
			blobVal = f.Value
		case "encryptionAlg":
			encAlg = f.Value
		case "version":
			version = interfaceToInt(f.Value)
		}
	}

	return &domainsecrets.SecretCredential{
		NamespaceID:   namespaceID,
		EncryptedBlob: []byte(blobVal),
		EncryptionAlg: encAlg,
		Version:       version,
	}, nil
}

func (b *OnePasswordBackend) Rotate(namespaceID uuid.UUID, blob []byte, encryptionAlg string) error {
	_, cancel := context.WithTimeout(context.Background(), onePasswordTimeout)
	defer cancel()

	current, err := b.Retrieve(namespaceID)
	nextVersion := 1
	if err == nil && current != nil {
		nextVersion = current.Version + 1
	}

	items, err := b.client.GetItemsByTitle(itemTitle(namespaceID), b.vaultUUID)
	if err != nil || len(items) == 0 {
		return fmt.Errorf("%w: 1password rotate lookup failed: %v", domainsecrets.ErrBackendUnavailable, err)
	}

	item, err := b.client.GetItem(items[0].ID, b.vaultUUID)
	if err != nil {
		return fmt.Errorf("%w: 1password get item failed: %v", domainsecrets.ErrBackendUnavailable, err)
	}

	item.Fields = []*onepassword.ItemField{
		{Label: "blob", Value: string(blob), Type: "CONCEALED"},
		{Label: "encryptionAlg", Value: encryptionAlg, Type: "STRING"},
		{Label: "version", Value: fmt.Sprintf("%d", nextVersion), Type: "STRING"},
	}

	_, err = b.client.UpdateItem(item, b.vaultUUID)
	if err != nil {
		return fmt.Errorf("%w: 1password rotate failed: %v", domainsecrets.ErrBackendUnavailable, err)
	}
	return nil
}

func (b *OnePasswordBackend) Delete(namespaceID uuid.UUID) error {
	_, cancel := context.WithTimeout(context.Background(), onePasswordTimeout)
	defer cancel()

	items, err := b.client.GetItemsByTitle(itemTitle(namespaceID), b.vaultUUID)
	if err != nil {
		return fmt.Errorf("%w: 1password delete lookup failed: %v", domainsecrets.ErrBackendUnavailable, err)
	}
	if len(items) == 0 {
		return nil // Already deleted
	}

	for _, item := range items {
		if err := b.client.DeleteItem(&item, b.vaultUUID); err != nil {
			return fmt.Errorf("%w: 1password delete failed: %v", domainsecrets.ErrBackendUnavailable, err)
		}
	}
	return nil
}

func (b *OnePasswordBackend) Type() domainsecrets.BackendType {
	return domainsecrets.BackendType1Password
}
