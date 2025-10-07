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
)

var (
	ErrRegistrationNotFound    = errors.New("registration request not found")
	ErrRegistrationNotPending  = errors.New("registration request is not pending")
	ErrOAuthConnectionNotFound = errors.New("OAuth connection not found")
	ErrUserAlreadyExists       = errors.New("user with this email already exists")
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
	ListPendingRegistrationRequests(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.UserRegistrationRequest, int, error)
	UpdateRegistrationRequest(ctx context.Context, req *domain.UserRegistrationRequest) error

	// OAuth connections
	CreateOAuthConnection(ctx context.Context, conn *domain.OAuthConnection) error
	GetOAuthConnection(ctx context.Context, provider domain.OAuthProvider, providerUserID string) (*domain.OAuthConnection, error)
	GetOAuthConnectionsByUser(ctx context.Context, userID uuid.UUID) ([]*domain.OAuthConnection, error)
	UpdateOAuthConnection(ctx context.Context, conn *domain.OAuthConnection) error
	DeleteOAuthConnection(ctx context.Context, id uuid.UUID) error
}

// OAuthService handles OAuth authentication and user registration
type OAuthService struct {
	oauthRepo    OAuthRepository
	userRepo     domain.UserRepository
	authService  *AuthService
	auditService *AuditService
	providers    map[domain.OAuthProvider]OAuthProvider
}

func NewOAuthService(
	oauthRepo OAuthRepository,
	userRepo domain.UserRepository,
	authService *AuthService,
	auditService *AuditService,
	providers map[domain.OAuthProvider]OAuthProvider,
) *OAuthService {
	return &OAuthService{
		oauthRepo:    oauthRepo,
		userRepo:     userRepo,
		authService:  authService,
		auditService: auditService,
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

// HandleOAuthCallback processes the OAuth callback and creates a registration request
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
	req := domain.NewUserRegistrationRequest(
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

	oauthProvider := string(req.OAuthProvider)
	user := &domain.User{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Email:          req.Email,
		Name:           fullName,
		Role:           domain.RoleViewer, // Default to viewer role for new users
		Provider:       oauthProvider,
		ProviderID:     req.OAuthUserID,
		EmailVerified:  req.OAuthEmailVerified,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Create user via repository
	if err := s.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

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
			"oauth_provider":      req.OAuthProvider,
			"registration_id":     req.ID,
			"registration_method": "oauth_self_registration",
		},
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

// hashToken creates a SHA-256 hash of a token
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
