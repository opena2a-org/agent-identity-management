package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	gax "github.com/googleapis/gax-go/v2"
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/google/uuid"

	domainsecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

const gcpTimeout = 30 * time.Second

// GCPSecretConfig holds the configuration for GCP Secret Manager backend.
type GCPSecretConfig struct {
	// ProjectID is the GCP project ID.
	ProjectID string `json:"projectId"`
}

// GCPSecretConfigFromMap parses a GCPSecretConfig from a generic config map.
func GCPSecretConfigFromMap(m map[string]interface{}) (*GCPSecretConfig, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	var cfg GCPSecretConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse GCP config: %w", err)
	}
	if cfg.ProjectID == "" {
		return nil, fmt.Errorf("GCP project ID is required")
	}
	return &cfg, nil
}

// GCPSecretManagerAPI abstracts the GCP Secret Manager client for testability.
type GCPSecretManagerAPI interface {
	CreateSecret(ctx context.Context, req *secretmanagerpb.CreateSecretRequest, opts ...gax.CallOption) (*secretmanagerpb.Secret, error)
	AddSecretVersion(ctx context.Context, req *secretmanagerpb.AddSecretVersionRequest, opts ...gax.CallOption) (*secretmanagerpb.SecretVersion, error)
	AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, opts ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error)
	DeleteSecret(ctx context.Context, req *secretmanagerpb.DeleteSecretRequest, opts ...gax.CallOption) error
}

// GCPSecretBackend implements SecretsBackend using GCP Secret Manager.
// No business logic here (CR-009) — just maps CRUD to GCP Secret Manager API.
type GCPSecretBackend struct {
	client    GCPSecretManagerAPI
	projectID string
}

// NewGCPSecretBackend creates a backend using Application Default Credentials.
func NewGCPSecretBackend(cfg *GCPSecretConfig) (*GCPSecretBackend, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gcpTimeout)
	defer cancel()

	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP secret manager client: %w", err)
	}

	return &GCPSecretBackend{
		client:    client,
		projectID: cfg.ProjectID,
	}, nil
}

// NewGCPSecretBackendFromClient creates a backend from an existing client (for testing).
func NewGCPSecretBackendFromClient(client GCPSecretManagerAPI, projectID string) *GCPSecretBackend {
	return &GCPSecretBackend{
		client:    client,
		projectID: projectID,
	}
}

// secretID returns the GCP secret ID for a namespace.
func gcpSecretID(namespaceID uuid.UUID) string {
	return fmt.Sprintf("aim-secrets-%s", namespaceID.String())
}

// secretName returns the full GCP resource name for a secret.
func (b *GCPSecretBackend) secretName(namespaceID uuid.UUID) string {
	return fmt.Sprintf("projects/%s/secrets/%s", b.projectID, gcpSecretID(namespaceID))
}

// gcpPayload is the JSON structure stored as the secret version data.
type gcpPayload struct {
	Blob          []byte `json:"blob"`
	EncryptionAlg string `json:"encryptionAlg"`
	Version       int    `json:"version"`
}

func (b *GCPSecretBackend) Store(namespaceID uuid.UUID, blob []byte, encryptionAlg string) error {
	ctx, cancel := context.WithTimeout(context.Background(), gcpTimeout)
	defer cancel()

	// Create the secret resource (container).
	_, err := b.client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent:   fmt.Sprintf("projects/%s", b.projectID),
		SecretId: gcpSecretID(namespaceID),
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("%w: gcp create secret failed: %v", domainsecrets.ErrBackendUnavailable, err)
	}

	// Add the first version with the payload.
	payload := gcpPayload{
		Blob:          blob,
		EncryptionAlg: encryptionAlg,
		Version:       1,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	_, err = b.client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: b.secretName(namespaceID),
		Payload: &secretmanagerpb.SecretPayload{
			Data: payloadBytes,
		},
	})
	if err != nil {
		return fmt.Errorf("%w: gcp add version failed: %v", domainsecrets.ErrBackendUnavailable, err)
	}
	return nil
}

func (b *GCPSecretBackend) Retrieve(namespaceID uuid.UUID) (*domainsecrets.SecretCredential, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gcpTimeout)
	defer cancel()

	resp, err := b.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("%s/versions/latest", b.secretName(namespaceID)),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: gcp retrieve failed: %v", domainsecrets.ErrBackendUnavailable, err)
	}

	var payload gcpPayload
	if err := json.Unmarshal(resp.Payload.Data, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal gcp secret payload: %w", err)
	}

	return &domainsecrets.SecretCredential{
		NamespaceID:   namespaceID,
		EncryptedBlob: payload.Blob,
		EncryptionAlg: payload.EncryptionAlg,
		Version:       payload.Version,
	}, nil
}

func (b *GCPSecretBackend) Rotate(namespaceID uuid.UUID, blob []byte, encryptionAlg string) error {
	ctx, cancel := context.WithTimeout(context.Background(), gcpTimeout)
	defer cancel()

	current, err := b.Retrieve(namespaceID)
	nextVersion := 1
	if err == nil && current != nil {
		nextVersion = current.Version + 1
	}

	payload := gcpPayload{
		Blob:          blob,
		EncryptionAlg: encryptionAlg,
		Version:       nextVersion,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	_, err = b.client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: b.secretName(namespaceID),
		Payload: &secretmanagerpb.SecretPayload{
			Data: payloadBytes,
		},
	})
	if err != nil {
		return fmt.Errorf("%w: gcp rotate failed: %v", domainsecrets.ErrBackendUnavailable, err)
	}
	return nil
}

func (b *GCPSecretBackend) Delete(namespaceID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), gcpTimeout)
	defer cancel()

	err := b.client.DeleteSecret(ctx, &secretmanagerpb.DeleteSecretRequest{
		Name: b.secretName(namespaceID),
	})
	if err != nil {
		return fmt.Errorf("%w: gcp delete failed: %v", domainsecrets.ErrBackendUnavailable, err)
	}
	return nil
}

func (b *GCPSecretBackend) Type() domainsecrets.BackendType {
	return domainsecrets.BackendTypeGCPSM
}
