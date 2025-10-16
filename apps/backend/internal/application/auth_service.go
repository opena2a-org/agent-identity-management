package application

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
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
}

// NewAuthService creates a new auth service
func NewAuthService(
	userRepo domain.UserRepository,
	orgRepo domain.OrganizationRepository,
	apiKeyRepo domain.APIKeyRepository,
	policyService *SecurityPolicyService,
) *AuthService {
	return &AuthService{
		userRepo:      userRepo,
		orgRepo:       orgRepo,
		apiKeyRepo:    apiKeyRepo,
		policyService: policyService,
	}
}

// LoginResponse contains login result (used internally)
type LoginResponse struct {
	User         *domain.User
	AccessToken  string
	RefreshToken string
}

// findOrCreateUser finds existing user or creates new one with auto-provisioning
func (s *AuthService) findOrCreateUser(ctx context.Context, oauthUser *auth.OAuthUser) (*domain.User, error) {
	// Try to find existing user by provider ID
	user, err := s.userRepo.GetByProvider(oauthUser.Provider, oauthUser.ID)
	if err != nil {
		return nil, err
	}

	if user != nil {
		// Check if user account is deactivated
		if user.Status == domain.UserStatusDeactivated || user.DeletedAt != nil {
			return nil, fmt.Errorf("your account has been deactivated. Please contact your administrator for assistance")
		}

		// User exists, update profile and last_login_at
		now := time.Now()
		avatarChanged := (user.AvatarURL == nil && oauthUser.AvatarURL != "") ||
			(user.AvatarURL != nil && *user.AvatarURL != oauthUser.AvatarURL)

		// Always update last_login_at on successful login
		needsUpdate := user.Name != oauthUser.Name || avatarChanged
		user.LastLoginAt = &now
		user.UpdatedAt = now

		if needsUpdate {
			user.Name = oauthUser.Name
			user.AvatarURL = &oauthUser.AvatarURL
		}

		// Update user with new login timestamp (and profile if changed)
		if err := s.userRepo.Update(user); err != nil {
			// Log error but don't fail the login - this is non-critical
			fmt.Printf("Warning: failed to update user on login for %s: %v\n", user.ID, err)
		}

		return user, nil
	}

	// User doesn't exist, auto-provision
	return s.autoProvisionUser(ctx, oauthUser)
}

// autoProvisionUser creates a new user and organization if needed
func (s *AuthService) autoProvisionUser(ctx context.Context, oauthUser *auth.OAuthUser) (*domain.User, error) {
	// Extract domain from email
	emailDomain := extractDomain(oauthUser.Email)
	if emailDomain == "" {
		return nil, fmt.Errorf("invalid email format")
	}

	// Find or create organization by domain
	org, err := s.orgRepo.GetByDomain(emailDomain)
	if err != nil {
		return nil, err
	}

	if org == nil {
		// Create new organization
		org = &domain.Organization{
			Name:      emailDomain,
			Domain:    emailDomain,
			PlanType:  "free",
			MaxAgents: 100,
			MaxUsers:  10,
			IsActive:  true,
		}

		if err := s.orgRepo.Create(org); err != nil {
			return nil, fmt.Errorf("failed to create organization: %w", err)
		}
	}

	// Check if this is the first user (make them admin)
	existingUsers, err := s.userRepo.GetByOrganization(org.ID)
	if err != nil {
		return nil, err
	}

	role := domain.RoleMember
	userStatus := domain.UserStatusActive
	isFirstUser := len(existingUsers) == 0

	if isFirstUser {
		// First user is always admin and active
		role = domain.RoleAdmin
		userStatus = domain.UserStatusActive
	} else {
		// Subsequent users: check organization's auto_approve_sso setting
		if !org.AutoApproveSSO {
			userStatus = domain.UserStatusPending
		}
	}

	// Create new user
	avatarURL := oauthUser.AvatarURL
	user := &domain.User{
		OrganizationID: org.ID,
		Email:          oauthUser.Email,
		Name:           oauthUser.Name,
		AvatarURL:      &avatarURL,
		Role:           role,
		Status:         userStatus,
		Provider:       oauthUser.Provider,
		ProviderID:     oauthUser.ID,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// 🛡️ Create default security policies for new organizations
	if isFirstUser {
		fmt.Printf("✅ Creating default security policies for new organization %s\n", org.ID)
		if err := s.policyService.CreateDefaultPolicies(ctx, org.ID, user.ID); err != nil {
			// Log error but don't fail user creation - policies can be created manually later
			fmt.Printf("⚠️  Warning: failed to create default security policies: %v\n", err)
		} else {
			fmt.Printf("✅ Successfully created default security policies for organization %s\n", org.ID)
		}
	}

	return user, nil
}

// extractDomain extracts domain from email address
func extractDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

// LoginWithOAuth logs in or creates user using OAuth data
func (s *AuthService) LoginWithOAuth(ctx context.Context, oauthUser *auth.OAuthUser) (*domain.User, error) {
	return s.findOrCreateUser(ctx, oauthUser)
}

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

	// Check if email is verified
	if !user.EmailVerified {
		return nil, fmt.Errorf("email not verified")
	}

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


	if user.Provider != "local" {
		return fmt.Errorf("password change not available for OAuth users")
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
