package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// APIKeyService handles API key operations
type APIKeyService struct {
	apiKeyRepo domain.APIKeyRepository
	agentRepo  domain.AgentRepository
}

// NewAPIKeyService creates a new API key service
func NewAPIKeyService(
	apiKeyRepo domain.APIKeyRepository,
	agentRepo domain.AgentRepository,
) *APIKeyService {
	return &APIKeyService{
		apiKeyRepo: apiKeyRepo,
		agentRepo:  agentRepo,
	}
}

// GenerateAPIKey generates a new API key for an agent
func (s *APIKeyService) GenerateAPIKey(ctx context.Context, agentID, orgID, userID uuid.UUID, name string, expiresInDays int) (string, *domain.APIKey, error) {
	// Verify agent exists and belongs to organization
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return "", nil, fmt.Errorf("agent not found: %w", err)
	}

	if agent.OrganizationID != orgID {
		return "", nil, fmt.Errorf("agent does not belong to organization")
	}

	// Generate random key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", nil, fmt.Errorf("failed to generate random key: %w", err)
	}

	// Encode key to base64
	keyString := base64.URLEncoding.EncodeToString(keyBytes)

	// Create full API key with prefix
	fullKey := fmt.Sprintf("aim_live_%s", keyString)

	// Hash the key for storage
	hash := sha256.Sum256([]byte(fullKey))
	keyHash := base64.StdEncoding.EncodeToString(hash[:])

	// Extract prefix (first 12 chars after aim_live_)
	prefix := fullKey[:16] // "aim_live_" + first 8 chars

	// Calculate expiry
	var expiresAt *time.Time
	if expiresInDays > 0 {
		expiry := time.Now().AddDate(0, 0, expiresInDays)
		expiresAt = &expiry
	}

	// Create API key record
	apiKey := &domain.APIKey{
		OrganizationID: orgID,
		AgentID:        agentID,
		Name:           name,
		KeyHash:        keyHash,
		Prefix:         prefix,
		ExpiresAt:      expiresAt,
		IsActive:       true,
		CreatedBy:      userID,
	}

	if err := s.apiKeyRepo.Create(apiKey); err != nil {
		return "", nil, fmt.Errorf("failed to create API key: %w", err)
	}

	return fullKey, apiKey, nil
}

// ListAPIKeys lists all API keys for an organization
func (s *APIKeyService) ListAPIKeys(ctx context.Context, orgID uuid.UUID) ([]*domain.APIKey, error) {
	return s.apiKeyRepo.GetByOrganization(orgID)
}

// RevokeAPIKey revokes an API key (sets is_active=false)
func (s *APIKeyService) RevokeAPIKey(ctx context.Context, keyID, orgID uuid.UUID) error {
	// Verify key belongs to organization
	key, err := s.apiKeyRepo.GetByID(keyID)
	if err != nil {
		return err
	}

	if key.OrganizationID != orgID {
		return fmt.Errorf("API key does not belong to organization")
	}

	return s.apiKeyRepo.Revoke(keyID)
}

// DeleteAPIKey permanently deletes an API key (only if disabled)
func (s *APIKeyService) DeleteAPIKey(ctx context.Context, keyID, orgID uuid.UUID) error {
	// Verify key belongs to organization
	key, err := s.apiKeyRepo.GetByID(keyID)
	if err != nil {
		return err
	}

	if key.OrganizationID != orgID {
		return fmt.Errorf("API key does not belong to organization")
	}

	// Check if key is disabled (is_active=false)
	if key.IsActive {
		return fmt.Errorf("API key must be disabled before deletion")
	}

	return s.apiKeyRepo.Delete(keyID)
}

// ValidateAPIKey validates an API key and returns the associated API key record
func (s *APIKeyService) ValidateAPIKey(ctx context.Context, fullKey string) (*domain.APIKey, error) {
	// Hash the provided key
	hash := sha256.Sum256([]byte(fullKey))
	keyHash := base64.StdEncoding.EncodeToString(hash[:])

	// Look up key by hash
	apiKey, err := s.apiKeyRepo.GetByHash(keyHash)
	if err != nil {
		return nil, err
	}

	if apiKey == nil {
		return nil, fmt.Errorf("invalid API key")
	}

	// Check if expired
	if apiKey.ExpiresAt != nil && time.Now().After(*apiKey.ExpiresAt) {
		return nil, fmt.Errorf("API key expired")
	}

	// Update last used timestamp
	s.apiKeyRepo.UpdateLastUsed(apiKey.ID)

	return apiKey, nil
}
