package secrets

import (
	"time"

	"github.com/google/uuid"
)

// NamespaceRepository provides data access for secret namespaces.
type NamespaceRepository interface {
	Create(ns *SecretNamespace) error
	GetByID(id uuid.UUID) (*SecretNamespace, error)
	GetByAgentAndName(agentID uuid.UUID, namespace string) (*SecretNamespace, error)
	ListByAgent(agentID uuid.UUID) ([]*SecretNamespace, error)
	UpdateStatus(id uuid.UUID, status NamespaceStatus) error
	Delete(id uuid.UUID) error
}

// CredentialRepository provides data access for encrypted credential blobs.
type CredentialRepository interface {
	Create(cred *SecretCredential) error
	GetLatestByNamespace(namespaceID uuid.UUID) (*SecretCredential, error)
	GetByVersion(namespaceID uuid.UUID, version int) (*SecretCredential, error)
	DeleteByNamespace(namespaceID uuid.UUID) error
}

// AuditRepository provides append-only access to the secrets audit log.
type AuditRepository interface {
	Create(entry *SecretAuditEntry) error
	ListByNamespace(namespaceID uuid.UUID, limit, offset int) ([]*SecretAuditEntry, error)
	ListByAgent(agentID uuid.UUID, since *time.Time, limit, offset int) ([]*SecretAuditEntry, error)
}

// BackendConfigRepository provides data access for per-agent backend configurations.
type BackendConfigRepository interface {
	Create(config *SecretBackendConfig) error
	GetByAgentAndType(agentID uuid.UUID, backendType BackendType) (*SecretBackendConfig, error)
	ListByAgent(agentID uuid.UUID) ([]*SecretBackendConfig, error)
	Update(config *SecretBackendConfig) error
	Delete(id uuid.UUID) error
}
