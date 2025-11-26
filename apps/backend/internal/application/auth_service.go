package application

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/opena2a/identity/backend/internal/infrastructure/auth"
)

// AuthService handles authentication business logic
type AuthService struct {
	userRepo      domain.UserRepository
	orgRepo       domain.OrganizationRepository
	apiKeyRepo    domain.APIKeyRepository
	policyService *SecurityPolicyService
	emailService  domain.EmailService
}

// NewAuthService creates a new auth service
func NewAuthService(
	userRepo domain.UserRepository,
	orgRepo domain.OrganizationRepository,
	apiKeyRepo domain.APIKeyRepository,
	policyService *SecurityPolicyService,
	emailService domain.EmailService,
) *AuthService {
	return &AuthService{
		userRepo:      userRepo,
		orgRepo:       orgRepo,
		apiKeyRepo:    apiKeyRepo,
		policyService: policyService,
		emailService:  emailService,
	}
}

// LoginResponse contains login result (used internally)
type LoginResponse struct {
	User         *domain.User
	AccessToken  string
	RefreshToken string
}

// OAuth functions removed - OAuth infrastructure has been completely removed

// LoginWithPassword authenticates a user with email and password
func (s *AuthService) LoginWithPassword(ctx context.Context, email, password string) (*domain.User, error) {
	// Get user by email
	user, err := s.userRepo.GetByEmail(email)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check if user account is deactivated
	if user.Status == domain.UserStatusDeactivated || user.DeletedAt != nil {
		return nil, fmt.Errorf("your account has been deactivated. Please contact your administrator for assistance")
	}

	// Check if user has a password (local authentication enabled)
	if user.PasswordHash == nil || *user.PasswordHash == "" {
		return nil, fmt.Errorf("local authentication not configured for this user")
	}

	// Verify password
	passwordHasher := auth.NewPasswordHasher()
	if err := passwordHasher.VerifyPassword(password, *user.PasswordHash); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Email verification removed - handled during registration approval

	// Update last login timestamp
	now := time.Now()
	user.LastLoginAt = &now
	user.UpdatedAt = now
	if err := s.userRepo.Update(user); err != nil {
		// Log error but don't fail the login - this is non-critical
		fmt.Printf("Warning: failed to update last_login_at for user %s: %v\n", user.ID, err)
	}

	return user, nil
}

// GetUserByID retrieves a user by ID
func (s *AuthService) GetUserByID(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return s.userRepo.GetByID(userID)
}

// GetUserByEmail retrieves a user by email
func (s *AuthService) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	return s.userRepo.GetByEmail(email)
}

// GetUsersByOrganization retrieves all users in an organization
func (s *AuthService) GetUsersByOrganization(ctx context.Context, orgID uuid.UUID) ([]*domain.User, error) {
	return s.userRepo.GetByOrganization(orgID)
}

// CountActiveUsers returns the count of users who logged in recently
// withinMinutes: time window for "active" definition (e.g., 60 for last hour)
func (s *AuthService) CountActiveUsers(ctx context.Context, orgID uuid.UUID, withinMinutes int) (int, error) {
	return s.userRepo.CountActiveUsers(orgID, withinMinutes)
}

// UpdateUserRole updates a user's role
func (s *AuthService) UpdateUserRole(
	ctx context.Context,
	userID uuid.UUID,
	orgID uuid.UUID,
	role domain.UserRole,
	adminID uuid.UUID,
) (*domain.User, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, err
	}

	// Verify user belongs to organization
	if user.OrganizationID != orgID {
		return nil, fmt.Errorf("user not found in organization")
	}

	// Update role
	user.Role = role
	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}

	return user, nil
}

// DeactivateUser deactivates a user account (soft delete)
func (s *AuthService) DeactivateUser(
	ctx context.Context,
	userID uuid.UUID,
	orgID uuid.UUID,
	adminID uuid.UUID,
) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}

	// Verify user belongs to organization
	if user.OrganizationID != orgID {
		return fmt.Errorf("user not found in organization")
	}

	// Prevent self-deactivation
	if userID == adminID {
		return fmt.Errorf("cannot deactivate your own account")
	}

	// Update status to deactivated (soft delete) and set deleted_at timestamp
	now := time.Now()
	user.Status = domain.UserStatusDeactivated
	user.DeletedAt = &now
	user.UpdatedAt = now
	return s.userRepo.Update(user)
}

// ChangePassword changes a user's password
func (s *AuthService) ChangePassword(
	ctx context.Context,
	userID uuid.UUID,
	currentPassword string,
	newPassword string,
) error {
	// Get user
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	if user.PasswordHash == nil || *user.PasswordHash == "" {
		return fmt.Errorf("password not configured for this account, please contact administrator")
	}

	passwordHasher := auth.NewPasswordHasher()
	if err := passwordHasher.VerifyPassword(currentPassword, *user.PasswordHash); err != nil {
		return fmt.Errorf("current password is incorrect")
	}

	if err := passwordHasher.ValidatePassword(newPassword); err != nil {
		return err
	}

	// Hash new password
	newHash, err := passwordHasher.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	// Update password in database
	user.PasswordHash = &newHash
	user.ForcePasswordChange = false
	user.UpdatedAt = time.Now()

	if err := s.userRepo.Update(user); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// ValidateAPIKeyResponse contains API key validation result
type ValidateAPIKeyResponse struct {
	User         *domain.User
	Organization *domain.Organization
	APIKey       *domain.APIKey
}

// ValidateAPIKey validates an API key and returns the associated user and organization
func (s *AuthService) ValidateAPIKey(ctx context.Context, apiKey string) (*ValidateAPIKeyResponse, error) {
	// Hash the API key using SHA-256 (must match api_key_service.go encoding)
	hash := sha256.Sum256([]byte(apiKey))
	hashedKey := base64.StdEncoding.EncodeToString(hash[:])

	// Retrieve API key from database
	key, err := s.apiKeyRepo.GetByHash(hashedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve API key: %w", err)
	}

	if key == nil {
		return nil, fmt.Errorf("invalid API key")
	}

	// Validate API key is active
	if !key.IsActive {
		return nil, fmt.Errorf("API key is inactive")
	}

	// Validate API key has not expired
	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("API key has expired")
	}

	// Retrieve the user who owns the API key
	user, err := s.userRepo.GetByID(key.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve user: %w", err)
	}

	if user == nil {
		return nil, fmt.Errorf("user not found for API key")
	}

	// Retrieve the organization
	org, err := s.orgRepo.GetByID(key.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve organization: %w", err)
	}

	if org == nil {
		return nil, fmt.Errorf("organization not found for API key")
	}

	// Update last_used_at timestamp
	if err := s.apiKeyRepo.UpdateLastUsed(key.ID); err != nil {
		// Log error but don't fail the request - this is non-critical
		// Note: In production, this should use proper structured logging
	}

	return &ValidateAPIKeyResponse{
		User:         user,
		Organization: org,
		APIKey:       key,
	}, nil
}

// UpdateLastLogin updates a user's last_login_at timestamp
func (s *AuthService) UpdateLastLogin(ctx context.Context, user *domain.User) error {
	now := time.Now()
	user.LastLoginAt = &now
	user.UpdatedAt = now
	if err := s.userRepo.Update(user); err != nil {
		// Log error but don't fail - this is non-critical
		fmt.Printf("Warning: failed to update last_login_at for user %s: %v\n", user.ID, err)
		return err
	}
	return nil
}
