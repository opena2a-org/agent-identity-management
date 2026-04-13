package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/google/uuid"

	domainsecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

// AWSConfig holds the configuration for AWS Secrets Manager backend.
type AWSConfig struct {
	// Region is the AWS region (e.g., "us-east-1").
	Region string `json:"region"`

	// KMSKeyID is an optional custom KMS key ARN for encryption.
	// If empty, AWS uses the default aws/secretsmanager key.
	KMSKeyID string `json:"kmsKeyId,omitempty"`

	// Prefix is prepended to secret names in AWS (default: "aim-secrets/").
	Prefix string `json:"prefix,omitempty"`
}

// AWSConfigFromMap parses an AWSConfig from a generic config map.
func AWSConfigFromMap(m map[string]interface{}) (*AWSConfig, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	var cfg AWSConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse AWS config: %w", err)
	}
	if cfg.Region == "" {
		return nil, fmt.Errorf("AWS region is required")
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "aim-secrets/"
	}
	return &cfg, nil
}

// AWSSecretsBackend implements SecretsBackend using AWS Secrets Manager.
// No business logic here (CR-009) — just maps CRUD to AWS Secrets Manager API.
type AWSSecretsBackend struct {
	client   SecretsManagerAPI
	kmsKeyID string
	prefix   string
}

// SecretsManagerAPI abstracts the AWS Secrets Manager client for testability.
type SecretsManagerAPI interface {
	CreateSecret(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error)
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	PutSecretValue(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error)
	DeleteSecret(ctx context.Context, params *secretsmanager.DeleteSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error)
}

// NewAWSSecretsBackend creates a backend using standard AWS credential chain.
func NewAWSSecretsBackend(cfg *AWSConfig) (*AWSSecretsBackend, error) {
	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := secretsmanager.NewFromConfig(awsCfg)

	return &AWSSecretsBackend{
		client:   client,
		kmsKeyID: cfg.KMSKeyID,
		prefix:   cfg.Prefix,
	}, nil
}

// NewAWSSecretsBackendFromClient creates a backend from an existing client (for testing).
func NewAWSSecretsBackendFromClient(client SecretsManagerAPI, kmsKeyID, prefix string) *AWSSecretsBackend {
	if prefix == "" {
		prefix = "aim-secrets/"
	}
	return &AWSSecretsBackend{
		client:   client,
		kmsKeyID: kmsKeyID,
		prefix:   prefix,
	}
}

// secretName returns the AWS secret name for a namespace.
func (b *AWSSecretsBackend) secretName(namespaceID uuid.UUID) string {
	return fmt.Sprintf("%s%s", b.prefix, namespaceID.String())
}

// secretPayload is the JSON structure stored in AWS Secrets Manager.
type secretPayload struct {
	Blob          []byte `json:"blob"`
	EncryptionAlg string `json:"encryptionAlg"`
	Version       int    `json:"version"`
}

func (b *AWSSecretsBackend) Store(namespaceID uuid.UUID, blob []byte, encryptionAlg string) error {
	payload := secretPayload{
		Blob:          blob,
		EncryptionAlg: encryptionAlg,
		Version:       1,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	input := &secretsmanager.CreateSecretInput{
		Name:         aws.String(b.secretName(namespaceID)),
		SecretBinary: payloadBytes,
	}
	if b.kmsKeyID != "" {
		input.KmsKeyId = aws.String(b.kmsKeyID)
	}

	_, err = b.client.CreateSecret(context.Background(), input)
	if err != nil {
		return fmt.Errorf("AWS create secret failed: %w", err)
	}
	return nil
}

func (b *AWSSecretsBackend) Retrieve(namespaceID uuid.UUID) (*domainsecrets.SecretCredential, error) {
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(b.secretName(namespaceID)),
	}

	result, err := b.client.GetSecretValue(context.Background(), input)
	if err != nil {
		return nil, fmt.Errorf("AWS get secret failed: %w", err)
	}

	var payload secretPayload
	secretData := result.SecretBinary
	if len(secretData) == 0 && result.SecretString != nil {
		secretData = []byte(*result.SecretString)
	}

	if err := json.Unmarshal(secretData, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secret payload: %w", err)
	}

	return &domainsecrets.SecretCredential{
		NamespaceID:   namespaceID,
		EncryptedBlob: payload.Blob,
		EncryptionAlg: payload.EncryptionAlg,
		Version:       payload.Version,
	}, nil
}

func (b *AWSSecretsBackend) Rotate(namespaceID uuid.UUID, blob []byte, encryptionAlg string) error {
	// Read current version
	current, err := b.Retrieve(namespaceID)
	nextVersion := 1
	if err == nil && current != nil {
		nextVersion = current.Version + 1
	}

	payload := secretPayload{
		Blob:          blob,
		EncryptionAlg: encryptionAlg,
		Version:       nextVersion,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	input := &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(b.secretName(namespaceID)),
		SecretBinary: payloadBytes,
	}

	_, err = b.client.PutSecretValue(context.Background(), input)
	if err != nil {
		return fmt.Errorf("AWS rotate secret failed: %w", err)
	}
	return nil
}

func (b *AWSSecretsBackend) Delete(namespaceID uuid.UUID) error {
	input := &secretsmanager.DeleteSecretInput{
		SecretId:                   aws.String(b.secretName(namespaceID)),
		ForceDeleteWithoutRecovery: aws.Bool(true),
	}

	_, err := b.client.DeleteSecret(context.Background(), input)
	if err != nil {
		return fmt.Errorf("AWS delete secret failed: %w", err)
	}
	return nil
}

func (b *AWSSecretsBackend) Type() domainsecrets.BackendType {
	return domainsecrets.BackendTypeAWSKMS
}

// interfaceToString safely converts interface{} to string.
func interfaceToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}
