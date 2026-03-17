package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// Sentinel errors for the device authorization grant flow (RFC 8628 Section 3.5).
var (
	ErrAuthorizationPending = errors.New("authorization_pending")
	ErrSlowDown             = errors.New("slow_down")
	ErrExpiredToken         = errors.New("expired_token")
	ErrAccessDenied         = errors.New("access_denied")
	ErrInvalidDeviceCode    = errors.New("invalid_device_code")
	ErrInvalidUserCode      = errors.New("invalid_user_code")
)

// deviceCodeExpiry is the lifetime of a device authorization code.
const deviceCodeExpiry = 15 * time.Minute

// defaultPollInterval is the minimum number of seconds between token poll requests.
const defaultPollInterval = 5

// userCodeCharset excludes vowels (to avoid offensive words) and ambiguous characters (0/O/1/I/l).
const userCodeCharset = "BCDFGHJKLMNPQRSTVWXZ23456789"

// userCodeLength is the number of random characters in a user code.
const userCodeLength = 8

// deviceCodeBytes is the number of random bytes for a device code (hex-encoded to 80 chars).
const deviceCodeBytes = 40

// DeviceAuthResponse is returned when a device authorization request is initiated.
type DeviceAuthResponse struct {
	DeviceCode              string `json:"deviceCode"`
	UserCode                string `json:"userCode"`
	VerificationURI         string `json:"verificationUri"`
	VerificationURIComplete string `json:"verificationUriComplete"`
	ExpiresIn               int    `json:"expiresIn"`
	Interval                int    `json:"interval"`
}

// DeviceTokenResponse is returned when a device code has been approved and tokens are issued.
type DeviceTokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	TokenType    string `json:"tokenType"`
	ExpiresIn    int    `json:"expiresIn"`
}

// DeviceAuthService handles the device authorization grant flow (RFC 8628).
type DeviceAuthService struct {
	deviceCodeRepo domain.DeviceCodeRepository
	jwtService     *auth.JWTService
	authService    *AuthService
	baseURL        string
}

// NewDeviceAuthService creates a new DeviceAuthService.
func NewDeviceAuthService(
	repo domain.DeviceCodeRepository,
	jwtService *auth.JWTService,
	authService *AuthService,
	baseURL string,
) *DeviceAuthService {
	return &DeviceAuthService{
		deviceCodeRepo: repo,
		jwtService:     jwtService,
		authService:    authService,
		baseURL:        strings.TrimRight(baseURL, "/"),
	}
}

// InitiateDeviceAuth creates a new device authorization request.
// It generates a device code and user code, stores them in the database,
// and returns the information the CLI needs to display to the user.
func (s *DeviceAuthService) InitiateDeviceAuth(
	ctx context.Context,
	clientID string,
	scope string,
	ipAddress string,
	userAgent string,
) (*DeviceAuthResponse, error) {
	if clientID == "" {
		clientID = "opena2a-cli"
	}

	// Generate cryptographically random device code (80 hex chars = 320 bits)
	deviceCodeRaw := make([]byte, deviceCodeBytes)
	if _, err := rand.Read(deviceCodeRaw); err != nil {
		return nil, fmt.Errorf("failed to generate device code: %w", err)
	}
	deviceCode := hex.EncodeToString(deviceCodeRaw)

	// Generate cryptographically random user code (8 chars from safe charset)
	userCode, err := generateUserCode()
	if err != nil {
		return nil, fmt.Errorf("failed to generate user code: %w", err)
	}

	verificationURI := s.baseURL + "/device"
	formattedUserCode := formatUserCode(userCode)

	dc := &domain.DeviceCode{
		DeviceCode:      deviceCode,
		UserCode:        userCode, // stored without dash
		ClientID:        clientID,
		Scope:           scope,
		VerificationURI: verificationURI,
		ExpiresAt:       time.Now().Add(deviceCodeExpiry),
		IntervalSeconds: defaultPollInterval,
		Status:          domain.DeviceCodeStatusPending,
		IPAddress:       ipAddress,
		UserAgent:       userAgent,
	}

	if err := s.deviceCodeRepo.Create(ctx, dc); err != nil {
		return nil, fmt.Errorf("failed to store device code: %w", err)
	}

	return &DeviceAuthResponse{
		DeviceCode:              deviceCode,
		UserCode:                formattedUserCode,
		VerificationURI:         verificationURI,
		VerificationURIComplete: verificationURI + "?user_code=" + formattedUserCode,
		ExpiresIn:               int(deviceCodeExpiry.Seconds()),
		Interval:                defaultPollInterval,
	}, nil
}

