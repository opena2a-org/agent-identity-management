package secrets

import (
	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
	"errors"
	"time"

	"github.com/google/uuid"
)

// BackendType identifies the secrets backend implementation.
type BackendType string

const (
	BackendTypeAIMNative BackendType = "aim_native"
	BackendTypeAWSKMS    BackendType = "aws_kms"
	BackendTypeVault     BackendType = "hashicorp_vault"
	BackendTypeAzureKV   BackendType = "azure_keyvault"
	BackendTypeGCPSM     BackendType = "gcp_secret_manager"
	BackendType1Password BackendType = "1password"
)

// ErrBackendUnavailable is returned when a backend cannot be reached.
// Callers must NOT fall back to another backend or return plaintext.
var ErrBackendUnavailable = errors.New("secrets backend unavailable")

// NamespaceStatus represents the lifecycle state of a secret namespace.
type NamespaceStatus string

const (
	NamespaceStatusActive  NamespaceStatus = "active"
	NamespaceStatusRevoked NamespaceStatus = "revoked"
)

// SecretNamespace is a logical grouping of credentials for an agent.
// Each namespace has an owning agent, a backend type, and allowed operations/URL patterns.
type SecretNamespace struct {
	ID          uuid.UUID       `json:"id"`
	AgentID     uuid.UUID       `json:"agentId"`
	Namespace   string          `json:"namespace"`
	BackendType BackendType     `json:"backendType"`
	Operations  []string        `json:"operations"`
	URLPatterns []string        `json:"urlPatterns"`
	Status      NamespaceStatus `json:"status"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
	CreatedBy   uuid.UUID       `json:"createdBy"`
}

// SecretCredential stores an encrypted credential blob within a namespace.
type SecretCredential struct {
	ID              uuid.UUID `json:"id"`
	NamespaceID     uuid.UUID `json:"namespaceId"`
	EncryptedBlob   []byte    `json:"-"`
	EncryptionAlg   string    `json:"encryptionAlg"`
	EphemeralPubKey []byte    `json:"ephemeralPubKey,omitempty"`
	Version         int       `json:"version"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// SecretAuditEntry records every resolution attempt (granted or denied).
type SecretAuditEntry struct {
	ID          uuid.UUID `json:"id"`
	NamespaceID uuid.UUID `json:"namespaceId"`
	AgentID     uuid.UUID `json:"agentId"`
	Operation   string    `json:"operation"`
	Result      string    `json:"result"` // "granted" or "denied"
	DenyReason  string    `json:"denyReason,omitempty"`
	ATCID       string    `json:"atcId,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Audit result constants
const (
	AuditResultGranted = "granted"
	AuditResultDenied  = "denied"
)

// SecretBackendConfig stores per-agent backend configuration (e.g., AWS KMS key ARN).
type SecretBackendConfig struct {
	ID          uuid.UUID              `json:"id"`
	AgentID     uuid.UUID              `json:"agentId"`
	BackendType BackendType            `json:"backendType"`
	ConfigJSON  map[string]interface{} `json:"config"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
}

// ResolutionRequest is submitted by an agent to resolve a credential.
type ResolutionRequest struct {
	Namespace      string `json:"namespace"`
	Operation      string `json:"operation"`
	Nonce          string `json:"nonce"`
	Signature      []byte `json:"signature"`
	AgentPublicKey []byte `json:"agentPublicKey"`
	ATCID          string                `json:"atcId,omitempty"`
	ATCClaims      *atcdomain.ATCClaims  `json:"-"` // Set by ATCAuthMiddleware when ATC auth is used
}

// ResolutionResult is returned after a successful resolution.
type ResolutionResult struct {
	EncryptedBlob   []byte `json:"encryptedBlob"`
	EphemeralPubKey []byte `json:"ephemeralPubKey"`
	EncryptionAlg   string `json:"encryptionAlg"`
}
