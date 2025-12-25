package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// GenerateState Tests
// ===========================

func TestGenerateState_ReturnsUUID(t *testing.T) {
	state := GenerateState()

	assert.NotEmpty(t, state)
	// UUID format: 8-4-4-4-12 hex digits with dashes
	assert.Len(t, state, 36)
	assert.Contains(t, state, "-")
}

func TestGenerateState_UniqueValues(t *testing.T) {
	state1 := GenerateState()
	state2 := GenerateState()

	assert.NotEqual(t, state1, state2, "Each state should be unique")
}

// ===========================
// HashAPIKey Tests
// ===========================

func TestHashAPIKey_Success(t *testing.T) {
	apiKey := "test-api-key-12345"

	hash, err := HashAPIKey(apiKey)

	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, apiKey, hash, "Hash should be different from original key")
}

func TestHashAPIKey_DifferentHashes(t *testing.T) {
	apiKey := "test-api-key-12345"

	hash1, err := HashAPIKey(apiKey)
	require.NoError(t, err)

	hash2, err := HashAPIKey(apiKey)
	require.NoError(t, err)

	// bcrypt produces different hashes each time due to random salt
	assert.NotEqual(t, hash1, hash2, "Each hash should include unique salt")
}

// ===========================
// CompareAPIKey Tests
// ===========================

func TestCompareAPIKey_ValidKey(t *testing.T) {
	apiKey := "test-api-key-12345"

	hash, err := HashAPIKey(apiKey)
	require.NoError(t, err)

	err = CompareAPIKey(hash, apiKey)
	assert.NoError(t, err)
}

func TestCompareAPIKey_InvalidKey(t *testing.T) {
	apiKey := "test-api-key-12345"

	hash, err := HashAPIKey(apiKey)
	require.NoError(t, err)

	err = CompareAPIKey(hash, "wrong-api-key")
	assert.Error(t, err)
}

func TestCompareAPIKey_EmptyKey(t *testing.T) {
	apiKey := "test-api-key-12345"

	hash, err := HashAPIKey(apiKey)
	require.NoError(t, err)

	err = CompareAPIKey(hash, "")
	assert.Error(t, err)
}

// ===========================
// getStringField Tests
// ===========================

func TestGetStringField_Exists(t *testing.T) {
	m := map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
	}

	result := getStringField(m, "name")
	assert.Equal(t, "John Doe", result)
}

func TestGetStringField_NotExists(t *testing.T) {
	m := map[string]interface{}{
		"name": "John Doe",
	}

	result := getStringField(m, "email")
	assert.Empty(t, result)
}

func TestGetStringField_NotString(t *testing.T) {
	m := map[string]interface{}{
		"age": 30,
	}

	result := getStringField(m, "age")
	assert.Empty(t, result, "Non-string values should return empty string")
}

func TestGetStringField_EmptyMap(t *testing.T) {
	m := map[string]interface{}{}

	result := getStringField(m, "anything")
	assert.Empty(t, result)
}

// ===========================
// NewOAuthService Tests
// ===========================

func TestNewOAuthService_ReturnsService(t *testing.T) {
	service := NewOAuthService()

	assert.NotNil(t, service)
	assert.NotNil(t, service.configs)
	assert.Contains(t, service.configs, ProviderGoogle)
	assert.Contains(t, service.configs, ProviderMicrosoft)
	assert.Contains(t, service.configs, ProviderOkta)
}

// ===========================
// GetAuthURL Tests
// ===========================

func TestGetAuthURL_Google(t *testing.T) {
	service := NewOAuthService()
	state := "test-state-123"

	url, err := service.GetAuthURL(ProviderGoogle, state)

	assert.NoError(t, err)
	assert.Contains(t, url, "accounts.google.com")
	assert.Contains(t, url, "state="+state)
	assert.Contains(t, url, "openid")
	assert.Contains(t, url, "email")
	assert.Contains(t, url, "profile")
}

func TestGetAuthURL_Microsoft(t *testing.T) {
	service := NewOAuthService()
	state := "test-state-456"

	url, err := service.GetAuthURL(ProviderMicrosoft, state)

	assert.NoError(t, err)
	assert.Contains(t, url, "login.microsoftonline.com")
	assert.Contains(t, url, "state="+state)
	assert.Contains(t, url, "User.Read")
}

