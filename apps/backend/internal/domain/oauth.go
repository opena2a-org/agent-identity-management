package domain

import (
	"time"

	"github.com/google/uuid"
)

// OAuthProvider represents supported OAuth providers
type OAuthProvider string

const (
	OAuthProviderGoogle    OAuthProvider = "google"
	OAuthProviderMicrosoft OAuthProvider = "microsoft"
	OAuthProviderOkta      OAuthProvider = "okta"
	OAuthProviderLocal     OAuthProvider = "local" // For email/password registrations
)

// RegistrationRequestStatus represents the status of a registration request
type RegistrationRequestStatus string

const (
	RegistrationStatusPending  RegistrationRequestStatus = "pending"
	RegistrationStatusApproved RegistrationRequestStatus = "approved"
	RegistrationStatusRejected RegistrationRequestStatus = "rejected"
)

// UserRegistrationRequest represents a user's request to register via OAuth or email/password
type UserRegistrationRequest struct {
	ID                   uuid.UUID                 `json:"id" db:"id"`
	Email                string                    `json:"email" db:"email"`
	FirstName            string                    `json:"firstName" db:"first_name"`
	LastName             string                    `json:"lastName" db:"last_name"`
	PasswordHash         *string                   `json:"-" db:"password_hash"` // Only for email/password registrations
	OAuthProvider        *OAuthProvider            `json:"oauthProvider,omitempty" db:"oauth_provider"` // Nullable for manual registrations
	OAuthUserID          *string                   `json:"oauthUserId,omitempty" db:"oauth_user_id"` // Nullable for manual registrations
	OrganizationID       *uuid.UUID                `json:"organizationId,omitempty" db:"organization_id"`
	Status               RegistrationRequestStatus `json:"status" db:"status"`
	RequestedAt          time.Time                 `json:"requestedAt" db:"requested_at"`
	ReviewedAt           *time.Time                `json:"reviewedAt,omitempty" db:"reviewed_at"`
	ReviewedBy           *uuid.UUID                `json:"reviewedBy,omitempty" db:"reviewed_by"`
	RejectionReason      *string                   `json:"rejectionReason,omitempty" db:"rejection_reason"`
	ProfilePictureURL    *string                   `json:"profilePictureUrl,omitempty" db:"profile_picture_url"`
	OAuthEmailVerified   bool                      `json:"oauthEmailVerified" db:"oauth_email_verified"`
	Metadata             map[string]interface{}    `json:"metadata,omitempty" db:"metadata"`
	CreatedAt            time.Time                 `json:"createdAt" db:"created_at"`
	UpdatedAt            time.Time                 `json:"updatedAt" db:"updated_at"`
}

// OAuthConnection represents an OAuth connection for a user
type OAuthConnection struct {
	ID                uuid.UUID              `json:"id" db:"id"`
	UserID            uuid.UUID              `json:"userId" db:"user_id"`
	Provider          OAuthProvider          `json:"provider" db:"provider"`
	ProviderUserID    string                 `json:"providerUserId" db:"provider_user_id"`
	ProviderEmail     string                 `json:"providerEmail" db:"provider_email"`
	AccessTokenHash   string                 `json:"-" db:"access_token_hash"` // Never expose in JSON
	RefreshTokenHash  string                 `json:"-" db:"refresh_token_hash"` // Never expose in JSON
	TokenExpiresAt    *time.Time             `json:"tokenExpiresAt,omitempty" db:"token_expires_at"`
	ProfileData       map[string]interface{} `json:"profileData,omitempty" db:"profile_data"`
	LastUsedAt        *time.Time             `json:"lastUsedAt,omitempty" db:"last_used_at"`
	CreatedAt         time.Time              `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time              `json:"updatedAt" db:"updated_at"`
}

// OAuthProfile represents user profile data from OAuth provider
type OAuthProfile struct {
	ProviderUserID string
	Email          string
	EmailVerified  bool
	FirstName      string
	LastName       string
	FullName       string
	PictureURL     string
	Locale         string
	RawProfile     map[string]interface{}
}

// NewUserRegistrationRequestOAuth creates a new OAuth registration request
func NewUserRegistrationRequestOAuth(
	email, firstName, lastName string,
	provider OAuthProvider,
	providerUserID string,
	profile *OAuthProfile,
) *UserRegistrationRequest {
	now := time.Now()

	req := &UserRegistrationRequest{
		ID:                 uuid.New(),
		Email:              email,
		FirstName:          firstName,
		LastName:           lastName,
		OAuthProvider:      &provider,
		OAuthUserID:        &providerUserID,
		Status:             RegistrationStatusPending,
		RequestedAt:        now,
		OAuthEmailVerified: profile.EmailVerified,
		Metadata:           profile.RawProfile,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if profile.PictureURL != "" {
		req.ProfilePictureURL = &profile.PictureURL
	}

	return req
}

// NewUserRegistrationRequestManual creates a new manual (email/password) registration request
func NewUserRegistrationRequestManual(
	email, firstName, lastName string,
	passwordHash string,
) *UserRegistrationRequest {
	now := time.Now()
	localProvider := OAuthProviderLocal

	return &UserRegistrationRequest{
		ID:                 uuid.New(),
		Email:              email,
		FirstName:          firstName,
		LastName:           lastName,
		PasswordHash:       &passwordHash,
		OAuthProvider:      &localProvider, // Mark as local/email authentication
		OAuthUserID:        nil,
		Status:             RegistrationStatusPending,
		RequestedAt:        now,
		OAuthEmailVerified: false, // Manual registrations require email verification
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

// Approve marks the registration request as approved
func (r *UserRegistrationRequest) Approve(reviewerID uuid.UUID) {
	now := time.Now()
	r.Status = RegistrationStatusApproved
	r.ReviewedAt = &now
	r.ReviewedBy = &reviewerID
	r.UpdatedAt = now
}

// Reject marks the registration request as rejected
func (r *UserRegistrationRequest) Reject(reviewerID uuid.UUID, reason string) {
	now := time.Now()
	r.Status = RegistrationStatusRejected
	r.ReviewedAt = &now
	r.ReviewedBy = &reviewerID
	r.RejectionReason = &reason
	r.UpdatedAt = now
}

// IsPending checks if the request is pending review
func (r *UserRegistrationRequest) IsPending() bool {
	return r.Status == RegistrationStatusPending
}

// IsApproved checks if the request has been approved
func (r *UserRegistrationRequest) IsApproved() bool {
	return r.Status == RegistrationStatusApproved
}

// IsRejected checks if the request has been rejected
func (r *UserRegistrationRequest) IsRejected() bool {
	return r.Status == RegistrationStatusRejected
}
