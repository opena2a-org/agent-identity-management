package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/google/uuid"

	domainsecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

// mockSecretsManagerClient implements SecretsManagerAPI for testing.
type mockSecretsManagerClient struct {
	secrets map[string][]byte
}

func newMockSecretsManagerClient() *mockSecretsManagerClient {
	return &mockSecretsManagerClient{
		secrets: make(map[string][]byte),
	}
}

func (m *mockSecretsManagerClient) CreateSecret(_ context.Context, input *secretsmanager.CreateSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
	name := aws.ToString(input.Name)
	if _, exists := m.secrets[name]; exists {
		return nil, fmt.Errorf("secret %s already exists", name)
	}
	m.secrets[name] = input.SecretBinary
	return &secretsmanager.CreateSecretOutput{
		Name: input.Name,
	}, nil
}

func (m *mockSecretsManagerClient) GetSecretValue(_ context.Context, input *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	name := aws.ToString(input.SecretId)
	data, exists := m.secrets[name]
	if !exists {
		return nil, fmt.Errorf("secret %s not found", name)
	}
	return &secretsmanager.GetSecretValueOutput{
		SecretBinary: data,
		Name:         input.SecretId,
	}, nil
}

func (m *mockSecretsManagerClient) PutSecretValue(_ context.Context, input *secretsmanager.PutSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
	name := aws.ToString(input.SecretId)
	if _, exists := m.secrets[name]; !exists {
		return nil, fmt.Errorf("secret %s not found", name)
	}
	m.secrets[name] = input.SecretBinary
	return &secretsmanager.PutSecretValueOutput{
		Name: input.SecretId,
	}, nil
}

func (m *mockSecretsManagerClient) DeleteSecret(_ context.Context, input *secretsmanager.DeleteSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error) {
	name := aws.ToString(input.SecretId)
	if _, exists := m.secrets[name]; !exists {
		return nil, fmt.Errorf("secret %s not found", name)
	}
	delete(m.secrets, name)
	return &secretsmanager.DeleteSecretOutput{
		Name: input.SecretId,
	}, nil
}

func TestAWSConfigFromMap(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		m := map[string]interface{}{
			"region":   "us-east-1",
			"kmsKeyId": "arn:aws:kms:us-east-1:123456789:key/abc",
		}
		cfg, err := AWSConfigFromMap(m)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Region != "us-east-1" {
			t.Errorf("region = %q, want %q", cfg.Region, "us-east-1")
		}
		if cfg.Prefix != "aim-secrets/" {
			t.Errorf("prefix = %q, want %q (default)", cfg.Prefix, "aim-secrets/")
		}
	})

	t.Run("missing region", func(t *testing.T) {
		m := map[string]interface{}{
			"kmsKeyId": "arn:aws:kms:us-east-1:123456789:key/abc",
		}
		_, err := AWSConfigFromMap(m)
		if err == nil {
			t.Fatal("expected error for missing region")
		}
	})
}

func TestAWSSecretsBackendType(t *testing.T) {
	b := NewAWSSecretsBackendFromClient(newMockSecretsManagerClient(), "", "aim-secrets/")
	if b.Type() != domainsecrets.BackendTypeAWSKMS {
		t.Errorf("Type() = %q, want %q", b.Type(), domainsecrets.BackendTypeAWSKMS)
	}
}

