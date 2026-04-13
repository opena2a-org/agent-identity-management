package secrets

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	domainsecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

// AIMNativeBackend implements SecretsBackend using the AIM credential repository.
// No business logic here (CR-009) — just a thin wrapper around CredentialRepository.
type AIMNativeBackend struct {
	credRepo domainsecrets.CredentialRepository
}

func NewAIMNativeBackend(credRepo domainsecrets.CredentialRepository) *AIMNativeBackend {
	return &AIMNativeBackend{credRepo: credRepo}
}

func (b *AIMNativeBackend) Store(namespaceID uuid.UUID, blob []byte, encryptionAlg string) error {
	cred := &domainsecrets.SecretCredential{
		NamespaceID:   namespaceID,
		EncryptedBlob: blob,
		EncryptionAlg: encryptionAlg,
		Version:       1,
	}
	return b.credRepo.Create(cred)
}

func (b *AIMNativeBackend) Retrieve(namespaceID uuid.UUID) (*domainsecrets.SecretCredential, error) {
	cred, err := b.credRepo.GetLatestByNamespace(namespaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve credential: %w", err)
	}
	if cred == nil {
		return nil, fmt.Errorf("no credential found for namespace %s", namespaceID)
	}
	return cred, nil
}

func (b *AIMNativeBackend) Rotate(namespaceID uuid.UUID, blob []byte, encryptionAlg string) error {
	latest, err := b.credRepo.GetLatestByNamespace(namespaceID)
	if err != nil {
		return fmt.Errorf("failed to get latest credential for rotation: %w", err)
	}

	nextVersion := 1
	if latest != nil {
		nextVersion = latest.Version + 1
	}

	cred := &domainsecrets.SecretCredential{
		ID:            uuid.New(),
		NamespaceID:   namespaceID,
		EncryptedBlob: blob,
		EncryptionAlg: encryptionAlg,
		Version:       nextVersion,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	return b.credRepo.Create(cred)
}

func (b *AIMNativeBackend) Delete(namespaceID uuid.UUID) error {
	return b.credRepo.DeleteByNamespace(namespaceID)
}

func (b *AIMNativeBackend) Type() domainsecrets.BackendType {
	return domainsecrets.BackendTypeAIMNative
}