func TestGetAuthURL_Okta(t *testing.T) {
	service := NewOAuthService()
	state := "test-state-789"

	url, err := service.GetAuthURL(ProviderOkta, state)

	assert.NoError(t, err)
	assert.Contains(t, url, "state="+state)
}

func TestGetAuthURL_UnsupportedProvider(t *testing.T) {
	service := NewOAuthService()

	url, err := service.GetAuthURL(OAuthProvider("github"), "state")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported provider")
	assert.Empty(t, url)
}

// ===========================
// parseUserInfo Tests
// ===========================

func TestParseUserInfo_Google(t *testing.T) {
	service := NewOAuthService()

	info := map[string]interface{}{
		"id":      "google-user-123",
		"email":   "user@gmail.com",
		"name":    "Google User",
		"picture": "https://example.com/avatar.jpg",
	}

	user, err := service.parseUserInfo(ProviderGoogle, info)

	assert.NoError(t, err)
	assert.Equal(t, "google-user-123", user.ID)
	assert.Equal(t, "user@gmail.com", user.Email)
	assert.Equal(t, "Google User", user.Name)
	assert.Equal(t, "https://example.com/avatar.jpg", user.AvatarURL)
	assert.Equal(t, "google", user.Provider)
}

func TestParseUserInfo_Microsoft_WithPrincipalName(t *testing.T) {
	service := NewOAuthService()

	info := map[string]interface{}{
		"id":                "ms-user-123",
		"userPrincipalName": "user@microsoft.com",
		"displayName":       "Microsoft User",
	}

	user, err := service.parseUserInfo(ProviderMicrosoft, info)

	assert.NoError(t, err)
	assert.Equal(t, "ms-user-123", user.ID)
	assert.Equal(t, "user@microsoft.com", user.Email)
	assert.Equal(t, "Microsoft User", user.Name)
	assert.Equal(t, "microsoft", user.Provider)
}

func TestParseUserInfo_Microsoft_WithMail(t *testing.T) {
	service := NewOAuthService()

	info := map[string]interface{}{
		"id":          "ms-user-456",
		"mail":        "user@outlook.com",
		"displayName": "Outlook User",
	}

	user, err := service.parseUserInfo(ProviderMicrosoft, info)

	assert.NoError(t, err)
	assert.Equal(t, "ms-user-456", user.ID)
	assert.Equal(t, "user@outlook.com", user.Email)
}

func TestParseUserInfo_Okta(t *testing.T) {
	service := NewOAuthService()

	info := map[string]interface{}{
		"sub":   "okta-user-789",
		"email": "user@okta.com",
		"name":  "Okta User",
	}

	user, err := service.parseUserInfo(ProviderOkta, info)

	assert.NoError(t, err)
	assert.Equal(t, "okta-user-789", user.ID)
	assert.Equal(t, "user@okta.com", user.Email)
	assert.Equal(t, "Okta User", user.Name)
	assert.Equal(t, "okta", user.Provider)
}

func TestParseUserInfo_MissingID(t *testing.T) {
	service := NewOAuthService()

	info := map[string]interface{}{
		"email": "user@gmail.com",
		"name":  "No ID User",
	}

	user, err := service.parseUserInfo(ProviderGoogle, info)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required user fields")
	assert.Nil(t, user)
}

func TestParseUserInfo_MissingEmail(t *testing.T) {
	service := NewOAuthService()

	info := map[string]interface{}{
		"id":   "user-123",
		"name": "No Email User",
	}

	user, err := service.parseUserInfo(ProviderGoogle, info)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required user fields")
	assert.Nil(t, user)
}

func TestParseUserInfo_EmptyInfo(t *testing.T) {
	service := NewOAuthService()

	info := map[string]interface{}{}

	user, err := service.parseUserInfo(ProviderGoogle, info)

	assert.Error(t, err)
	assert.Nil(t, user)
}

// ===========================
// OAuthProvider Constants Tests
// ===========================

func TestOAuthProviderConstants(t *testing.T) {
	assert.Equal(t, OAuthProvider("google"), ProviderGoogle)
	assert.Equal(t, OAuthProvider("microsoft"), ProviderMicrosoft)
	assert.Equal(t, OAuthProvider("okta"), ProviderOkta)
}
