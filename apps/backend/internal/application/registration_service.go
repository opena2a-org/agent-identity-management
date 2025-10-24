package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/opena2a/identity/backend/internal/infrastructure/auth"
)

var (
	ErrRegistrationNotFound      = errors.New("registration request not found")
	ErrRegistrationNotPending    = errors.New("registration request is not pending")
	ErrUserAlreadyExists         = errors.New("user with this email already exists")
	ErrRegistrationRequestExists = errors.New("registration request with this email already exists")
)

// RegistrationRepository defines the interface for registration data persistence
type RegistrationRepository interface {
	// Registration requests
	CreateRegistrationRequest(ctx context.Context, req *domain.UserRegistrationRequest) error
	GetRegistrationRequest(ctx context.Context, id uuid.UUID) (*domain.UserRegistrationRequest, error)
	GetRegistrationRequestByEmail(ctx context.Context, email string) (*domain.UserRegistrationRequest, error)
	GetRegistrationRequestByEmailAnyStatus(ctx context.Context, email string) (*domain.UserRegistrationRequest, error)
	ListPendingRegistrationRequests(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.UserRegistrationRequest, int, error)
	UpdateRegistrationRequest(ctx context.Context, req *domain.UserRegistrationRequest) error
}

// RegistrationService handles user registration and approval workflows
type RegistrationService struct {
	registrationRepo RegistrationRepository
	userRepo         domain.UserRepository
	auditService     *AuditService
	emailService     domain.EmailService
}

func NewRegistrationService(
	registrationRepo RegistrationRepository,
	userRepo domain.UserRepository,
	auditService *AuditService,
	emailService domain.EmailService,
) *RegistrationService {
	return &RegistrationService{
		registrationRepo: registrationRepo,
		userRepo:         userRepo,
		auditService:     auditService,
		emailService:     emailService,
	}
}

// CreateManualRegistrationRequest creates a registration request for email/password user registration
func (s *RegistrationService) CreateManualRegistrationRequest(
	ctx context.Context,
	email, firstName, lastName, password string,
) (*domain.UserRegistrationRequest, error) {
	// Check if user already exists
	existingUser, err := s.userRepo.GetByEmail(email)
	if err == nil && existingUser != nil {
		return nil, ErrUserAlreadyExists
	}

	// Check if a registration request already exists for this email
	existingRequest, err := s.registrationRepo.GetRegistrationRequestByEmail(ctx, email)
	if err == nil && existingRequest != nil && existingRequest.IsPending() {
		return nil, ErrRegistrationRequestExists
	}

	// Hash and validate password
	passwordHasher := auth.NewPasswordHasher()
	if err := passwordHasher.ValidatePassword(password); err != nil {
		return nil, fmt.Errorf("password validation failed: %w", err)
	}

	hashedPassword, err := passwordHasher.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create new manual registration request
	req := domain.NewUserRegistrationRequestManual(
		email,
		firstName,
		lastName,
		hashedPassword,
	)

	// Save registration request
	if err := s.registrationRepo.CreateRegistrationRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("failed to create registration request: %w", err)
	}

	// Send registration confirmation email
	if s.emailService != nil {
		frontendURL := os.Getenv("FRONTEND_URL")
		if frontendURL == "" {
			frontendURL = "http://localhost:3000"
		}

		supportEmail := os.Getenv("SUPPORT_EMAIL")
		if supportEmail == "" {
			supportEmail = "info@opena2a.org"
		}

		// Combine first and last name
		fullName := firstName
		if lastName != "" {
			if fullName != "" {
				fullName += " "
			}
			fullName += lastName
		}
		if fullName == "" {
			fullName = email // Fallback to email if no name
		}

		templateData := domain.EmailTemplateData{
			UserName:     fullName,
			UserEmail:    email,
			DashboardURL: frontendURL,
			SupportEmail: supportEmail,
			Timestamp:    time.Now(),
			CustomData: map[string]interface{}{
				"FirstName": firstName,
				"LastName":  lastName,
			},
		}

		if err := s.emailService.SendTemplatedEmail(domain.TemplateWelcome, email, templateData); err != nil {
			// Log error but don't fail the request (email is non-critical)
			fmt.Printf("⚠️  Failed to send registration confirmation email to %s: %v\n", email, err)
		} else {
			fmt.Printf("✅ Sent registration confirmation email to %s\n", email)
		}
	}

	return req, nil
}

