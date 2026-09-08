package application

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

var (
	ErrRegistrationNotFound      = errors.New("registration request not found")
	ErrRegistrationNotPending    = errors.New("registration request is not pending")
	ErrUserAlreadyExists         = errors.New("user with this email already exists")
	ErrRegistrationRequestExists = errors.New("registration request with this email already exists")
	// ErrNoAdministrators: the request would wait for an approval nobody in this deployment can give.
	ErrNoAdministrators = errors.New("no administrator can approve a registration request in this deployment")
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
	orgRepo          domain.OrganizationRepository
	auditService     *AuditService
	emailService     domain.EmailService
}

func NewRegistrationService(
	registrationRepo RegistrationRepository,
	userRepo domain.UserRepository,
	orgRepo domain.OrganizationRepository,
	auditService *AuditService,
	emailService domain.EmailService,
) *RegistrationService {
	return &RegistrationService{
		registrationRepo: registrationRepo,
		userRepo:         userRepo,
		orgRepo:          orgRepo,
		auditService:     auditService,
		emailService:     emailService,
	}
}

// CreateManualRegistrationRequest creates a registration request for email/password user registration.
// signupProfile holds optional, already-validated questionnaire answers
// (see domain.BuildSignupProfile); pass nil when none were given.
func (s *RegistrationService) CreateManualRegistrationRequest(
	ctx context.Context,
	email, firstName, lastName, password string,
	signupProfile map[string]string,
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

	// SECURITY: Auto-approve ONLY for platform admins listed in AIM_PLATFORM_ADMINS env var.
	// Per-domain auto-approval was removed in PR-A (2026-04-14) because it allowed
	// domain squatting — anyone could claim "first registrant" of a victim's domain
	// and become admin of that org. Domain claims now require explicit DNS verification
	// (see PR-B) and team creation is an explicit user action (see PR-C).
	emailDomain := extractEmailDomain(email)
	shouldAutoApprove := isPlatformAdmin(email)

	if err := s.ensureApproverExists(email, shouldAutoApprove); err != nil {
		return nil, err
	}

	// Create new manual registration request
	req := domain.NewUserRegistrationRequestManual(
		email,
		firstName,
		lastName,
		hashedPassword,
	)

	if len(signupProfile) > 0 {
		req.Metadata = map[string]interface{}{
			domain.SignupProfileMetadataKey: signupProfile,
		}
	}

	if shouldAutoApprove {
		req.Status = domain.RegistrationStatusApproved
		now := time.Now()
		req.ReviewedAt = &now
		req.ReviewedBy = nil // System auto-approval (platform admin allowlist)
		fmt.Printf("✅ Auto-approving platform admin: %s\n", email)
	}

	// Save registration request
	if err := s.registrationRepo.CreateRegistrationRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("failed to create registration request: %w", err)
	}

	// If auto-approved, create the user account immediately
	if shouldAutoApprove {
		// Find or create organization
		targetOrgID, err := s.findOrCreateOrganization(ctx, emailDomain)
		if err != nil {
			return nil, fmt.Errorf("failed to find or create organization: %w", err)
		}

		// Create user account
		fullName := firstName
		if lastName != "" {
			if fullName != "" {
				fullName += " "
			}
			fullName += lastName
		}
		if fullName == "" {
			fullName = email
		}

		now := time.Now()
		user := &domain.User{
			ID:                  uuid.New(),
			OrganizationID:      targetOrgID,
			Email:               email,
			Name:                fullName,
			Role:                domain.RoleAdmin, // First user becomes admin
			Provider:            "local",
			ProviderID:          email,
			PasswordHash:        &hashedPassword,
			ApprovedBy:          nil, // System auto-approval (no reviewer)
			ApprovedAt:          &now,
			Status:              domain.UserStatusActive,
			ForcePasswordChange: true, // SECURITY: Force password change to confirm account ownership
			CreatedAt:           now,
			UpdatedAt:           now,
		}

		if err := s.userRepo.Create(user); err != nil {
			return nil, fmt.Errorf("failed to create auto-approved user: %w", err)
		}

		fmt.Printf("✅ Auto-created admin user %s for new organization %s\n", email, emailDomain)
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

	// An access request is never auto-approved, so it needs an approver like any other.
	if err := s.ensureApproverExists(email, false); err != nil {
		return nil, err
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
		Metadata: map[string]interface{}{
			"reason": reason,
		},
		CreatedAt: now,
		UpdatedAt: now,
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

	// Use the approving admin's organization so the user joins the same org
	targetOrgID := orgID

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
	provider := "local"     // Default to local for email/password
	providerID := req.Email // Use email as provider ID for local auth

	// If OAuth registration, use OAuth provider info
	if req.OAuthProvider != nil && *req.OAuthProvider != "" {
		provider = string(*req.OAuthProvider) // Convert OAuthProvider enum to string
		if req.OAuthUserID != nil {
			providerID = *req.OAuthUserID
		}
	}

	// Check if this is the first user in the organization (make them admin)
	existingUsers, err := s.userRepo.GetByOrganization(targetOrgID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing users: %w", err)
	}

	userRole := domain.RoleViewer // Default to viewer
	if len(existingUsers) == 0 {
		userRole = domain.RoleAdmin // First user becomes admin
		fmt.Printf("Making user %s admin (first user in organization %s)\n", req.Email, targetOrgID)
	}

	user := &domain.User{
		ID:             uuid.New(),
		OrganizationID: targetOrgID, // Use the approving admin's organization
		Email:          req.Email,
		Name:           fullName,
		Role:           userRole, // Admin if first user, otherwise viewer
		Provider:       provider,
		ProviderID:     providerID,
		PasswordHash:   req.PasswordHash, // Will be set for email/password registrations
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

// extractEmailDomain extracts the domain from an email address
func extractEmailDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return email // Return original if invalid format
	}
	return parts[1]
}

// findOrCreateOrganization finds an existing organization by domain or creates a new one
func (s *RegistrationService) findOrCreateOrganization(ctx context.Context, domainName string) (uuid.UUID, error) {
	// Try to find existing organization by domain
	org, err := s.orgRepo.GetByDomain(domainName)
	if err == nil && org != nil {
		// Organization exists
		fmt.Printf("✅ Found existing organization for domain: %s\n", domainName)
		return org.ID, nil
	}

	// Organization doesn't exist, create new one
	fmt.Printf("📝 Creating new organization for domain: %s\n", domainName)

	newOrg := &domain.Organization{
		ID:        uuid.New(),
		Name:      domainName, // Use domain as organization name
		Domain:    domainName,
		PlanType:  "free",
		MaxAgents: 100,
		MaxUsers:  10,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.orgRepo.Create(newOrg); err != nil {
		return uuid.Nil, fmt.Errorf("failed to create organization: %w", err)
	}

	fmt.Printf("✅ Created new organization: %s (ID: %s)\n", domainName, newOrg.ID)
	return newOrg.ID, nil
}

// isPlatformAdmin returns true if the email is in the AIM_PLATFORM_ADMINS allowlist.
// Platform admins are AIM Cloud operators (e.g., the OpenA2A team), distinct from
// customer org admins. The allowlist is a comma-separated env var, e.g.:
//
//	AIM_PLATFORM_ADMINS=info@opena2a.org,abdel@opena2a.org
//
// Email comparison is case-insensitive and trims whitespace. Empty env var means
// no platform admins exist (safe default — only approved-by-existing-admin users
// can register).
// ensureApproverExists refuses a request that would wait for an approval nobody in this
// deployment can give. On a fresh self-hosted install with no AIM_PLATFORM_ADMINS allowlist
// and no active administrator, a pending request would wait forever and nothing would say
// so; the refusal happens before anything is written (never write-then-refuse). A request
// that auto-approves needs no approver.
func (s *RegistrationService) ensureApproverExists(email string, autoApproved bool) error {
	if autoApproved {
		return nil
	}
	// The allowlist is intent; the users table is state. A listed address that never
	// registered (or a mistyped entry) is not an approver, so the count runs regardless.
	approvers, err := s.userRepo.CountByRoleAndStatus(domain.RoleAdmin, domain.UserStatusActive)
	if err != nil {
		return fmt.Errorf("failed to count administrators: %w", err)
	}
	if approvers == 0 {
		// The client receives one message for every sub-case; the sub-case is operator-only.
		log.Printf("registration refused for %s: 0 active administrators, %d valid AIM_PLATFORM_ADMINS entries, registrant listed=%t", email, len(platformAdminAllowlist()), isPlatformAdmin(email))
		return ErrNoAdministrators
	}
	return nil
}

// platformAdminAllowlist returns the lower-cased, trimmed entries of AIM_PLATFORM_ADMINS
// that could be an email address (something@something). An unset, empty or separator-only
// variable yields no entries; a token that cannot be an address is ignored (reported once at
// startup by ReportPlatformAdminAllowlist). The filter is deliberately no stricter than what
// registration itself accepts.
func platformAdminAllowlist() []string {
	entries, _ := parsePlatformAdminAllowlist(os.Getenv("AIM_PLATFORM_ADMINS"))
	return entries
}

// parsePlatformAdminAllowlist splits the variable into accepted entries and ignored tokens.
func parsePlatformAdminAllowlist(raw string) (entries []string, ignored []string) {
	entries = make([]string, 0)
	ignored = make([]string, 0)
	for _, entry := range strings.Split(raw, ",") {
		e := strings.ToLower(strings.TrimSpace(entry))
		if e == "" {
			continue
		}
		at := strings.Index(e, "@")
		if at <= 0 || at == len(e)-1 {
			ignored = append(ignored, e)
			continue
		}
		entries = append(entries, e)
	}
	return entries, ignored
}

// ReportPlatformAdminAllowlist logs, once at startup, how AIM_PLATFORM_ADMINS was read: the
// number of accepted addresses and every ignored token by position, so a mistyped variable is
// visible to the operator without refusing to boot.
func ReportPlatformAdminAllowlist() {
	raw := os.Getenv("AIM_PLATFORM_ADMINS")
	if strings.TrimSpace(raw) == "" {
		return
	}
	entries, _ := parsePlatformAdminAllowlist(raw)
	for i, tok := range strings.Split(raw, ",") {
		t := strings.ToLower(strings.TrimSpace(tok))
		if t == "" {
			continue
		}
		// One line per position, so a token that appears twice is reported where it appears.
		if accepted, _ := parsePlatformAdminAllowlist(t); len(accepted) == 0 {
			log.Printf("AIM_PLATFORM_ADMINS entry %d (%q) is not an email address and is ignored", i+1, t)
		}
	}
	if len(entries) == 0 {
		log.Printf("AIM_PLATFORM_ADMINS is set but contains no email address; no account will be approved automatically")
		return
	}
	log.Printf("AIM_PLATFORM_ADMINS: %d address(es) accepted", len(entries))
}

func isPlatformAdmin(email string) bool {
	target := strings.ToLower(strings.TrimSpace(email))
	for _, entry := range platformAdminAllowlist() {
		if entry == target {
			return true
		}
	}
	return false
}