func TestAWSSecretsBackendCRUD(t *testing.T) {
	mock := newMockSecretsManagerClient()
	b := NewAWSSecretsBackendFromClient(mock, "", "aim-secrets/")
	nsID := uuid.New()
	blob := []byte("encrypted-credential-data")
	alg := "X25519-ChaCha20-Poly1305"

	// Store
	err := b.Store(nsID, blob, alg)
	if err != nil {
		t.Fatalf("Store() error: %v", err)
	}

	// Retrieve
	cred, err := b.Retrieve(nsID)
	if err != nil {
		t.Fatalf("Retrieve() error: %v", err)
	}
	if string(cred.EncryptedBlob) != string(blob) {
		t.Errorf("blob = %q, want %q", cred.EncryptedBlob, blob)
	}
	if cred.EncryptionAlg != alg {
		t.Errorf("encryptionAlg = %q, want %q", cred.EncryptionAlg, alg)
	}
	if cred.Version != 1 {
		t.Errorf("version = %d, want 1", cred.Version)
	}
	if cred.NamespaceID != nsID {
		t.Errorf("namespaceID = %s, want %s", cred.NamespaceID, nsID)
	}

	// Rotate
	newBlob := []byte("rotated-credential-data")
	err = b.Rotate(nsID, newBlob, alg)
	if err != nil {
		t.Fatalf("Rotate() error: %v", err)
	}

	cred, err = b.Retrieve(nsID)
	if err != nil {
		t.Fatalf("Retrieve() after rotate error: %v", err)
	}
	if string(cred.EncryptedBlob) != string(newBlob) {
		t.Errorf("rotated blob = %q, want %q", cred.EncryptedBlob, newBlob)
	}
	if cred.Version != 2 {
		t.Errorf("rotated version = %d, want 2", cred.Version)
	}

	// Delete
	err = b.Delete(nsID)
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	// Retrieve after delete should fail
	_, err = b.Retrieve(nsID)
	if err == nil {
		t.Error("Retrieve() after delete should fail")
	}
}

func TestAWSSecretsBackendStoreAlreadyExists(t *testing.T) {
	mock := newMockSecretsManagerClient()
	b := NewAWSSecretsBackendFromClient(mock, "", "aim-secrets/")
	nsID := uuid.New()

	err := b.Store(nsID, []byte("data"), "alg")
	if err != nil {
		t.Fatalf("first Store() error: %v", err)
	}

	err = b.Store(nsID, []byte("data2"), "alg")
	if err == nil {
		t.Error("second Store() should fail (secret already exists)")
	}
}

func TestAWSSecretsBackendRetrieveNotFound(t *testing.T) {
	mock := newMockSecretsManagerClient()
	b := NewAWSSecretsBackendFromClient(mock, "", "aim-secrets/")

	_, err := b.Retrieve(uuid.New())
	if err == nil {
		t.Error("Retrieve() on non-existent secret should fail")
	}
}

func TestAWSSecretsBackendDeleteNotFound(t *testing.T) {
	mock := newMockSecretsManagerClient()
	b := NewAWSSecretsBackendFromClient(mock, "", "aim-secrets/")

	err := b.Delete(uuid.New())
	if err == nil {
		t.Error("Delete() on non-existent secret should fail")
	}
}

func TestAWSSecretsBackendSecretPayloadJSON(t *testing.T) {
	payload := secretPayload{
		Blob:          []byte("test-blob"),
		EncryptionAlg: "X25519-ChaCha20-Poly1305",
		Version:       3,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded secretPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if string(decoded.Blob) != "test-blob" {
		t.Errorf("blob = %q, want %q", decoded.Blob, "test-blob")
	}
	if decoded.Version != 3 {
		t.Errorf("version = %d, want 3", decoded.Version)
	}
}

func TestAWSSecretsBackendWithKMSKey(t *testing.T) {
	mock := newMockSecretsManagerClient()
	kmsKey := "arn:aws:kms:us-east-1:123456789:key/abc-def"
	b := NewAWSSecretsBackendFromClient(mock, kmsKey, "aim-secrets/")

	if b.kmsKeyID != kmsKey {
		t.Errorf("kmsKeyID = %q, want %q", b.kmsKeyID, kmsKey)
	}

	// Store should work (KMS key is passed in CreateSecretInput)
	err := b.Store(uuid.New(), []byte("data"), "alg")
	if err != nil {
		t.Fatalf("Store() with KMS key error: %v", err)
	}
}