// CreateAccessRequest creates an access request without password (for request-access endpoint)
// This differs from CreateManualRegistrationRequest by not requiring a password
func (s *RegistrationService) CreateAccessRequest(
	ctx context.Context,
	email, firstName, lastName, reason string,
	organizationName *string,
) (*domain.UserRegistrationRequest, error) {
	// Check if user already exists
	existingUser, err := s.userRepo.GetByEmail(email)
	if err == nil && existingUser != nil {
		return nil, ErrUserAlreadyExists
	}

	// Check if a registration request already exists for this email
	existingRequest, err := s.registrationRepo.GetRegistrationRequestByEmail(ctx, email)
	if err == nil && existingRequest != nil && existingRequest.IsPending() {
		return nil, ErrRegistrationRequestExists
	}

	// Create new access request (no password)
	now := time.Now()
	localProvider := domain.OAuthProviderLocal

	req := &domain.UserRegistrationRequest{
		ID:                 uuid.New(),
		Email:              email,
		FirstName:          firstName,
		LastName:           lastName,
		PasswordHash:       nil, // No password for access requests
		OAuthProvider:      &localProvider,
		OAuthUserID:        nil,
		Status:             domain.RegistrationStatusPending,
		RequestedAt:        now,
		OAuthEmailVerified: false,
		Metadata:           map[string]interface{}{
			"reason": reason,
		},
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	// Add organization name to metadata if provided
	if organizationName != nil && *organizationName != "" {
		req.Metadata["organization_name"] = *organizationName
	}

	// Save access request
	if err := s.registrationRepo.CreateRegistrationRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("failed to create access request: %w", err)
	}

	return req, nil
}

// GetRegistrationRequest retrieves a registration request by ID
func (s *RegistrationService) GetRegistrationRequest(ctx context.Context, requestID uuid.UUID) (*domain.UserRegistrationRequest, error) {
	return s.registrationRepo.GetRegistrationRequest(ctx, requestID)
}

// GetRegistrationRequestByEmail retrieves a registration request by email
func (s *RegistrationService) GetRegistrationRequestByEmail(ctx context.Context, email string) (*domain.UserRegistrationRequest, error) {
	// Use the any status method to find registration requests regardless of status
	return s.registrationRepo.GetRegistrationRequestByEmailAnyStatus(ctx, email)
}

// ListPendingRegistrationRequests returns all pending registration requests for an organization
func (s *RegistrationService) ListPendingRegistrationRequests(
	ctx context.Context,
	orgID uuid.UUID,
	limit, offset int,
) ([]*domain.UserRegistrationRequest, int, error) {
	return s.registrationRepo.ListPendingRegistrationRequests(ctx, orgID, limit, offset)
}

