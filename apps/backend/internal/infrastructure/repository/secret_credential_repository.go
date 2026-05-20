package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

type SecretCredentialRepository struct {
	db *sql.DB
}

func NewSecretCredentialRepository(db *sql.DB) *SecretCredentialRepository {
	return &SecretCredentialRepository{db: db}
}

func (r *SecretCredentialRepository) Create(cred *secrets.SecretCredential) error {
	now := time.Now()
	if cred.ID == uuid.Nil {
		cred.ID = uuid.New()
	}
	cred.CreatedAt = now
	cred.UpdatedAt = now
	if cred.Version == 0 {
		cred.Version = 1
	}

	query := `
		INSERT INTO secret_credentials (id, namespace_id, encrypted_blob, encryption_alg, ephemeral_pubkey, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(query,
		cred.ID, cred.NamespaceID, cred.EncryptedBlob, cred.EncryptionAlg,
		cred.EphemeralPubKey, cred.Version, cred.CreatedAt, cred.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create secret credential: %w", err)
	}
	return nil
}

func (r *SecretCredentialRepository) GetLatestByNamespace(namespaceID uuid.UUID) (*secrets.SecretCredential, error) {
	query := `
		SELECT id, namespace_id, encrypted_blob, encryption_alg, ephemeral_pubkey, version, created_at, updated_at
		FROM secret_credentials WHERE namespace_id = $1 ORDER BY version DESC LIMIT 1
	`
	cred := &secrets.SecretCredential{}
	ephPub := make([]byte, 0)

	err := r.db.QueryRow(query, namespaceID).Scan(
		&cred.ID, &cred.NamespaceID, &cred.EncryptedBlob, &cred.EncryptionAlg,
		&ephPub, &cred.Version, &cred.CreatedAt, &cred.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest secret credential: %w", err)
	}

	cred.EphemeralPubKey = ephPub
	return cred, nil
}

func (r *SecretCredentialRepository) GetByVersion(namespaceID uuid.UUID, version int) (*secrets.SecretCredential, error) {
	query := `
		SELECT id, namespace_id, encrypted_blob, encryption_alg, ephemeral_pubkey, version, created_at, updated_at
		FROM secret_credentials WHERE namespace_id = $1 AND version = $2
	`
	cred := &secrets.SecretCredential{}
	ephPub := make([]byte, 0)

	err := r.db.QueryRow(query, namespaceID, version).Scan(
		&cred.ID, &cred.NamespaceID, &cred.EncryptedBlob, &cred.EncryptionAlg,
		&ephPub, &cred.Version, &cred.CreatedAt, &cred.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get secret credential by version: %w", err)
	}

	cred.EphemeralPubKey = ephPub
	return cred, nil
}

func (r *SecretCredentialRepository) DeleteByNamespace(namespaceID uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM secret_credentials WHERE namespace_id = $1`, namespaceID)
	if err != nil {
		return fmt.Errorf("failed to delete secret credentials: %w", err)
	}
	return nil
}
