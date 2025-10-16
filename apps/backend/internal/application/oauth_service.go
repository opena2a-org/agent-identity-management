package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/opena2a/identity/backend/internal/infrastructure/auth"
)

var (
	ErrRegistrationNotFound      = errors.New("registration request not found")
	ErrRegistrationNotPending    = errors.New("registration request is not pending")
	ErrOAuthConnectionNotFound   = errors.New("OAuth connection not found")
	ErrUserAlreadyExists         = errors.New("user with this email already exists")
	ErrRegistrationRequestExists = errors.New("registration request with this email already exists")
	ErrUserNotFound              = errors.New("user not found")
	ErrUserNotApproved           = errors.New("user registration not approved")
	ErrPasswordAlreadySet        = errors.New("password already set for this user")
)

// OAuthProvider defines the interface for OAuth providers
type OAuthProvider interface {
	GetAuthURL(state string) string
	ExchangeCode(ctx context.Context, code string) (accessToken, refreshToken string, expiresIn int, err error)
	GetUserProfile(ctx context.Context, accessToken string) (*domain.OAuthProfile, error)
	GetProviderName() domain.OAuthProvider
}

// OAuthRepository defines the interface for OAuth data persistence
type OAuthRepository interface {
	// Registration requests
	CreateRegistrationRequest(ctx context.Context, req *domain.UserRegistrationRequest) error
	GetRegistrationRequest(ctx context.Context, id uuid.UUID) (*domain.UserRegistrationRequest, error)
	GetRegistrationRequestByOAuth(ctx context.Context, provider domain.OAuthProvider, providerUserID string) (*domain.UserRegistrationRequest, error)
	GetRegistrationRequestByEmail(ctx context.Context, email string) (*domain.UserRegistrationRequest, error)
	GetRegistrationRequestByEmailAnyStatus(ctx context.Context, email string) (*domain.UserRegistrationRequest, error)
	ListPendingRegistrationRequests(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.UserRegistrationRequest, int, error)
	UpdateRegistrationRequest(ctx context.Context, req *domain.UserRegistrationRequest) error

	// OAuth connections
	CreateOAuthConnection(ctx context.Context, conn *domain.OAuthConnection) error
	GetOAuthConnection(ctx context.Context, provider domain.OAuthProvider, providerUserID string) (*domain.OAuthConnection, error)
	GetOAuthConnectionsByUser(ctx context.Context, userID uuid.UUID) ([]*domain.OAuthConnection, error)
	UpdateOAuthConnection(ctx context.Context, conn *domain.OAuthConnection) error
	DeleteOAuthConnection(ctx context.Context, id uuid.UUID) error
}

// JWTService interface for JWT token generation
type JWTService interface {
	GenerateAccessToken(userID, orgID, email, role string) (string, error)
	GenerateTokenPair(userID, orgID, email, role string) (accessToken, refreshToken string, err error)
}

// OAuthService handles OAuth authentication and user registration
type OAuthService struct {
	oauthRepo    OAuthRepository
	userRepo     domain.UserRepository
	orgRepo      domain.OrganizationRepository
	authService  *AuthService
	auditService *AuditService
	jwtService   JWTService
	providers    map[domain.OAuthProvider]OAuthProvider
}

func NewOAuthService(
	oauthRepo OAuthRepository,
	userRepo domain.UserRepository,
	orgRepo domain.OrganizationRepository,
	authService *AuthService,
	auditService *AuditService,
	jwtService JWTService,
	providers map[domain.OAuthProvider]OAuthProvider,
) *OAuthService {
	return &OAuthService{
		oauthRepo:    oauthRepo,
		userRepo:     userRepo,
		orgRepo:      orgRepo,
		authService:  authService,
		auditService: auditService,
		jwtService:   jwtService,
		providers:    providers,
	}
}

// GetAuthURL returns the OAuth authorization URL for the specified provider
func (s *OAuthService) GetAuthURL(provider domain.OAuthProvider, state string) (string, error) {
	p, ok := s.providers[provider]
	if !ok {
		return "", fmt.Errorf("unsupported OAuth provider: %s", provider)
	}

	return p.GetAuthURL(state), nil
}

