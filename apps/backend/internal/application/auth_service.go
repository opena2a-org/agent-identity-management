package application

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// ChangePassword refusals the caller can act on.
var (
	ErrPasswordNotConfigured    = errors.New("password not configured for this account, please contact administrator")
	ErrCurrentPasswordIncorrect = errors.New("current password is incorrect")
)

// AuthService handles authentication business logic
type AuthService struct {
	userRepo        domain.UserRepository
	orgRepo         domain.OrganizationRepository
	apiKeyRepo      domain.APIKeyRepository
	authFailureRepo domain.AuthFailureRepository
	policyService   *SecurityPolicyService
	emailService    domain.EmailService
	failureConfig   domain.AuthFailureConfig
	sdkTokenRepo    domain.SDKTokenRepository
	providerSignIn  ProviderSignInRequirement
	passwordHasher  *auth.PasswordHasher
}

// ProviderSignInRequirement reports whether an organization requires its
// users who are not administrators to sign in through its identity provider.
// *OIDCSignInService implements it.
type ProviderSignInRequirement interface {
	RequiresProviderSignIn(ctx context.Context, organizationID uuid.UUID) (bool, error)
}

// SetProviderSignInRequirement attaches the requirement CheckPasswordSignIn
// reads. With none attached, no organization requires provider sign-in.
func (s *AuthService) SetProviderSignInRequirement(r ProviderSignInRequirement) {
	s.providerSignIn = r
}

// SetSDKTokenRepository attaches the sdk_tokens rows a password change
// revokes: ChangePassword marks each row of the user not yet revoked as
// revoked, with the reason domain.SDKTokenRevokeReasonPasswordChanged. With
// none attached, a change revokes no row.
func (s *AuthService) SetSDKTokenRepository(repo domain.SDKTokenRepository) {
	s.sdkTokenRepo = repo
}

// SetPasswordHasher attaches the hasher ChangePassword hashes the new
// password with. With none attached, it hashes at auth.BcryptCost.
func (s *AuthService) SetPasswordHasher(h *auth.PasswordHasher) {
	s.passwordHasher = h
}

