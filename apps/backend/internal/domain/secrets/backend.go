package secrets

import "github.com/google/uuid"

// SecretsBackend is the storage interface for credential blobs.
// Implementations must NOT contain business logic (CR-009).
// All policy, auth, and audit logic lives in the application service.
type SecretsBackend interface {
	// Store persists an encrypted credential blob for a namespace.
	Store(namespaceID uuid.UUID, blob []byte, encryptionAlg string) error

	// Retrieve returns the latest encrypted credential blob for a namespace.
	Retrieve(namespaceID uuid.UUID) (*SecretCredential, error)

	// Rotate stores a new version of the credential, incrementing the version.
	Rotate(namespaceID uuid.UUID, blob []byte, encryptionAlg string) error

	// Delete removes all credential versions for a namespace.
	Delete(namespaceID uuid.UUID) error

	// Type returns the backend type identifier.
	Type() BackendType
}
