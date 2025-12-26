package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockSDKTokenRepository is a mock implementation of SDKTokenRepository
type MockSDKTokenRepository struct {
	tokens          map[uuid.UUID]*domain.SDKToken
	tokensByHash    map[string]*domain.SDKToken
	tokensByTokenID map[string]*domain.SDKToken
	createErr       error
	getByIDErr      error
	getByHashErr    error
	revokeErr       error
	deleteExpiredErr error
}

func NewMockSDKTokenRepository() *MockSDKTokenRepository {
	return &MockSDKTokenRepository{
		tokens:          make(map[uuid.UUID]*domain.SDKToken),
		tokensByHash:    make(map[string]*domain.SDKToken),
		tokensByTokenID: make(map[string]*domain.SDKToken),
	}
}

func (m *MockSDKTokenRepository) Create(token *domain.SDKToken) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.tokens[token.ID] = token
	m.tokensByHash[token.TokenHash] = token
	m.tokensByTokenID[token.TokenID] = token
	return nil
}

func (m *MockSDKTokenRepository) GetByID(id uuid.UUID) (*domain.SDKToken, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	token, ok := m.tokens[id]
	if !ok {
		return nil, errors.New("token not found")
	}
	return token, nil
}

func (m *MockSDKTokenRepository) GetByTokenID(tokenID string) (*domain.SDKToken, error) {
	token, ok := m.tokensByTokenID[tokenID]
	if !ok {
		return nil, errors.New("token not found")
	}
	return token, nil
}

func (m *MockSDKTokenRepository) GetByTokenHash(tokenHash string) (*domain.SDKToken, error) {
	if m.getByHashErr != nil {
		return nil, m.getByHashErr
	}
	token, ok := m.tokensByHash[tokenHash]
	if !ok {
		return nil, errors.New("token not found")
	}
	return token, nil
}

func (m *MockSDKTokenRepository) GetByUserID(userID uuid.UUID, includeRevoked bool) ([]*domain.SDKToken, error) {
	var result []*domain.SDKToken
	for _, token := range m.tokens {
		if token.UserID == userID {
			if includeRevoked || token.RevokedAt == nil {
				result = append(result, token)
			}
		}
	}
	return result, nil
}

func (m *MockSDKTokenRepository) GetByOrganizationID(organizationID uuid.UUID, includeRevoked bool) ([]*domain.SDKToken, error) {
	var result []*domain.SDKToken
	for _, token := range m.tokens {
		if token.OrganizationID == organizationID {
			if includeRevoked || token.RevokedAt == nil {
				result = append(result, token)
			}
		}
	}
	return result, nil
}

func (m *MockSDKTokenRepository) Update(token *domain.SDKToken) error {
	m.tokens[token.ID] = token
	return nil
}

func (m *MockSDKTokenRepository) Revoke(id uuid.UUID, reason string) error {
	if m.revokeErr != nil {
		return m.revokeErr
	}
	token, ok := m.tokens[id]
	if !ok {
		return errors.New("token not found")
	}
	token.Revoke(reason)
	return nil
}

func (m *MockSDKTokenRepository) RevokeByTokenHash(tokenHash string, reason string) error {
	if m.revokeErr != nil {
		return m.revokeErr
	}
	token, ok := m.tokensByHash[tokenHash]
	if !ok {
		return errors.New("token not found")
	}
	token.Revoke(reason)
	return nil
}

func (m *MockSDKTokenRepository) RevokeAllForUser(userID uuid.UUID, reason string) error {
	if m.revokeErr != nil {
		return m.revokeErr
	}
	for _, token := range m.tokens {
		if token.UserID == userID {
			token.Revoke(reason)
		}
	}
	return nil
}

func (m *MockSDKTokenRepository) RecordUsage(tokenID string, ipAddress string) error {
	token, ok := m.tokensByTokenID[tokenID]
	if !ok {
		return errors.New("token not found")
	}
	token.RecordUsage(ipAddress)
	return nil
}

func (m *MockSDKTokenRepository) DeleteExpired() error {
	if m.deleteExpiredErr != nil {
		return m.deleteExpiredErr
	}
	now := time.Now()
	for id, token := range m.tokens {
		if now.After(token.ExpiresAt) {
			delete(m.tokens, id)
			delete(m.tokensByHash, token.TokenHash)
			delete(m.tokensByTokenID, token.TokenID)
		}
	}
	return nil
}

func (m *MockSDKTokenRepository) GetActiveCount(userID uuid.UUID) (int, error) {
	count := 0
	for _, token := range m.tokens {
		if token.UserID == userID && token.IsActive() {
			count++
		}
	}
	return count, nil
}

