package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOAuthProvider_Constants(t *testing.T) {
	tests := []struct {
		constant OAuthProvider
		expected string
	}{
		{OAuthProviderGoogle, "google"},
		{OAuthProviderMicrosoft, "microsoft"},
		{OAuthProviderOkta, "okta"},
		{OAuthProviderLocal, "local"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, OAuthProvider(tt.expected), tt.constant)
		})
	}
}

func TestRegistrationRequestStatus_Constants(t *testing.T) {
	tests := []struct {
		constant RegistrationRequestStatus
		expected string
	}{
		{RegistrationStatusPending, "pending"},
		{RegistrationStatusApproved, "approved"},
		{RegistrationStatusRejected, "rejected"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, RegistrationRequestStatus(tt.expected), tt.constant)
		})
	}
}

func TestUserRegistrationRequest_Structure(t *testing.T) {
	now := time.Now()
	provider := OAuthProviderGoogle
	oauthUserID := "google-user-123"
	orgID := uuid.New()
	pictureURL := "https://example.com/photo.jpg"

	request := UserRegistrationRequest{
		ID:                 uuid.New(),
		Email:              "user@example.com",
		FirstName:          "John",
		LastName:           "Doe",
		OAuthProvider:      &provider,
		OAuthUserID:        &oauthUserID,
		OrganizationID:     &orgID,
		Status:             RegistrationStatusPending,
		RequestedAt:        now,
		ProfilePictureURL:  &pictureURL,
		OAuthEmailVerified: true,
		Metadata: map[string]interface{}{
			"locale": "en",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	assert.NotEqual(t, uuid.Nil, request.ID)
	assert.Equal(t, "user@example.com", request.Email)
	assert.Equal(t, "John", request.FirstName)
	assert.Equal(t, "Doe", request.LastName)
	assert.Equal(t, OAuthProviderGoogle, *request.OAuthProvider)
	assert.Equal(t, RegistrationStatusPending, request.Status)
	assert.True(t, request.OAuthEmailVerified)
	assert.Nil(t, request.PasswordHash)
}

func TestUserRegistrationRequest_ManualRegistration(t *testing.T) {
	now := time.Now()
	passwordHash := "hashed_password_123"
	provider := OAuthProviderLocal

	request := UserRegistrationRequest{
		ID:                 uuid.New(),
		Email:              "user@example.com",
		FirstName:          "Jane",
		LastName:           "Smith",
		PasswordHash:       &passwordHash,
		OAuthProvider:      &provider,
		Status:             RegistrationStatusPending,
		RequestedAt:        now,
		OAuthEmailVerified: false,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	assert.NotNil(t, request.PasswordHash)
	assert.Equal(t, OAuthProviderLocal, *request.OAuthProvider)
	assert.False(t, request.OAuthEmailVerified)
	assert.Nil(t, request.OAuthUserID)
}

func TestNewUserRegistrationRequestOAuth(t *testing.T) {
	profile := &OAuthProfile{
		ProviderUserID: "google-123",
		Email:          "oauth@example.com",
		EmailVerified:  true,
		FirstName:      "OAuth",
		LastName:       "User",
		PictureURL:     "https://example.com/photo.jpg",
		RawProfile: map[string]interface{}{
			"sub": "google-123",
		},
	}

	request := NewUserRegistrationRequestOAuth(
		"oauth@example.com",
		"OAuth",
		"User",
		OAuthProviderGoogle,
		"google-123",
		profile,
	)

	assert.NotEqual(t, uuid.Nil, request.ID)
	assert.Equal(t, "oauth@example.com", request.Email)
	assert.Equal(t, "OAuth", request.FirstName)
	assert.Equal(t, "User", request.LastName)
	assert.Equal(t, OAuthProviderGoogle, *request.OAuthProvider)
	assert.Equal(t, "google-123", *request.OAuthUserID)
	assert.Equal(t, RegistrationStatusPending, request.Status)
	assert.True(t, request.OAuthEmailVerified)
	assert.NotNil(t, request.ProfilePictureURL)
	assert.Equal(t, "https://example.com/photo.jpg", *request.ProfilePictureURL)
}

func TestNewUserRegistrationRequestOAuth_NoPicture(t *testing.T) {
	profile := &OAuthProfile{
		ProviderUserID: "google-456",
		Email:          "nopic@example.com",
		EmailVerified:  true,
		FirstName:      "No",
		LastName:       "Picture",
		PictureURL:     "", // Empty
	}

	request := NewUserRegistrationRequestOAuth(
		"nopic@example.com",
		"No",
		"Picture",
		OAuthProviderGoogle,
		"google-456",
		profile,
	)

	assert.Nil(t, request.ProfilePictureURL)
}

func TestNewUserRegistrationRequestManual(t *testing.T) {
	request := NewUserRegistrationRequestManual(
		"manual@example.com",
		"Manual",
		"User",
		"hashed_password",
	)

	assert.NotEqual(t, uuid.Nil, request.ID)
	assert.Equal(t, "manual@example.com", request.Email)
	assert.Equal(t, "Manual", request.FirstName)
	assert.Equal(t, "User", request.LastName)
	assert.NotNil(t, request.PasswordHash)
	assert.Equal(t, "hashed_password", *request.PasswordHash)
	assert.Equal(t, OAuthProviderLocal, *request.OAuthProvider)
	assert.Nil(t, request.OAuthUserID)
	assert.False(t, request.OAuthEmailVerified)
	assert.Equal(t, RegistrationStatusPending, request.Status)
}

func TestUserRegistrationRequest_Approve(t *testing.T) {
	request := NewUserRegistrationRequestManual(
		"test@example.com",
		"Test",
		"User",
		"password",
	)

	// Initially pending
	assert.True(t, request.IsPending())
	assert.False(t, request.IsApproved())
	assert.Nil(t, request.ReviewedAt)
	assert.Nil(t, request.ReviewedBy)

	// Approve
	reviewerID := uuid.New()
	request.Approve(reviewerID)

	assert.False(t, request.IsPending())
	assert.True(t, request.IsApproved())
	assert.False(t, request.IsRejected())
	assert.NotNil(t, request.ReviewedAt)
	assert.Equal(t, &reviewerID, request.ReviewedBy)
	assert.Equal(t, RegistrationStatusApproved, request.Status)
}

func TestUserRegistrationRequest_Reject(t *testing.T) {
	request := NewUserRegistrationRequestManual(
		"test@example.com",
		"Test",
		"User",
		"password",
	)

	// Initially pending
	assert.True(t, request.IsPending())
	assert.Nil(t, request.RejectionReason)

	// Reject
	reviewerID := uuid.New()
	request.Reject(reviewerID, "Invalid email domain")

	assert.False(t, request.IsPending())
	assert.False(t, request.IsApproved())
	assert.True(t, request.IsRejected())
	assert.NotNil(t, request.ReviewedAt)
	assert.Equal(t, &reviewerID, request.ReviewedBy)
	assert.NotNil(t, request.RejectionReason)
	assert.Equal(t, "Invalid email domain", *request.RejectionReason)
	assert.Equal(t, RegistrationStatusRejected, request.Status)
}

func TestUserRegistrationRequest_IsPending(t *testing.T) {
	request := UserRegistrationRequest{Status: RegistrationStatusPending}
	assert.True(t, request.IsPending())

	request.Status = RegistrationStatusApproved
	assert.False(t, request.IsPending())

	request.Status = RegistrationStatusRejected
	assert.False(t, request.IsPending())
}

func TestUserRegistrationRequest_IsApproved(t *testing.T) {
	request := UserRegistrationRequest{Status: RegistrationStatusApproved}
	assert.True(t, request.IsApproved())

	request.Status = RegistrationStatusPending
	assert.False(t, request.IsApproved())

	request.Status = RegistrationStatusRejected
	assert.False(t, request.IsApproved())
}

func TestUserRegistrationRequest_IsRejected(t *testing.T) {
	request := UserRegistrationRequest{Status: RegistrationStatusRejected}
	assert.True(t, request.IsRejected())

	request.Status = RegistrationStatusPending
	assert.False(t, request.IsRejected())

	request.Status = RegistrationStatusApproved
	assert.False(t, request.IsRejected())
}

func TestOAuthConnection_Structure(t *testing.T) {
	now := time.Now()
	tokenExpiresAt := now.Add(time.Hour)
	lastUsedAt := now.Add(-10 * time.Minute)

	connection := OAuthConnection{
		ID:               uuid.New(),
		UserID:           uuid.New(),
		Provider:         OAuthProviderGoogle,
		ProviderUserID:   "google-user-789",
		ProviderEmail:    "user@gmail.com",
		AccessTokenHash:  "hashed_access_token",
		RefreshTokenHash: "hashed_refresh_token",
		TokenExpiresAt:   &tokenExpiresAt,
		ProfileData: map[string]interface{}{
			"name":   "Test User",
			"locale": "en",
		},
		LastUsedAt: &lastUsedAt,
		CreatedAt:  now.Add(-24 * time.Hour),
		UpdatedAt:  now,
	}

	assert.NotEqual(t, uuid.Nil, connection.ID)
	assert.NotEqual(t, uuid.Nil, connection.UserID)
	assert.Equal(t, OAuthProviderGoogle, connection.Provider)
	assert.Equal(t, "google-user-789", connection.ProviderUserID)
	assert.Equal(t, "user@gmail.com", connection.ProviderEmail)
	assert.NotEmpty(t, connection.AccessTokenHash)
	assert.NotEmpty(t, connection.RefreshTokenHash)
	assert.NotNil(t, connection.TokenExpiresAt)
	assert.NotNil(t, connection.LastUsedAt)
}

func TestOAuthConnection_MicrosoftProvider(t *testing.T) {
	connection := OAuthConnection{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		Provider:       OAuthProviderMicrosoft,
		ProviderUserID: "ms-user-123",
		ProviderEmail:  "user@outlook.com",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	assert.Equal(t, OAuthProviderMicrosoft, connection.Provider)
}

func TestOAuthConnection_OktaProvider(t *testing.T) {
	connection := OAuthConnection{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		Provider:       OAuthProviderOkta,
		ProviderUserID: "okta-user-456",
		ProviderEmail:  "user@company.com",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	assert.Equal(t, OAuthProviderOkta, connection.Provider)
}

func TestOAuthProfile_Structure(t *testing.T) {
	profile := OAuthProfile{
		ProviderUserID: "provider-123",
		Email:          "profile@example.com",
		EmailVerified:  true,
		FirstName:      "First",
		LastName:       "Last",
		FullName:       "First Last",
		PictureURL:     "https://example.com/photo.jpg",
		Locale:         "en-US",
		RawProfile: map[string]interface{}{
			"sub":            "provider-123",
			"email":          "profile@example.com",
			"email_verified": true,
		},
	}

	assert.Equal(t, "provider-123", profile.ProviderUserID)
	assert.Equal(t, "profile@example.com", profile.Email)
	assert.True(t, profile.EmailVerified)
	assert.Equal(t, "First", profile.FirstName)
	assert.Equal(t, "Last", profile.LastName)
	assert.Equal(t, "First Last", profile.FullName)
	assert.Equal(t, "en-US", profile.Locale)
	assert.NotNil(t, profile.RawProfile)
}

func TestUserRegistrationRequest_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	provider := OAuthProviderGoogle
	oauthUserID := "google-123"

	request := UserRegistrationRequest{
		ID:                 uuid.New(),
		Email:              "test@example.com",
		FirstName:          "Test",
		LastName:           "User",
		OAuthProvider:      &provider,
		OAuthUserID:        &oauthUserID,
		Status:             RegistrationStatusPending,
		RequestedAt:        now,
		OAuthEmailVerified: true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	// Serialize to JSON
	data, err := json.Marshal(request)
	require.NoError(t, err)

	// Verify password hash is not in JSON
	assert.NotContains(t, string(data), "password")

	// Deserialize back
	var restored UserRegistrationRequest
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, request.ID, restored.ID)
	assert.Equal(t, request.Email, restored.Email)
	assert.Equal(t, request.Status, restored.Status)
	assert.Equal(t, request.OAuthEmailVerified, restored.OAuthEmailVerified)
}

func TestOAuthConnection_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	connection := OAuthConnection{
		ID:               uuid.New(),
		UserID:           uuid.New(),
		Provider:         OAuthProviderGoogle,
		ProviderUserID:   "google-456",
		ProviderEmail:    "test@gmail.com",
		AccessTokenHash:  "secret_hash",
		RefreshTokenHash: "refresh_secret",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	// Serialize to JSON
	data, err := json.Marshal(connection)
	require.NoError(t, err)

	// Verify tokens are not in JSON (marked with json:"-")
	assert.NotContains(t, string(data), "secret_hash")
	assert.NotContains(t, string(data), "refresh_secret")

	// Deserialize back
	var restored OAuthConnection
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, connection.ID, restored.ID)
	assert.Equal(t, connection.Provider, restored.Provider)
	assert.Equal(t, connection.ProviderEmail, restored.ProviderEmail)
}

func TestOAuthProvider_AllProviders(t *testing.T) {
	providers := []OAuthProvider{
		OAuthProviderGoogle,
		OAuthProviderMicrosoft,
		OAuthProviderOkta,
		OAuthProviderLocal,
	}

	for _, provider := range providers {
		t.Run(string(provider), func(t *testing.T) {
			assert.NotEmpty(t, string(provider))
		})
	}
}