// ApproveRegistrationRequest approves a registration request and creates the user account
func (s *RegistrationService) ApproveRegistrationRequest(
	ctx context.Context,
	requestID uuid.UUID,
	reviewerID uuid.UUID,
	orgID uuid.UUID,
) (*domain.User, error) {
	// Get registration request
	req, err := s.registrationRepo.GetRegistrationRequest(ctx, requestID)
	if err != nil {
		return nil, ErrRegistrationNotFound
	}

	if !req.IsPending() {
		return nil, ErrRegistrationNotPending
	}

	// Approve request
	req.Approve(reviewerID)
	if err := s.registrationRepo.UpdateRegistrationRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("failed to update registration request: %w", err)
	}

	// Create user account
	// Combine first and last name for the Name field
	fullName := req.FirstName
	if req.LastName != "" {
		if fullName != "" {
			fullName += " "
		}
		fullName += req.LastName
	}
	if fullName == "" {
		fullName = req.Email // Fallback to email if no name provided
	}

	// Determine provider based on registration request type
	provider := "local" // Default to local for email/password
	providerID := req.Email // Use email as provider ID for local auth

	// If OAuth registration, use OAuth provider info
	if req.OAuthProvider != nil && *req.OAuthProvider != "" {
		provider = string(*req.OAuthProvider) // Convert OAuthProvider enum to string
		if req.OAuthUserID != nil {
			providerID = *req.OAuthUserID
		}
	}

	user := &domain.User{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Email:          req.Email,
		Name:           fullName,
		Role:           domain.RoleViewer, // Default to viewer role for new users
		Provider:       provider,
		ProviderID:     providerID,
		PasswordHash:   req.PasswordHash,  // Will be set for email/password registrations
		ApprovedBy:     &reviewerID,
		ApprovedAt:     &time.Time{},
		Status:         domain.UserStatusActive, // Set user as active upon approval
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if req.PasswordHash != nil && *req.PasswordHash != "" {
		fmt.Printf("✅ Approving user with password hash for email: %s\n", req.Email)
	} else {
		fmt.Printf("⚠️  WARNING: Approving user without password hash - this should not happen for email/password registrations\n")
	}

	// Set approval timestamp
	now := time.Now()
	user.ApprovedAt = &now

	// Create user via repository
	if err := s.userRepo.Create(user); err != nil {
		// Log detailed error for debugging
		fmt.Printf("❌ CRITICAL ERROR: Failed to create user account after approval\n")
		fmt.Printf("   Email: %s\n", user.Email)
		fmt.Printf("   Organization ID: %s\n", user.OrganizationID)
		fmt.Printf("   Provider: %s\n", user.Provider)
		fmt.Printf("   ProviderID: %s\n", user.ProviderID)
		fmt.Printf("   Role: %s\n", user.Role)
		fmt.Printf("   Status: %s\n", user.Status)
		fmt.Printf("   Password Hash Present: %v\n", user.PasswordHash != nil && *user.PasswordHash != "")
		fmt.Printf("   Error: %v\n", err)

		return nil, fmt.Errorf("failed to create user '%s' in database: %w", user.Email, err)
	}

	// Success logging
	fmt.Printf("✅ Successfully created user account: %s (ID: %s)\n", user.Email, user.ID)

	// Log audit
	s.auditService.LogAction(
		ctx,
		orgID,
		reviewerID,
		domain.AuditActionCreate,
		"user",
		user.ID,
		"", // IP address
		"", // User agent
		map[string]interface{}{
			"registration_id":     req.ID,
			"registration_method": "email_password_registration",
		},
	)

	// Send approval email to user
	if s.emailService != nil {
		frontendURL := os.Getenv("FRONTEND_URL")
		if frontendURL == "" {
			frontendURL = "http://localhost:3000"
		}

		supportEmail := os.Getenv("SUPPORT_EMAIL")
		if supportEmail == "" {
			supportEmail = "info@opena2a.org"
		}

		loginURL := fmt.Sprintf("%s/auth/login", frontendURL)

		templateData := domain.EmailTemplateData{
			UserName:     fullName,
			UserEmail:    user.Email,
			DashboardURL: frontendURL,
			SupportEmail: supportEmail,
			Timestamp:    now,
			CustomData: map[string]interface{}{
				"LoginURL": loginURL,
				"Role":     string(user.Role),
			},
		}

		if err := s.emailService.SendTemplatedEmail(domain.TemplateUserApproved, user.Email, templateData); err != nil {
			// Log error but don't fail the request (email is non-critical)
			fmt.Printf("⚠️  Failed to send approval email to %s: %v\n", user.Email, err)
		} else {
			fmt.Printf("✅ Sent approval email to %s\n", user.Email)
		}
	}

	return user, nil
}

// RejectRegistrationRequest rejects a registration request
func (s *RegistrationService) RejectRegistrationRequest(
	ctx context.Context,
	requestID uuid.UUID,
	reviewerID uuid.UUID,
	reason string,
) error {
	// Get registration request
	req, err := s.registrationRepo.GetRegistrationRequest(ctx, requestID)
	if err != nil {
		return ErrRegistrationNotFound
	}

	if !req.IsPending() {
		return ErrRegistrationNotPending
	}

	// Reject request
	req.Reject(reviewerID, reason)
	if err := s.registrationRepo.UpdateRegistrationRequest(ctx, req); err != nil {
		return fmt.Errorf("failed to update registration request: %w", err)
	}

	// TODO: Send rejection email to user

	return nil
}