// NewAuthService creates a new auth service
func NewAuthService(
	userRepo domain.UserRepository,
	orgRepo domain.OrganizationRepository,
	apiKeyRepo domain.APIKeyRepository,
	authFailureRepo domain.AuthFailureRepository,
	policyService *SecurityPolicyService,
	emailService domain.EmailService,
) *AuthService {
	return &AuthService{
		userRepo:        userRepo,
		orgRepo:         orgRepo,
		apiKeyRepo:      apiKeyRepo,
		authFailureRepo: authFailureRepo,
		policyService:   policyService,
		emailService:    emailService,
		failureConfig:   domain.DefaultAuthFailureConfig(),
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
	return s.LoginWithPasswordExtended(ctx, email, password, "", "")
}

// The reasons a SignInRefusal names, one per check that refuses a password
// sign-in.
const (
	SignInRefusedAccountLocked      = "account_locked"      // the email has an active lockout
	SignInRefusedUnknownEmail       = "unknown_email"       // no user with the email was read
	SignInRefusedAccountDeactivated = "account_deactivated" // the user is deactivated or deleted
	SignInRefusedPasswordNotSet     = "password_not_set"    // the user has no password
	SignInRefusedPasswordMismatch   = "password_mismatch"   // the password is not the user's
	// the password is the user's, and the user's organization requires its
	// users who are not administrators to sign in through its identity provider
	SignInRefusedProviderSignInRequired = "provider_sign_in_required"
)

// CheckPasswordSignIn returns nil when user may sign in with a password, and
// a *SignInRefusal with the reason SignInRefusedProviderSignInRequired when
// the user's organization requires sign-in through its identity provider and
// the user is not an administrator. An administrator's sign-in reads no
// requirement. When the requirement cannot be read, the returned error is
// not a *SignInRefusal, and the caller refuses the sign-in.
func (s *AuthService) CheckPasswordSignIn(ctx context.Context, user *domain.User) error {
	if s.providerSignIn == nil || user.Role == domain.RoleAdmin {
		return nil
	}
	required, err := s.providerSignIn.RequiresProviderSignIn(ctx, user.OrganizationID)
	if err != nil {
		return fmt.Errorf("reading whether the organization requires provider sign-in: %w", err)
	}
	if required {
		return signInRefusal(SignInRefusedProviderSignInRequired, user,
			"the organization requires sign-in through its identity provider")
	}
	return nil
}

// SignInRefusal is the error LoginWithPasswordExtended returns when it
// refuses a sign-in. Its text is the refusal's message. Reason names the
// check that refused it; UserID and OrganizationID name the user when the
// email matched one, and are uuid.Nil otherwise.
type SignInRefusal struct {
	Reason         string
	UserID         uuid.UUID
	OrganizationID uuid.UUID
	message        string
}

func (e *SignInRefusal) Error() string { return e.message }

func signInRefusal(reason string, user *domain.User, message string) *SignInRefusal {
	refusal := &SignInRefusal{Reason: reason, message: message}
	if user != nil {
		refusal.UserID = user.ID
		refusal.OrganizationID = user.OrganizationID
	}
	return refusal
}

// LoginWithPasswordExtended authenticates with additional context for
// failure tracking. A refused sign-in returns a *SignInRefusal. A sign-in
// with the user's password whose organization's requirement of provider
// sign-in cannot be read (CheckPasswordSignIn) returns another error, and no
// user.
func (s *AuthService) LoginWithPasswordExtended(ctx context.Context, email, password, ipAddress, userAgent string) (*domain.User, error) {
	// Check for active lockout before attempting authentication
	if s.authFailureRepo != nil {
		lockout, err := s.authFailureRepo.GetActiveLockout(email)
		if err != nil {
			// Log error but continue - don't block login on monitoring errors
			log.Printf("Warning: failed to check lockout status for a sign-in: %s\n", redactEmailAddresses(err.Error()))
		} else if lockout != nil {
			remainingMins := int(time.Until(lockout.LockedUntil).Minutes()) + 1
			return nil, signInRefusal(SignInRefusedAccountLocked, nil,
				fmt.Sprintf("account is temporarily locked due to too many failed attempts. Please try again in %d minutes", remainingMins))
		}
	}

	// Get user by email
	user, err := s.userRepo.GetByEmail(email)
	if err != nil {
		// Record failure for unknown user
		s.recordAuthFailure(email, nil, nil, domain.AuthFailureInvalidCredentials, ipAddress, userAgent)
		return nil, signInRefusal(SignInRefusedUnknownEmail, nil, "invalid credentials")
	}

	// Check if user account is deactivated
	if user.Status == domain.UserStatusDeactivated || user.DeletedAt != nil {
		s.recordAuthFailure(email, &user.OrganizationID, &user.ID, domain.AuthFailureAccountDeactivated, ipAddress, userAgent)
		return nil, signInRefusal(SignInRefusedAccountDeactivated, user,
			"your account has been deactivated. Please contact your administrator for assistance")
	}

	// Check if user has a password (local authentication enabled)
	if user.PasswordHash == nil || *user.PasswordHash == "" {
		return nil, signInRefusal(SignInRefusedPasswordNotSet, user, "local authentication not configured for this user")
	}

	// Verify password
	passwordHasher := auth.NewPasswordHasher()
	if err := passwordHasher.VerifyPassword(password, *user.PasswordHash); err != nil {
		s.recordAuthFailure(email, &user.OrganizationID, &user.ID, domain.AuthFailureInvalidCredentials, ipAddress, userAgent)
		return nil, signInRefusal(SignInRefusedPasswordMismatch, user, "invalid credentials")
	}

	// The requirement of provider sign-in is read after the password check,
	// so a caller without the password learns nothing of it. Its refusal is
	// not recorded as a failed attempt and clears none, and the sign-in is
	// not recorded in last_login_at.
	if err := s.CheckPasswordSignIn(ctx, user); err != nil {
		return nil, err
	}

	// Email verification removed - handled during registration approval

	// Clear previous failures on successful login
	if s.authFailureRepo != nil {
		if err := s.authFailureRepo.ClearFailures(email); err != nil {
			log.Printf("Warning: failed to clear auth failures for user %s: %s\n", user.ID, redactEmailAddresses(err.Error()))
		}
	}

	// Record the sign-in. Only last_login_at and updated_at are written: user was
	// read before the password check, and writing the whole row back would undo a
	// password change, deactivation or role change committed since then. The
	// instant is clocked in UTC: the caller's copy is what a sign-in response
	// serialises, and a time.Time prints with the location it carries.
	now := time.Now().UTC()
	user.LastLoginAt = &now
	user.UpdatedAt = now
	if err := s.userRepo.UpdateLastLogin(user.ID, now); err != nil {
		// Log error but don't fail the login - this is non-critical
		log.Printf("Warning: failed to update last_login_at for user %s: %v\n", user.ID, err)
	}

	return user, nil
}

// recordAuthFailure records a failed authentication attempt and handles lockout logic
func (s *AuthService) recordAuthFailure(email string, orgID, userID *uuid.UUID, failureType domain.AuthFailureType, ipAddress, userAgent string) {
	if s.authFailureRepo == nil {
		return
	}

	// Record the failure
	failure := &domain.AuthFailure{
		OrganizationID: orgID,
		UserID:         userID,
		Email:          email,
		FailureType:    failureType,
		IPAddress:      ipAddress,
		UserAgent:      userAgent,
		Metadata:       make(map[string]interface{}),
	}

	if err := s.authFailureRepo.RecordFailure(failure); err != nil {
		log.Printf("Warning: failed to record auth failure: %s\n", redactEmailAddresses(err.Error()))
		return
	}

	// Check if lockout threshold reached
	failureCount, err := s.authFailureRepo.CountRecentFailures(email, s.failureConfig.TimeWindowMins)
	if err != nil {
		log.Printf("Warning: failed to count auth failures: %s\n", redactEmailAddresses(err.Error()))
		return
	}

	// Create lockout if threshold exceeded
	if failureCount >= s.failureConfig.MaxAttempts {
		lockout := &domain.AuthLockout{
			OrganizationID: orgID,
			UserID:         userID,
			Email:          email,
			FailedAttempts: failureCount,
			LockedAt:       time.Now(),
			LockedUntil:    time.Now().Add(time.Duration(s.failureConfig.LockoutMins) * time.Minute),
		}

		if err := s.authFailureRepo.CreateLockout(lockout); err != nil {
			log.Printf("Warning: failed to create lockout: %s\n", redactEmailAddresses(err.Error()))
			return
		}

		// Trigger policy evaluation for alerts
		if s.policyService != nil && orgID != nil {
			s.policyService.EvaluateAuthFailures(email, *orgID, failureCount, true)
		}
	} else if failureCount >= s.failureConfig.AlertThreshold {
		// Trigger warning alert for approaching lockout
		if s.policyService != nil && orgID != nil {
			s.policyService.EvaluateAuthFailures(email, *orgID, failureCount, false)
		}
	}
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
		return ErrPasswordNotConfigured
	}

	passwordHasher := s.passwordHasher
	if passwordHasher == nil {
		passwordHasher = auth.NewPasswordHasher()
	}
	if err := passwordHasher.VerifyPassword(currentPassword, *user.PasswordHash); err != nil {
		return ErrCurrentPasswordIncorrect
	}

	if err := passwordHasher.ValidatePassword(newPassword); err != nil {
		return err
	}

	// Hash new password
	newHash, err := passwordHasher.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	// Update password in database, and record the change: every session
	// signed in before it is refused from now on (users.password_changed_at).
	// Microseconds are what the column stores, so the instant read back is
	// the one written.
	now := time.Now().UTC().Truncate(time.Microsecond)
	user.PasswordHash = &newHash
	user.ForcePasswordChange = false
	user.PasswordChangedAt = &now
	user.UpdatedAt = now

	if err := s.userRepo.Update(user); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	revokeSDKTokenRowsAtPasswordChange(s.sdkTokenRepo, user.ID)
	return nil
}

