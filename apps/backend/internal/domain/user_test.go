package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestUserRoleConstants(t *testing.T) {
	assert.Equal(t, UserRole("admin"), RoleAdmin)
	assert.Equal(t, UserRole("manager"), RoleManager)
	assert.Equal(t, UserRole("member"), RoleMember)
	assert.Equal(t, UserRole("viewer"), RoleViewer)
}

func TestUserStatusConstants(t *testing.T) {
	assert.Equal(t, UserStatus("pending"), UserStatusPending)
	assert.Equal(t, UserStatus("active"), UserStatusActive)
	assert.Equal(t, UserStatus("suspended"), UserStatusSuspended)
	assert.Equal(t, UserStatus("deactivated"), UserStatusDeactivated)
}

func TestUserStruct(t *testing.T) {
	now := time.Now()
	userID := uuid.New()
	orgID := uuid.New()
	avatarURL := "https://example.com/avatar.jpg"
	passwordHash := "hashed_password"

	user := User{
		ID:                  userID,
		OrganizationID:      orgID,
		Email:               "test@example.com",
		Name:                "Test User",
		AvatarURL:           &avatarURL,
		Role:                RoleAdmin,
		Provider:            "local",
		ProviderID:          "local_123",
		Status:              UserStatusActive,
		PasswordHash:        &passwordHash,
		ForcePasswordChange: false,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	assert.Equal(t, userID, user.ID)
	assert.Equal(t, orgID, user.OrganizationID)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, &avatarURL, user.AvatarURL)
	assert.Equal(t, RoleAdmin, user.Role)
	assert.Equal(t, "local", user.Provider)
	assert.Equal(t, UserStatusActive, user.Status)
	assert.False(t, user.ForcePasswordChange)
}

func TestUserWithOAuthProvider(t *testing.T) {
	user := User{
		ID:         uuid.New(),
		Email:      "oauth@example.com",
		Name:       "OAuth User",
		Role:       RoleMember,
		Provider:   "google",
		ProviderID: "google_abc123",
		Status:     UserStatusActive,
	}

	assert.Equal(t, "google", user.Provider)
	assert.Equal(t, "google_abc123", user.ProviderID)
	assert.Nil(t, user.PasswordHash) // OAuth users don't have passwords
}

func TestUserApprovalFields(t *testing.T) {
	now := time.Now()
	adminID := uuid.New()

	user := User{
		ID:         uuid.New(),
		Email:      "pending@example.com",
		Status:     UserStatusPending,
		ApprovedBy: nil,
		ApprovedAt: nil,
	}

	// Initially pending
	assert.Nil(t, user.ApprovedBy)
	assert.Nil(t, user.ApprovedAt)

	// After approval
	user.Status = UserStatusActive
	user.ApprovedBy = &adminID
	user.ApprovedAt = &now

	assert.Equal(t, &adminID, user.ApprovedBy)
	assert.Equal(t, &now, user.ApprovedAt)
	assert.Equal(t, UserStatusActive, user.Status)
}

func TestUserPasswordResetFields(t *testing.T) {
	now := time.Now()
	expires := now.Add(24 * time.Hour)
	resetToken := "reset_token_123"

	user := User{
		ID:                     uuid.New(),
		Email:                  "reset@example.com",
		PasswordResetToken:     &resetToken,
		PasswordResetExpiresAt: &expires,
	}

	assert.Equal(t, &resetToken, user.PasswordResetToken)
	assert.Equal(t, &expires, user.PasswordResetExpiresAt)
}

func TestUserSoftDelete(t *testing.T) {
	now := time.Now()

	user := User{
		ID:        uuid.New(),
		Email:     "deleted@example.com",
		Status:    UserStatusDeactivated,
		DeletedAt: &now,
	}

	assert.Equal(t, UserStatusDeactivated, user.Status)
	assert.NotNil(t, user.DeletedAt)
}

func TestUserRoleStringConversion(t *testing.T) {
	role := RoleAdmin
	str := string(role)
	assert.Equal(t, "admin", str)

	// Convert back
	converted := UserRole(str)
	assert.Equal(t, RoleAdmin, converted)
}

func TestUserStatusStringConversion(t *testing.T) {
	status := UserStatusActive
	str := string(status)
	assert.Equal(t, "active", str)

	// Convert back
	converted := UserStatus(str)
	assert.Equal(t, UserStatusActive, converted)
}

func TestUserWithNilOptionalFields(t *testing.T) {
	user := User{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Email:          "minimal@example.com",
		Name:           "Minimal User",
		Role:           RoleMember,
		Provider:       "local",
		Status:         UserStatusPending,
	}

	// Verify nil optional fields don't cause issues
	assert.Nil(t, user.AvatarURL)
	assert.Nil(t, user.PasswordHash)
	assert.Nil(t, user.PasswordResetToken)
	assert.Nil(t, user.PasswordResetExpiresAt)
	assert.Nil(t, user.ApprovedBy)
	assert.Nil(t, user.ApprovedAt)
	assert.Nil(t, user.LastLoginAt)
	assert.Nil(t, user.DeletedAt)
}