// Helper to create a test token
func createTestSDKToken(userID, orgID uuid.UUID) *domain.SDKToken {
	return &domain.SDKToken{
		ID:             uuid.New(),
		UserID:         userID,
		OrganizationID: orgID,
		TokenHash:      "testhash_" + uuid.New().String(),
		TokenID:        "tokenid_" + uuid.New().String(),
		CreatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(24 * time.Hour),
	}
}

func TestNewSDKTokenService(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)

	assert.NotNil(t, service)
}

func TestSDKTokenService_GetUserTokens(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()

	userID := uuid.New()
	orgID := uuid.New()

	// Create active token
	activeToken := createTestSDKToken(userID, orgID)
	err := repo.Create(activeToken)
	require.NoError(t, err)

	// Create revoked token
	revokedToken := createTestSDKToken(userID, orgID)
	revokedToken.Revoke("test revocation")
	err = repo.Create(revokedToken)
	require.NoError(t, err)

	// Get only active tokens
	tokens, err := service.GetUserTokens(ctx, userID, false)
	require.NoError(t, err)
	assert.Len(t, tokens, 1)

	// Get all tokens including revoked
	tokens, err = service.GetUserTokens(ctx, userID, true)
	require.NoError(t, err)
	assert.Len(t, tokens, 2)
}

func TestSDKTokenService_GetOrganizationTokens(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()

	userID1 := uuid.New()
	userID2 := uuid.New()
	orgID := uuid.New()
	otherOrgID := uuid.New()

	// Create tokens for same org, different users
	token1 := createTestSDKToken(userID1, orgID)
	err := repo.Create(token1)
	require.NoError(t, err)

	token2 := createTestSDKToken(userID2, orgID)
	err = repo.Create(token2)
	require.NoError(t, err)

	// Create token for different org
	token3 := createTestSDKToken(userID1, otherOrgID)
	err = repo.Create(token3)
	require.NoError(t, err)

	// Get org tokens
	tokens, err := service.GetOrganizationTokens(ctx, orgID, false)
	require.NoError(t, err)
	assert.Len(t, tokens, 2)

	// Verify other org
	tokens, err = service.GetOrganizationTokens(ctx, otherOrgID, false)
	require.NoError(t, err)
	assert.Len(t, tokens, 1)
}

func TestSDKTokenService_RevokeToken(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()

	userID := uuid.New()
	orgID := uuid.New()

	token := createTestSDKToken(userID, orgID)
	err := repo.Create(token)
	require.NoError(t, err)

	// Revoke the token
	err = service.RevokeToken(ctx, token.ID, userID, "user requested")
	require.NoError(t, err)

	// Verify token is revoked
	assert.NotNil(t, token.RevokedAt)
	assert.Equal(t, "user requested", *token.RevokeReason)
}

