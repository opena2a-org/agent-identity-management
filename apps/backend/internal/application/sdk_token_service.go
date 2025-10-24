package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// SDKTokenService handles SDK token business logic
type SDKTokenService struct {
	sdkTokenRepo domain.SDKTokenRepository
}

// NewSDKTokenService creates a new SDK token service
func NewSDKTokenService(sdkTokenRepo domain.SDKTokenRepository) *SDKTokenService {
	return &SDKTokenService{
		sdkTokenRepo: sdkTokenRepo,
	}
}

// GetUserTokens retrieves all SDK tokens for a user
func (s *SDKTokenService) GetUserTokens(ctx context.Context, userID uuid.UUID, includeRevoked bool) ([]*domain.SDKToken, error) {
	return s.sdkTokenRepo.GetByUserID(userID, includeRevoked)
}

// GetOrganizationTokens retrieves all SDK tokens for an organization
func (s *SDKTokenService) GetOrganizationTokens(ctx context.Context, organizationID uuid.UUID, includeRevoked bool) ([]*domain.SDKToken, error) {
	return s.sdkTokenRepo.GetByOrganizationID(organizationID, includeRevoked)
}

// RevokeToken revokes a specific SDK token
func (s *SDKTokenService) RevokeToken(ctx context.Context, tokenID uuid.UUID, userID uuid.UUID, reason string) error {
	// Get token to verify ownership
	token, err := s.sdkTokenRepo.GetByID(tokenID)
	if err != nil {
		return fmt.Errorf("token not found: %w", err)
	}

	// Verify user owns this token
	if token.UserID != userID {
		return fmt.Errorf("unauthorized: token belongs to different user")
	}

	// Revoke token
	return s.sdkTokenRepo.Revoke(tokenID, reason)
}

// RevokeByTokenHash revokes a token using its hash (for token rotation)
func (s *SDKTokenService) RevokeByTokenHash(ctx context.Context, tokenHash string, reason string) error {
	return s.sdkTokenRepo.RevokeByTokenHash(tokenHash, reason)
}

// RevokeAllUserTokens revokes all SDK tokens for a user
func (s *SDKTokenService) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID, reason string) error {
	return s.sdkTokenRepo.RevokeAllForUser(userID, reason)
}

// GetActiveTokenCount returns count of active tokens for a user
func (s *SDKTokenService) GetActiveTokenCount(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.sdkTokenRepo.GetActiveCount(userID)
}

// RecordTokenUsage updates token usage statistics
func (s *SDKTokenService) RecordTokenUsage(ctx context.Context, tokenID string, ipAddress string) error {
	return s.sdkTokenRepo.RecordUsage(tokenID, ipAddress)
}

// ValidateToken checks if a token is active (not revoked, not expired)
func (s *SDKTokenService) ValidateToken(ctx context.Context, tokenHash string) (*domain.SDKToken, error) {
	token, err := s.sdkTokenRepo.GetByTokenHash(tokenHash)
	if err != nil {
		return nil, fmt.Errorf("token not found: %w", err)
	}

	if !token.IsActive() {
		return nil, fmt.Errorf("token is revoked or expired")
	}

	return token, nil
}

// CreateToken creates a new SDK token (for token rotation)
func (s *SDKTokenService) CreateToken(ctx context.Context, token *domain.SDKToken) error {
	return s.sdkTokenRepo.Create(token)
}

// GetByTokenHash retrieves a token by its hash (even if revoked - for recovery)
func (s *SDKTokenService) GetByTokenHash(ctx context.Context, tokenHash string) (*domain.SDKToken, error) {
	return s.sdkTokenRepo.GetByTokenHash(tokenHash)
}

// CleanupExpiredTokens removes expired tokens (for scheduled jobs)
func (s *SDKTokenService) CleanupExpiredTokens(ctx context.Context) error {
	return s.sdkTokenRepo.DeleteExpired()
}