// PollToken checks whether the device code has been approved and returns tokens if so.
// Returns sentinel errors per RFC 8628 Section 3.5 for each pending state.
func (s *DeviceAuthService) PollToken(ctx context.Context, deviceCode string) (*DeviceTokenResponse, error) {
	dc, err := s.deviceCodeRepo.GetByDeviceCode(ctx, deviceCode)
	if err != nil {
		return nil, ErrInvalidDeviceCode
	}

	// Check expiration first
	if dc.IsExpired() {
		return nil, ErrExpiredToken
	}

	switch dc.Status {
	case domain.DeviceCodeStatusPending:
		return nil, ErrAuthorizationPending

	case domain.DeviceCodeStatusDenied:
		return nil, ErrAccessDenied

	case domain.DeviceCodeStatusApproved:
		if dc.UserID == nil || dc.OrganizationID == nil {
			return nil, fmt.Errorf("device code approved but missing user or organization association")
		}

		// Retrieve user to get email and role for token claims
		user, err := s.authService.GetUserByID(ctx, *dc.UserID)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve user for token generation: %w", err)
		}

		accessToken, refreshToken, err := s.jwtService.GenerateTokenPair(
			dc.UserID.String(),
			dc.OrganizationID.String(),
			user.Email,
			string(user.Role),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to generate token pair: %w", err)
		}

		return &DeviceTokenResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			TokenType:    "Bearer",
			ExpiresIn:    7200, // 2 hours, matching JWTService default
		}, nil

	default:
		return nil, fmt.Errorf("unexpected device code status: %s", dc.Status)
	}
}

// ApproveDeviceCode marks a device authorization request as approved.
// Called by the web UI after the user logs in and confirms the user code.
func (s *DeviceAuthService) ApproveDeviceCode(ctx context.Context, userCode string, userID uuid.UUID, orgID uuid.UUID) error {
	// Normalize user code: strip dashes for DB lookup
	normalizedCode := strings.ReplaceAll(userCode, "-", "")

	// Verify the code exists and is still pending
	dc, err := s.deviceCodeRepo.GetByUserCode(ctx, normalizedCode)
	if err != nil {
		return ErrInvalidUserCode
	}

	if dc.IsExpired() {
		return ErrExpiredToken
	}

	if dc.Status != domain.DeviceCodeStatusPending {
		return fmt.Errorf("device code is no longer pending (status: %s)", dc.Status)
	}

	return s.deviceCodeRepo.Approve(ctx, normalizedCode, userID, orgID)
}

// DenyDeviceCode marks a device authorization request as denied.
func (s *DeviceAuthService) DenyDeviceCode(ctx context.Context, userCode string) error {
	normalizedCode := strings.ReplaceAll(userCode, "-", "")

	dc, err := s.deviceCodeRepo.GetByUserCode(ctx, normalizedCode)
	if err != nil {
		return ErrInvalidUserCode
	}

	if dc.IsExpired() {
		return ErrExpiredToken
	}

	if dc.Status != domain.DeviceCodeStatusPending {
		return fmt.Errorf("device code is no longer pending (status: %s)", dc.Status)
	}

	return s.deviceCodeRepo.Deny(ctx, normalizedCode)
}

// GetDeviceCodeByUserCode retrieves a device code by user code for the verification page.
func (s *DeviceAuthService) GetDeviceCodeByUserCode(ctx context.Context, userCode string) (*domain.DeviceCode, error) {
	normalizedCode := strings.ReplaceAll(userCode, "-", "")
	return s.deviceCodeRepo.GetByUserCode(ctx, normalizedCode)
}

// CleanupExpired removes device codes that have passed their expiration time.
func (s *DeviceAuthService) CleanupExpired(ctx context.Context) error {
	removed, err := s.deviceCodeRepo.CleanupExpired(ctx)
	if err != nil {
		return fmt.Errorf("failed to clean up expired device codes: %w", err)
	}
	if removed > 0 {
		log.Printf("Cleaned up %d expired device codes", removed)
	}
	return nil
}

// generateUserCode produces a random user code from the safe charset.
// The code is 8 characters long, stored without dashes in the database.
func generateUserCode() (string, error) {
	code := make([]byte, userCodeLength)
	charsetLen := len(userCodeCharset)

	for i := range code {
		b := make([]byte, 1)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		code[i] = userCodeCharset[int(b[0])%charsetLen]
	}

	return string(code), nil
}

// formatUserCode inserts a dash in the middle for display (XXXX-XXXX).
func formatUserCode(code string) string {
	if len(code) <= 4 {
		return code
	}
	return code[:4] + "-" + code[4:]
}
