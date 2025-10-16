package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/infrastructure/auth"
)

type AuthHandler struct {
	authService  *application.AuthService
	oauthService *auth.OAuthService
	jwtService   *auth.JWTService
}

func NewAuthHandler(
	authService *application.AuthService,
	oauthService *auth.OAuthService,
	jwtService *auth.JWTService,
) *AuthHandler {
	return &AuthHandler{
		authService:  authService,
		oauthService: oauthService,
		jwtService:   jwtService,
	}
}

// Login initiates OAuth login flow
func (h *AuthHandler) Login(c fiber.Ctx) error {
	provider := c.Params("provider")

	// Build OAuth URL manually with auth callback route
	var clientID, authURL, scope string
	redirectURI := fmt.Sprintf("http://localhost:8080/api/v1/auth/callback/%s", provider)

	switch provider {
	case "google":
		clientID = os.Getenv("GOOGLE_CLIENT_ID")
		authURL = "https://accounts.google.com/o/oauth2/v2/auth"
		scope = "openid email profile"
	case "microsoft":
		clientID = os.Getenv("MICROSOFT_CLIENT_ID")
		authURL = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
		scope = "openid email profile User.Read"
	case "okta":
		clientID = os.Getenv("OKTA_CLIENT_ID")
		oktaDomain := os.Getenv("OKTA_DOMAIN")
		authURL = fmt.Sprintf("https://%s/oauth2/v1/authorize", oktaDomain)
		scope = "openid email profile"
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid OAuth provider",
		})
	}

	// Generate state for CSRF protection
	state := uuid.New().String()

	// Build the OAuth authorization URL
	params := url.Values{}
	params.Add("client_id", clientID)
	params.Add("redirect_uri", redirectURI)
	params.Add("response_type", "code")
	params.Add("scope", scope)
	params.Add("state", state)

	fullAuthURL := fmt.Sprintf("%s?%s", authURL, params.Encode())

	return c.JSON(fiber.Map{
		"redirect_url": fullAuthURL,
	})
}

// Callback handles OAuth callback
func (h *AuthHandler) Callback(c fiber.Ctx) error {
	provider := c.Params("provider")
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing authorization code",
		})
	}

	var oauthProvider auth.OAuthProvider
	var clientID, clientSecret, tokenURL string
	redirectURI := fmt.Sprintf("http://localhost:8080/api/v1/auth/callback/%s", provider)

	switch provider {
	case "google":
		oauthProvider = auth.ProviderGoogle
		clientID = os.Getenv("GOOGLE_CLIENT_ID")
		clientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
		tokenURL = "https://oauth2.googleapis.com/token"
	case "microsoft":
		oauthProvider = auth.ProviderMicrosoft
		clientID = os.Getenv("MICROSOFT_CLIENT_ID")
		clientSecret = os.Getenv("MICROSOFT_CLIENT_SECRET")
		tokenURL = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	case "okta":
		oauthProvider = auth.ProviderOkta
		clientID = os.Getenv("OKTA_CLIENT_ID")
		clientSecret = os.Getenv("OKTA_CLIENT_SECRET")
		oktaDomain := os.Getenv("OKTA_DOMAIN")
		tokenURL = fmt.Sprintf("https://%s/oauth2/v1/token", oktaDomain)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid OAuth provider",
		})
	}

	// Exchange code for token manually with correct redirect URI
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)

	req, err := http.NewRequestWithContext(c.Context(), "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create token request",
		})
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Failed to exchange authorization code",
		})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Token exchange failed",
		})
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to parse token response",
		})
	}

	// Get user info from provider
	oauthUser, err := h.oauthService.GetUserInfo(c.Context(), oauthProvider, tokenResp.AccessToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Failed to get user info",
		})
	}

	// Login or create user (this also updates last_login_at)
	user, err := h.authService.LoginWithOAuth(c.Context(), oauthUser)
	if err != nil {
		fmt.Printf("ERROR in LoginWithOAuth: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to process login: %v", err),
		})
	}

	// Generate JWT tokens
	accessToken, refreshToken, err := h.jwtService.GenerateTokenPair(
		user.ID.String(),
		user.OrganizationID.String(),
		user.Email,
		string(user.Role),
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate tokens",
		})
	}

	// Set cookies
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		HTTPOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: "Lax",
	})

	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		HTTPOnly: true,
		Secure:   false,
		SameSite: "Lax",
	})

	// Redirect to frontend with token in URL (for localStorage)
	// In production, use more secure methods or proxy frontend through same domain
	frontendURL := "http://localhost:3000/dashboard?auth=success&token=" + accessToken
	if state != "" {
		frontendURL += "&state=" + state
	}

	return c.Redirect().To(frontendURL)
}

// Me returns current user info
func (h *AuthHandler) Me(c fiber.Ctx) error {
	// Get user_id from context (set by auth middleware)
	userIDValue := c.Locals("user_id")
	if userIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized - no user context",
		})
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized - invalid user context",
		})
	}

	user, err := h.authService.GetUserByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	return c.JSON(fiber.Map{
		"id":              user.ID,
		"email":           user.Email,
		"name":            user.Name,
		"role":            user.Role,
		"organization_id": user.OrganizationID,
		"provider":        user.Provider,
		"last_login_at":   user.LastLoginAt,
		"created_at":      user.CreatedAt,
	})
}

// LocalLogin handles email/password authentication
func (h *AuthHandler) LocalLogin(c fiber.Ctx) error {
	type LoginRequest struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	var req LoginRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate input
	if req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email and password are required",
		})
	}

	// Authenticate user (this also updates last_login_at)
	user, err := h.authService.LoginWithPassword(c.Context(), req.Email, req.Password)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid email or password",
		})
	}

	// Generate JWT tokens
	accessToken, refreshToken, err := h.jwtService.GenerateTokenPair(
		user.ID.String(),
		user.OrganizationID.String(),
		user.Email,
		string(user.Role),
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate tokens",
		})
	}

	// Set cookies
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		HTTPOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: "Lax",
	})

	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		HTTPOnly: true,
		Secure:   false,
		SameSite: "Lax",
	})

	return c.JSON(fiber.Map{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user": fiber.Map{
			"id":                    user.ID,
			"email":                 user.Email,
			"name":                  user.Name,
			"role":                  user.Role,
			"organization_id":       user.OrganizationID,
			"force_password_change": user.ForcePasswordChange,
		},
	})
}

// ChangePassword handles password change requests
func (h *AuthHandler) ChangePassword(c fiber.Ctx) error {
	type ChangePasswordRequest struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	var req ChangePasswordRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate input
	if req.CurrentPassword == "" || req.NewPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Current password and new password are required",
		})
	}

	// Get user_id from context (set by auth middleware)
	userIDValue := c.Locals("user_id")
	if userIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized - no user context",
		})
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized - invalid user context",
		})
	}

	// Change password
	err := h.authService.ChangePassword(c.Context(), userID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Password changed successfully",
	})
}

// Logout clears authentication
func (h *AuthHandler) Logout(c fiber.Ctx) error {
	// Clear cookies
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    "",
		HTTPOnly: true,
		MaxAge:   -1,
	})

	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    "",
		HTTPOnly: true,
		MaxAge:   -1,
	})

	return c.JSON(fiber.Map{
		"message": "Logged out successfully",
	})
}