// HandleOAuthLogin processes OAuth callback for existing users and returns JWT tokens (access + refresh)
func (s *OAuthService) HandleOAuthLogin(
	ctx context.Context,
	provider domain.OAuthProvider,
	code string,
) (accessToken, refreshToken string, user *domain.User, err error) {
	// Get provider
	p, ok := s.providers[provider]
	if !ok {
		return "", "", nil, fmt.Errorf("unsupported OAuth provider: %s", provider)
	}

	// Exchange code for tokens
	oauthAccessToken, _, _, err := p.ExchangeCode(ctx, code)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// Get user profile from OAuth provider
	profile, err := p.GetUserProfile(ctx, oauthAccessToken)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to get user profile: %w", err)
	}

	// Check if user exists
	existingUser, err := s.userRepo.GetByEmail(profile.Email)
	if err != nil || existingUser == nil {
		return "", "", nil, fmt.Errorf("user not found: please register first")
	}

	// Generate JWT token pair (access + refresh) for existing user
	accessToken, refreshToken, err = s.jwtService.GenerateTokenPair(
		existingUser.ID.String(),
		existingUser.OrganizationID.String(),
		existingUser.Email,
		string(existingUser.Role),
	)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to generate token pair: %w", err)
	}

	// Update user's last login timestamp
	now := time.Now()
	existingUser.LastLoginAt = &now
	existingUser.UpdatedAt = now
	if err := s.userRepo.Update(existingUser); err != nil {
		// Log error but don't fail the login - this is non-critical
		fmt.Printf("Warning: failed to update last_login_at for user %s: %v\n", existingUser.ID, err)
	}

	// Update user's OAuth connection (refresh tokens, etc.)
	// TODO: Store/update OAuth connection with new access token

	// Log audit trail
	s.auditService.LogAction(
		ctx,
		existingUser.OrganizationID,
		existingUser.ID,
		domain.AuditActionLogin,
		"user",
		existingUser.ID,
		"", // IP address
		"", // User agent
		map[string]interface{}{
			"oauth_provider": provider,
			"login_method":   "oauth",
		},
	)

	return accessToken, refreshToken, existingUser, nil
}

// OAuthCallbackResult represents the result of processing an OAuth callback
type OAuthCallbackResult struct {
	IsLogin             bool                            `json:"is_login"`
	AccessToken         string                          `json:"access_token,omitempty"`
	RefreshToken        string                          `json:"refresh_token,omitempty"`
	User                *domain.User                    `json:"user,omitempty"`
	RegistrationRequest *domain.UserRegistrationRequest `json:"registration_request,omitempty"`
}

// ProcessOAuthCallback processes OAuth callback and handles both login and registration scenarios
func (s *OAuthService) ProcessOAuthCallback(
	ctx context.Context,
	provider domain.OAuthProvider,
	code string,
) (*OAuthCallbackResult, error) {
	// Get provider
	p, ok := s.providers[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported OAuth provider: %s", provider)
	}

	// Exchange code for tokens (only once!)
	oauthAccessToken, _, _, err := p.ExchangeCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// Get user profile from OAuth provider
	profile, err := p.GetUserProfile(ctx, oauthAccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get user profile: %w", err)
	}

	// Check if user exists
	existingUser, err := s.userRepo.GetByEmail(profile.Email)
	if err == nil && existingUser != nil {
		// User exists - handle login
		accessToken, refreshToken, err := s.jwtService.GenerateTokenPair(
			existingUser.ID.String(),
			existingUser.OrganizationID.String(),
			existingUser.Email,
			string(existingUser.Role),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to generate token pair: %w", err)
		}

		// Log audit trail
		s.auditService.LogAction(
			ctx,
			existingUser.OrganizationID,
			existingUser.ID,
			domain.AuditActionLogin,
			"user",
			existingUser.ID,
			"", // IP address
			"", // User agent
			map[string]interface{}{
				"oauth_provider": provider,
				"login_method":   "oauth",
			},
		)

		return &OAuthCallbackResult{
			IsLogin:      true,
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			User:         existingUser,
		}, nil
	}

	// User doesn't exist - always create registration request for manual approval
	// Always require admin approval for new users (no auto-provisioning)
	// Create registration request for manual approval
	registrationRequest, err := s.createRegistrationRequest(ctx, provider, profile)
	if err != nil {
		return nil, fmt.Errorf("failed to create registration request: %w", err)
	}

	return &OAuthCallbackResult{
		IsLogin:             false,
		RegistrationRequest: registrationRequest,
	}, nil
}

