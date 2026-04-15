package secrets

import (
	"fmt"
	"testing"

	"github.com/1Password/connect-sdk-go/onepassword"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainsecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

// mockOnePasswordClient implements OnePasswordAPI for testing.
type mockOnePasswordClient struct {
	items map[string]*onepassword.Item // keyed by title
}

func newMockOnePasswordClient() *mockOnePasswordClient {
	return &mockOnePasswordClient{items: make(map[string]*onepassword.Item)}
}

func (m *mockOnePasswordClient) CreateItem(item *onepassword.Item, _ string) (*onepassword.Item, error) {
	if _, exists := m.items[item.Title]; exists {
		return nil, fmt.Errorf("item already exists")
	}
	item.ID = uuid.New().String()
	m.items[item.Title] = item
	return item, nil
}

func (m *mockOnePasswordClient) GetItem(itemUUID, _ string) (*onepassword.Item, error) {
	for _, item := range m.items {
		if item.ID == itemUUID {
			return item, nil
		}
	}
	return nil, fmt.Errorf("item not found")
}

func (m *mockOnePasswordClient) GetItemsByTitle(title string, _ string) ([]onepassword.Item, error) {
	if item, ok := m.items[title]; ok {
		return []onepassword.Item{*item}, nil
	}
	return nil, nil
}

func (m *mockOnePasswordClient) UpdateItem(item *onepassword.Item, _ string) (*onepassword.Item, error) {
	for title, existing := range m.items {
		if existing.ID == item.ID {
			m.items[title] = item
			return item, nil
		}
	}
	return nil, fmt.Errorf("item not found")
}

func (m *mockOnePasswordClient) DeleteItem(item *onepassword.Item, _ string) error {
	for title, existing := range m.items {
		if existing.ID == item.ID {
			delete(m.items, title)
			return nil
		}
	}
	return fmt.Errorf("item not found")
}

func TestOnePasswordConfigFromMap(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg, err := OnePasswordConfigFromMap(map[string]interface{}{
			"connectHost":  "https://connect.example.com",
			"connectToken": "token123",
			"vaultUuid":    "vault-uuid-123",
		})
		require.NoError(t, err)
		assert.Equal(t, "https://connect.example.com", cfg.ConnectHost)
		assert.Equal(t, "token123", cfg.ConnectToken)
		assert.Equal(t, "vault-uuid-123", cfg.VaultUUID)
	})

	t.Run("missing host", func(t *testing.T) {
		_, err := OnePasswordConfigFromMap(map[string]interface{}{
			"connectToken": "token",
			"vaultUuid":    "vault",
		})
		assert.Error(t, err)
	})

	t.Run("missing token", func(t *testing.T) {
		_, err := OnePasswordConfigFromMap(map[string]interface{}{
			"connectHost": "https://host",
			"vaultUuid":   "vault",
		})
		assert.Error(t, err)
	})

	t.Run("missing vault UUID", func(t *testing.T) {
		_, err := OnePasswordConfigFromMap(map[string]interface{}{
			"connectHost":  "https://host",
			"connectToken": "token",
		})
		assert.Error(t, err)
	})
}

func TestOnePasswordBackendType(t *testing.T) {
	backend := NewOnePasswordBackendFromClient(newMockOnePasswordClient(), "vault-uuid")
	assert.Equal(t, domainsecrets.BackendType1Password, backend.Type())
}

func TestOnePasswordBackendCRUD(t *testing.T) {
	client := newMockOnePasswordClient()
	backend := NewOnePasswordBackendFromClient(client, "vault-uuid")
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

	// Retrieve after delete
	items, _ := client.GetItemsByTitle(itemTitle(nsID), "vault-uuid")
	assert.Empty(t, items)
}

func TestOnePasswordBackendRetrieveNotFound(t *testing.T) {
	backend := NewOnePasswordBackendFromClient(newMockOnePasswordClient(), "vault-uuid")
	_, err := backend.Retrieve(uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no credential found")
}