func TestSDKTokenService_RevokeToken_NotFound(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()

	userID := uuid.New()
	nonExistentID := uuid.New()

	err := service.RevokeToken(ctx, nonExistentID, userID, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token not found")
}

func TestSDKTokenService_RevokeToken_WrongUser(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()

	ownerID := uuid.New()
	otherUserID := uuid.New()
	orgID := uuid.New()

	token := createTestSDKToken(ownerID, orgID)
	err := repo.Create(token)
	require.NoError(t, err)

	// Try to revoke with different user
	err = service.RevokeToken(ctx, token.ID, otherUserID, "unauthorized attempt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestSDKTokenService_RevokeByTokenHash(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()

	userID := uuid.New()
	orgID := uuid.New()

	token := createTestSDKToken(userID, orgID)
	err := repo.Create(token)
	require.NoError(t, err)

	// Revoke by hash
	err = service.RevokeByTokenHash(ctx, token.TokenHash, "token rotation")
	require.NoError(t, err)

	// Verify token is revoked
	assert.NotNil(t, token.RevokedAt)
}

func TestSDKTokenService_RevokeAllUserTokens(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()

	userID := uuid.New()
	orgID := uuid.New()

	// Create multiple tokens
	token1 := createTestSDKToken(userID, orgID)
	err := repo.Create(token1)
	require.NoError(t, err)

	token2 := createTestSDKToken(userID, orgID)
	err = repo.Create(token2)
	require.NoError(t, err)

	// Revoke all
	err = service.RevokeAllUserTokens(ctx, userID, "security incident")
	require.NoError(t, err)

	// Verify all are revoked
	assert.NotNil(t, token1.RevokedAt)
	assert.NotNil(t, token2.RevokedAt)
}

func TestSDKTokenService_GetActiveTokenCount(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()

	userID := uuid.New()
	orgID := uuid.New()

	// Initially zero
	count, err := service.GetActiveTokenCount(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Create active token
	token1 := createTestSDKToken(userID, orgID)
	err = repo.Create(token1)
	require.NoError(t, err)

	count, err = service.GetActiveTokenCount(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Create and revoke another token
	token2 := createTestSDKToken(userID, orgID)
	token2.Revoke("test")
	err = repo.Create(token2)
	require.NoError(t, err)

	// Count should still be 1
	count, err = service.GetActiveTokenCount(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestSDKTokenService_RecordTokenUsage(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()

	userID := uuid.New()
	orgID := uuid.New()

	token := createTestSDKToken(userID, orgID)
	err := repo.Create(token)
	require.NoError(t, err)

	// Record usage
	err = service.RecordTokenUsage(ctx, token.TokenID, "192.168.1.100")
	require.NoError(t, err)

	// Verify usage was recorded
	assert.NotNil(t, token.LastUsedAt)
	assert.Equal(t, "192.168.1.100", *token.LastIPAddress)
	assert.Equal(t, 1, token.UsageCount)
}

func TestSDKTokenService_ValidateToken(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()

	userID := uuid.New()
	orgID := uuid.New()

	// Create active token
	activeToken := createTestSDKToken(userID, orgID)
	err := repo.Create(activeToken)
	require.NoError(t, err)

	// Validate active token
	validated, err := service.ValidateToken(ctx, activeToken.TokenHash)
	require.NoError(t, err)
	assert.Equal(t, activeToken.ID, validated.ID)
}

func TestSDKTokenService_ValidateToken_Revoked(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()

	userID := uuid.New()
	orgID := uuid.New()

	// Create revoked token
	revokedToken := createTestSDKToken(userID, orgID)
	revokedToken.Revoke("test")
	err := repo.Create(revokedToken)
	require.NoError(t, err)

	// Validate should fail
	_, err = service.ValidateToken(ctx, revokedToken.TokenHash)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revoked or expired")
}

func TestSDKTokenService_ValidateToken_Expired(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()

	userID := uuid.New()
	orgID := uuid.New()

	// Create expired token
	expiredToken := createTestSDKToken(userID, orgID)
	expiredToken.ExpiresAt = time.Now().Add(-1 * time.Hour)
	err := repo.Create(expiredToken)
	require.NoError(t, err)

	// Validate should fail
	_, err = service.ValidateToken(ctx, expiredToken.TokenHash)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revoked or expired")
}

func TestSDKTokenService_ValidateToken_NotFound(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()

	_, err := service.ValidateToken(ctx, "nonexistent_hash")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token not found")
}

func TestSDKTokenService_CreateToken(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()

	userID := uuid.New()
	orgID := uuid.New()

	token := createTestSDKToken(userID, orgID)
	err := service.CreateToken(ctx, token)
	require.NoError(t, err)

	// Verify token was stored
	stored, err := repo.GetByID(token.ID)
	require.NoError(t, err)
	assert.Equal(t, token.ID, stored.ID)
}

func TestSDKTokenService_GetByTokenHash(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()

	userID := uuid.New()
	orgID := uuid.New()

	token := createTestSDKToken(userID, orgID)
	err := repo.Create(token)
	require.NoError(t, err)

	// Get by hash
	retrieved, err := service.GetByTokenHash(ctx, token.TokenHash)
	require.NoError(t, err)
	assert.Equal(t, token.ID, retrieved.ID)
}

func TestSDKTokenService_GetByTokenHash_NotFound(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()

	_, err := service.GetByTokenHash(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestSDKTokenService_CleanupExpiredTokens(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()

	userID := uuid.New()
	orgID := uuid.New()

	// Create active token
	activeToken := createTestSDKToken(userID, orgID)
	err := repo.Create(activeToken)
	require.NoError(t, err)

	// Create expired token
	expiredToken := createTestSDKToken(userID, orgID)
	expiredToken.ExpiresAt = time.Now().Add(-1 * time.Hour)
	err = repo.Create(expiredToken)
	require.NoError(t, err)

	// Cleanup
	err = service.CleanupExpiredTokens(ctx)
	require.NoError(t, err)

	// Active token should still exist
	_, err = repo.GetByID(activeToken.ID)
	require.NoError(t, err)

	// Expired token should be gone
	_, err = repo.GetByID(expiredToken.ID)
	assert.Error(t, err)
}

func TestSDKTokenService_CreateToken_Error(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	repo.createErr = errors.New("database error")
	service := NewSDKTokenService(repo)
	ctx := context.Background()

	token := createTestSDKToken(uuid.New(), uuid.New())
	err := service.CreateToken(ctx, token)
	assert.Error(t, err)
	assert.Equal(t, "database error", err.Error())
}

func TestSDKTokenService_CleanupExpiredTokens_Error(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	repo.deleteExpiredErr = errors.New("cleanup failed")
	service := NewSDKTokenService(repo)
	ctx := context.Background()

	err := service.CleanupExpiredTokens(ctx)
	assert.Error(t, err)
	assert.Equal(t, "cleanup failed", err.Error())
}