// revokeSDKTokenRowsAtPasswordChange marks each sdk_tokens row of the user
// not yet revoked as revoked, with the reason
// domain.SDKTokenRevokeReasonPasswordChanged, once a password change or reset
// is written, so the user's list of SDK tokens shows none of them active.
// Every token those rows track was minted before the change, which refuses
// it, so a failed write is logged and the change stands. Nil rows revoke
// nothing.
func revokeSDKTokenRowsAtPasswordChange(rows domain.SDKTokenRepository, userID uuid.UUID) {
	if rows == nil {
		return
	}
	if err := rows.RevokeAllForUser(userID, domain.SDKTokenRevokeReasonPasswordChanged); err != nil {
		log.Printf("Password change: sdk_tokens rows of user %s not revoked: %v", userID, err)
	}
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

// UpdateLastLogin updates a user's last_login_at timestamp. It writes only that
// and updated_at, never the rest of the caller's copy of the row. The instant
// is clocked in UTC, the form every response names a lastLoginAt in.
func (s *AuthService) UpdateLastLogin(ctx context.Context, user *domain.User) error {
	now := time.Now().UTC()
	user.LastLoginAt = &now
	user.UpdatedAt = now
	if err := s.userRepo.UpdateLastLogin(user.ID, now); err != nil {
		// Log error but don't fail - this is non-critical
		log.Printf("Warning: failed to update last_login_at for user %s: %v\n", user.ID, err)
		return err
	}
	return nil
}