// createRegistrationRequest creates a new user registration request
func (s *OAuthService) createRegistrationRequest(
	ctx context.Context,
	provider domain.OAuthProvider,
	profile *domain.OAuthProfile,
) (*domain.UserRegistrationRequest, error) {
	// Check if registration request already exists
	existingReq, err := s.oauthRepo.GetRegistrationRequestByOAuth(ctx, provider, profile.ProviderUserID)
	if err == nil && existingReq != nil {
		// Update existing request if it was rejected before
		if existingReq.IsRejected() {
			existingReq.Status = domain.RegistrationStatusPending
			existingReq.UpdatedAt = time.Now()
			if err := s.oauthRepo.UpdateRegistrationRequest(ctx, existingReq); err != nil {
				return nil, fmt.Errorf("failed to update registration request: %w", err)
			}
			return existingReq, nil
		}
		return existingReq, nil
	}

	// Create new registration request
	req := domain.NewUserRegistrationRequestOAuth(
		profile.Email,
		profile.FirstName,
		profile.LastName,
		provider,
		profile.ProviderUserID,
		profile,
	)

	if err := s.oauthRepo.CreateRegistrationRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("failed to create registration request: %w", err)
	}

	// TODO: Send notification to admins

	return req, nil
}

// HandleOAuthCallback processes the OAuth callback and creates a registration request
// DEPRECATED: Use ProcessOAuthCallback instead to avoid double code exchange
func (s *OAuthService) HandleOAuthCallback(
	ctx context.Context,
	provider domain.OAuthProvider,
	code string,
) (*domain.UserRegistrationRequest, error) {
	// Get provider
	p, ok := s.providers[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported OAuth provider: %s", provider)
	}

	// Exchange code for tokens
	accessToken, _, _, err := p.ExchangeCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// Get user profile
	profile, err := p.GetUserProfile(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get user profile: %w", err)
	}

	// Check if user already exists
	existingUser, err := s.userRepo.GetByEmail(profile.Email)
	if err == nil && existingUser != nil {
		return nil, ErrUserAlreadyExists
	}

	return s.createRegistrationRequest(ctx, provider, profile)
}

// extractEmailDomain extracts domain from email address
func extractEmailDomain(email string) string {
	for i := len(email) - 1; i >= 0; i-- {
		if email[i] == '@' {
			return email[i+1:]
		}
	}
	return ""
}

