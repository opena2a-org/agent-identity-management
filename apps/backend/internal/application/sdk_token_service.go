package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// ErrSDKTokenNotFound is returned by SDKTokenService.RevokeToken for
// both "token does not exist" and "token belongs to a different user"
// — collapsing the two cases closes an existence-oracle side channel
// that let an attacker enumerate SDK token UUIDs across users. The
// handler maps this sentinel to a fixed 404 body.
var ErrSDKTokenNotFound = errors.New("sdk token not found")

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

// RevokeToken revokes a specific SDK token.
//
// SECURITY: both "not found" and "wrong user" return ErrSDKTokenNotFound.
// The pre-fix implementation returned distinct error strings
// ("token not found: …" vs "unauthorized: token belongs to different
// user") which the handler then mapped to 500 vs 403 with distinct
// bodies — an existence-oracle side channel that let an attacker
// enumerate token UUIDs and distinguish "exists owned by another
// user" from "doesn't exist". Both branches now collapse to the
// sentinel and the handler emits a fixed 404 body.
func (s *SDKTokenService) RevokeToken(ctx context.Context, tokenID uuid.UUID, userID uuid.UUID, reason string) error {
	_, err := s.RevokeTokenRow(ctx, tokenID, userID, reason)
	return err
}

// RevokeTokenRow is RevokeToken reporting the row it revoked, as it was read
// before the revoke, so the caller can end the row's token family too.
func (s *SDKTokenService) RevokeTokenRow(ctx context.Context, tokenID uuid.UUID, userID uuid.UUID, reason string) (*domain.SDKToken, error) {
	// Get token to verify ownership
	token, err := s.sdkTokenRepo.GetByID(tokenID)
	if err != nil || token == nil {
		return nil, ErrSDKTokenNotFound
	}

	// Verify user owns this token
	if token.UserID != userID {
		return nil, ErrSDKTokenNotFound
	}

	// Revoke token
	if err := s.sdkTokenRepo.Revoke(tokenID, reason); err != nil {
		return nil, err
	}
	return token, nil
}

// ErrSDKTokenFamilyUnknown is returned by FamilyOf when a row's rotation
// lineage cannot be followed to the download row that names its family.
var ErrSDKTokenFamilyUnknown = errors.New("sdk token family unknown")

// FamilyOf returns the token family of the row: the token ID of the download
// row its rotation lineage grew from. An SDK-download token's family is the
// download token's jti (sid is set to it at download and copied on every
// rotation), and the download's row stores that jti as its token ID; each
// rotated row names the row it replaced in its parent_token metadata, so the
// family is found by walking the lineage to the row with no parent. A row
// whose lineage cannot be read to its root (a parent row the repository no
// longer holds, or an unreadable parent id) gives ErrSDKTokenFamilyUnknown.
func (s *SDKTokenService) FamilyOf(ctx context.Context, row *domain.SDKToken) (string, error) {
	if row == nil {
		return "", ErrSDKTokenFamilyUnknown
	}
	seen := map[uuid.UUID]bool{row.ID: true}
	for {
		parent, _ := row.Metadata["parent_token"].(string)
		if parent == "" {
			if row.TokenID == "" {
				return "", ErrSDKTokenFamilyUnknown
			}
			return row.TokenID, nil
		}
		parentID, err := uuid.Parse(parent)
		if err != nil || seen[parentID] {
			return "", ErrSDKTokenFamilyUnknown
		}
		seen[parentID] = true
		row, err = s.sdkTokenRepo.GetByID(parentID)
		if err != nil || row == nil {
			return "", ErrSDKTokenFamilyUnknown
		}
	}
}

// RevokeByTokenHash revokes a token using its hash (for token rotation)
func (s *SDKTokenService) RevokeByTokenHash(ctx context.Context, tokenHash string, reason string) error {
	return s.sdkTokenRepo.RevokeByTokenHash(tokenHash, reason)
}

// RotateToken revokes the active token whose hash is oldTokenHash and stores
// its successor in one transaction (for token rotation): both happen or
// neither does.
func (s *SDKTokenService) RotateToken(ctx context.Context, oldTokenHash string, reason string, next *domain.SDKToken) error {
	return s.sdkTokenRepo.Rotate(oldTokenHash, reason, next)
}

// RevokeAllUserTokens revokes all SDK tokens for a user
func (s *SDKTokenService) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID, reason string) error {
	return s.sdkTokenRepo.RevokeAllForUser(userID, reason)
}

// RevokeAllUserTokenRows is RevokeAllUserTokens reporting the user's rows
// that were active when it ran, as they were read before the revoke, so the
// caller can end each row's token family too.
func (s *SDKTokenService) RevokeAllUserTokenRows(ctx context.Context, userID uuid.UUID, reason string) ([]*domain.SDKToken, error) {
	rows, err := s.sdkTokenRepo.GetByUserID(userID, false)
	if err != nil {
		return nil, err
	}
	if err := s.sdkTokenRepo.RevokeAllForUser(userID, reason); err != nil {
		return nil, err
	}
	return rows, nil
}

// RevokeFamily revokes every active SDK token row of a user's token family
// (after a reuse ends the family).
func (s *SDKTokenService) RevokeFamily(ctx context.Context, userID uuid.UUID, familyID string, reason string) error {
	return s.sdkTokenRepo.RevokeFamily(userID, familyID, reason)
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