// RequestPasswordReset generates a password reset token for a user and sends a reset email
func (s *RegistrationService) RequestPasswordReset(
	ctx context.Context,
	email string,
) error {
	// Normalize email
	email = strings.ToLower(strings.TrimSpace(email))

	// Get user by email (fail silently for security)
	user, err := s.userRepo.GetByEmail(email)
	if err != nil || user == nil {
		// Don't reveal if user exists - always return success
		return nil
	}

	// Check if user account is deactivated
	if user.Status == domain.UserStatusDeactivated || user.DeletedAt != nil {
		// Don't reveal if user is deactivated - always return success
		return nil
	}

	// Generate password reset token (UUID format)
	resetToken := uuid.New().String()

	// Set expiration to 24 hours from now
	expiresAt := time.Now().Add(24 * time.Hour)

	// Update user with reset token and expiration
	user.PasswordResetToken = &resetToken
	user.PasswordResetExpiresAt = &expiresAt

	if err := s.userRepo.Update(user); err != nil {
		return fmt.Errorf("failed to update user with reset token: %w", err)
	}

	// Send password reset email using template
	if s.emailService != nil {
		frontendURL := os.Getenv("FRONTEND_URL")
		if frontendURL == "" {
			frontendURL = "http://localhost:3000"
		}

		supportEmail := os.Getenv("SUPPORT_EMAIL")
		if supportEmail == "" {
			supportEmail = "info@opena2a.org"
		}

		resetLink := fmt.Sprintf("%s/auth/reset-password?token=%s", frontendURL, resetToken)

		templateData := domain.EmailTemplateData{
			UserName:     user.Name,
			UserEmail:    user.Email,
			DashboardURL: frontendURL,
			SupportEmail: supportEmail,
			Timestamp:    time.Now(),
			ExpiresAt:    expiresAt,
			CustomData: map[string]interface{}{
				"ResetLink": resetLink,
				"ExpiresIn": "24 hours",
			},
		}

		if err := s.emailService.SendTemplatedEmail(domain.TemplatePasswordReset, user.Email, templateData); err != nil {
			// Log error but don't fail the request (email is non-critical)
			fmt.Printf("⚠️ Failed to send password reset email to %s: %v\n", email, err)
		}
	}

	return nil
}

// ResetPassword resets a user's password using a valid reset token
func (s *RegistrationService) ResetPassword(
	ctx context.Context,
	resetToken string,
	newPassword string,
	confirmPassword string,
) error {
	// Validate inputs
	if strings.TrimSpace(resetToken) == "" {
		return fmt.Errorf("reset token is required")
	}
	if strings.TrimSpace(newPassword) == "" {
		return fmt.Errorf("new password is required")
	}
	if newPassword != confirmPassword {
		return fmt.Errorf("passwords do not match")
	}

	// Find user by reset token (automatically validates expiration)
	user, err := s.userRepo.GetByPasswordResetToken(resetToken)
	if err != nil {
		return fmt.Errorf("invalid or expired reset token")
	}

	// Validate password strength
	passwordHasher := auth.NewPasswordHasher()
	if err := passwordHasher.ValidatePassword(newPassword); err != nil {
		return err
	}

	// Hash new password
	hashedPassword, err := passwordHasher.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update user password and clear reset token
	user.PasswordHash = &hashedPassword
	user.PasswordResetToken = nil
	user.PasswordResetExpiresAt = nil
	user.ForcePasswordChange = false // Clear force password change if set
	user.UpdatedAt = time.Now()

	if err := s.userRepo.Update(user); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Log audit event
	s.auditService.LogAction(
		ctx,
		user.OrganizationID,
		user.ID,
		domain.AuditActionUpdate,
		"user",
		user.ID,
		"", // IP address
		"", // User agent
		map[string]interface{}{
			"action": "password_reset_completed",
		},
	)

	return nil
}