// getOrCreateOrganization finds or creates an organization by domain
func (s *OAuthService) getOrCreateOrganization(ctx context.Context, domainName string) (*domain.Organization, error) {
	// Try to find existing organization
	org, err := s.orgRepo.GetByDomain(domainName)
	if err == nil && org != nil {
		return org, nil
	}

	// Create new organization with manual approval required by default
	org = &domain.Organization{
		ID:             uuid.New(),
		Name:           domainName,
		Domain:         domainName,
		PlanType:       "free",
		MaxAgents:      100,
		MaxUsers:       10,
		IsActive:       true,
		AutoApproveSSO: false, // Require manual approval for all new users
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.orgRepo.Create(org); err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	return org, nil
}

// autoProvisionUser creates a new user automatically
func (s *OAuthService) autoProvisionUser(ctx context.Context, provider domain.OAuthProvider, profile *domain.OAuthProfile, org *domain.Organization) (*domain.User, error) {
	// Check if this is the first user (make them admin)
	existingUsers, err := s.userRepo.GetByOrganization(org.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing users: %w", err)
	}

	role := domain.RoleViewer
	if len(existingUsers) == 0 {
		role = domain.RoleAdmin // First user becomes admin
	}

	// Create full name
	fullName := profile.FirstName
	if profile.LastName != "" {
		if fullName != "" {
			fullName += " "
		}
		fullName += profile.LastName
	}
	if fullName == "" {
		fullName = profile.Email // Fallback to email
	}

	// Create user
	user := &domain.User{
		ID:             uuid.New(),
		OrganizationID: org.ID,
		Email:          profile.Email,
		Name:           fullName,
		Role:           role,
		Provider:       string(provider),
		ProviderID:     profile.ProviderUserID,
		EmailVerified:  profile.EmailVerified,
		Status:         domain.UserStatusActive, // Auto-approved users are active
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if profile.PictureURL != "" {
		user.AvatarURL = &profile.PictureURL
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// ListPendingRegistrationRequests returns all pending registration requests for an organization
func (s *OAuthService) ListPendingRegistrationRequests(
	ctx context.Context,
	orgID uuid.UUID,
	limit, offset int,
) ([]*domain.UserRegistrationRequest, int, error) {
	return s.oauthRepo.ListPendingRegistrationRequests(ctx, orgID, limit, offset)
}

// ApproveRegistrationRequest approves a registration request and creates the user account
func (s *OAuthService) ApproveRegistrationRequest(
	ctx context.Context,
	requestID uuid.UUID,
	reviewerID uuid.UUID,
	orgID uuid.UUID,
) (*domain.User, error) {
	// Get registration request
	req, err := s.oauthRepo.GetRegistrationRequest(ctx, requestID)
	if err != nil {
		return nil, ErrRegistrationNotFound
	}

	if !req.IsPending() {
		return nil, ErrRegistrationNotPending
	}

	// Approve request
	req.Approve(reviewerID)
	if err := s.oauthRepo.UpdateRegistrationRequest(ctx, req); err != nil {
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

	// Determine provider and provider ID based on registration type
	provider := "local" // Default for manual registrations
	providerID := req.Email // Use email as provider_id for local/manual registrations to ensure uniqueness
	emailVerified := false
	
	if req.OAuthProvider != nil {
		provider = string(*req.OAuthProvider)
		
		if provider != "local" && req.OAuthUserID != nil {
			providerID = *req.OAuthUserID
			emailVerified = req.OAuthEmailVerified
		} else {
			provider = "local"
			providerID = req.Email
			emailVerified = false
		}
	} else {
		provider = "local"
		providerID = req.Email
		emailVerified = false
	}

	user := &domain.User{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Email:          req.Email,
		Name:           fullName,
		Role:           domain.RoleViewer, // Default to viewer role for new users
		Provider:       provider,
		ProviderID:     providerID,
		PasswordHash:   req.PasswordHash, // Will be set for manual registrations, nil for OAuth
		EmailVerified:  emailVerified,
		ApprovedBy:     &reviewerID,
		ApprovedAt:     &time.Time{},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	
	if req.PasswordHash != nil && *req.PasswordHash != "" {
		fmt.Printf("✅ Approving user with password hash (provider: %s)\n", provider)
	} else {
		fmt.Printf("ℹ️  Approving user without password hash (OAuth provider: %s)\n", provider)
	}
	
	// Set approval timestamp
	now := time.Now()
	user.ApprovedAt = &now

	// Create user via repository
	if err := s.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Log audit with appropriate registration method
	registrationMethod := "manual_self_registration"
	auditData := map[string]interface{}{
		"registration_id":     req.ID,
		"registration_method": registrationMethod,
	}
	
	if req.OAuthProvider != nil {
		registrationMethod = "oauth_self_registration"
		auditData["oauth_provider"] = *req.OAuthProvider
		auditData["registration_method"] = registrationMethod
	}

	s.auditService.LogAction(
		ctx,
		orgID,
		reviewerID,
		domain.AuditActionCreate,
		"user",
		user.ID,
		"", // IP address
		"", // User agent
		auditData,
	)

	// TODO: Send approval email to user

	return user, nil
}

// RejectRegistrationRequest rejects a registration request
func (s *OAuthService) RejectRegistrationRequest(
	ctx context.Context,
	requestID uuid.UUID,
	reviewerID uuid.UUID,
	reason string,
) error {
	// Get registration request
	req, err := s.oauthRepo.GetRegistrationRequest(ctx, requestID)
	if err != nil {
		return ErrRegistrationNotFound
	}

	if !req.IsPending() {
		return ErrRegistrationNotPending
	}

	// Reject request
	req.Reject(reviewerID, reason)
	if err := s.oauthRepo.UpdateRegistrationRequest(ctx, req); err != nil {
		return fmt.Errorf("failed to update registration request: %w", err)
	}

	// TODO: Send rejection email to user

	return nil
}

// CreateManualRegistrationRequest creates a registration request for manual (non-OAuth) user registration
func (s *OAuthService) CreateManualRegistrationRequest(
	ctx context.Context,
	email, firstName, lastName, password string,
) (*domain.UserRegistrationRequest, error) {
	// Check if user already exists
	existingUser, err := s.userRepo.GetByEmail(email)
	if err == nil && existingUser != nil {
		return nil, ErrUserAlreadyExists
	}

	// Check if a registration request already exists for this email
	existingRequest, err := s.oauthRepo.GetRegistrationRequestByEmail(ctx, email)
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
	if err := s.oauthRepo.CreateRegistrationRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("failed to create registration request: %w", err)
	}

	return req, nil
}

// GetRegistrationRequest retrieves a registration request by ID - exposed for public use
func (s *OAuthService) GetRegistrationRequest(ctx context.Context, requestID uuid.UUID) (*domain.UserRegistrationRequest, error) {
	return s.oauthRepo.GetRegistrationRequest(ctx, requestID)
}

// GetRegistrationRequestByEmail retrieves a registration request by email - exposed for public login
func (s *OAuthService) GetRegistrationRequestByEmail(ctx context.Context, email string) (*domain.UserRegistrationRequest, error) {
	// Use the any status method to find registration requests regardless of status
	return s.oauthRepo.GetRegistrationRequestByEmailAnyStatus(ctx, email)
}


// hashToken creates a SHA-256 hash of a token
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
