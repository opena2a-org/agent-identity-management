package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/opena2a/identity/backend/internal/infrastructure/auth"
)

// AuthService handles authentication business logic
type AuthService struct {
	userRepo domain.UserRepository
	orgRepo  domain.OrganizationRepository
}

// NewAuthService creates a new auth service
func NewAuthService(
	userRepo domain.UserRepository,
	orgRepo domain.OrganizationRepository,
) *AuthService {
	return &AuthService{
		userRepo: userRepo,
		orgRepo:  orgRepo,
	}
}

// LoginResponse contains login result
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
		// User exists, update profile if needed
		avatarChanged := (user.AvatarURL == nil && oauthUser.AvatarURL != "") ||
			(user.AvatarURL != nil && *user.AvatarURL != oauthUser.AvatarURL)

		if user.Name != oauthUser.Name || avatarChanged {
			user.Name = oauthUser.Name
			user.AvatarURL = &oauthUser.AvatarURL
			if err := s.userRepo.Update(user); err != nil {
				return nil, err
			}
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
	if len(existingUsers) == 0 {
		role = domain.RoleAdmin
	}

	// Create new user
	avatarURL := oauthUser.AvatarURL
	user := &domain.User{
		OrganizationID: org.ID,
		Email:          oauthUser.Email,
		Name:           oauthUser.Name,
		AvatarURL:      &avatarURL,
		Role:           role,
		Provider:       oauthUser.Provider,
		ProviderID:     oauthUser.ID,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
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

	return user, nil
}

// GetUserByID retrieves a user by ID
func (s *AuthService) GetUserByID(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return s.userRepo.GetByID(userID)
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

// DeactivateUser deactivates a user account
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

	// Delete user
	return s.userRepo.Delete(userID)
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

	// Check if user has a password (local authentication)
	if user.PasswordHash == nil || *user.PasswordHash == "" {
		return fmt.Errorf("password change not available for OAuth users")
	}

	// Verify current password
	passwordHasher := auth.NewPasswordHasher()
	if err := passwordHasher.VerifyPassword(currentPassword, *user.PasswordHash); err != nil {
		return fmt.Errorf("current password is incorrect")
	}

	// Validate new password
	if err := passwordHasher.ValidatePassword(newPassword); err != nil {
		return err
	}

	// Hash new password
	newHash, err := passwordHasher.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	// Update password and clear force_password_change flag
	user.PasswordHash = &newHash
	user.ForcePasswordChange = false

	if err := s.userRepo.Update(user); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}
