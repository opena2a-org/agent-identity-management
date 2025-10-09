package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
)

type OAuthHandler struct {
	oauthService *application.OAuthService
	authService  *application.AuthService
}

func NewOAuthHandler(oauthService *application.OAuthService, authService *application.AuthService) *OAuthHandler {
	return &OAuthHandler{
		oauthService: oauthService,
		authService:  authService,
	}
}

// generateState generates a random state parameter for OAuth flow
func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// InitiateOAuth redirects to OAuth provider
func (h *OAuthHandler) InitiateOAuth(c fiber.Ctx) error {
	provider := domain.OAuthProvider(c.Params("provider"))

	// Validate provider
	if provider != domain.OAuthProviderGoogle &&
		provider != domain.OAuthProviderMicrosoft &&
		provider != domain.OAuthProviderOkta {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid OAuth provider",
		})
	}

	// Generate state parameter
	state, err := generateState()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate state",
		})
	}

	// Store state in session (simple implementation - use Redis in production)
	c.Cookie(&fiber.Cookie{
		Name:     fmt.Sprintf("oauth_state_%s", provider),
		Value:    state,
		Expires:  time.Now().Add(10 * time.Minute),
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Lax",
	})

	// Get auth URL
	authURL, err := h.oauthService.GetAuthURL(provider, state)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get authorization URL",
		})
	}

	// Redirect to OAuth provider
	return c.Redirect().To(authURL)
}

// HandleOAuthCallback processes OAuth callback
func (h *OAuthHandler) HandleOAuthCallback(c fiber.Ctx) error {
	provider := domain.OAuthProvider(c.Params("provider"))

	// Validate provider
	if provider != domain.OAuthProviderGoogle &&
		provider != domain.OAuthProviderMicrosoft &&
		provider != domain.OAuthProviderOkta {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid OAuth provider",
		})
	}

	// Get code and state from query
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		errorMsg := c.Query("error")
		if errorMsg != "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":             "OAuth authorization failed",
				"error_description": c.Query("error_description"),
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing authorization code",
		})
	}

	// Verify state parameter
	storedState := c.Cookies(fmt.Sprintf("oauth_state_%s", provider))
	if storedState != state {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid state parameter",
		})
	}

	// Clear state cookie
	c.Cookie(&fiber.Cookie{
		Name:     fmt.Sprintf("oauth_state_%s", provider),
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		HTTPOnly: true,
	})

	// Try to log in existing user first
	token, _, err := h.oauthService.HandleOAuthLogin(c.Context(), provider, code)
	if err == nil {
		// User exists - return JWT token for login
		return c.Redirect().To(fmt.Sprintf("http://localhost:3000/auth/callback?token=%s", token))
	}

	// User doesn't exist - proceed with registration flow
	req, err := h.oauthService.HandleOAuthCallback(c.Context(), provider, code)
	if err != nil {
		if err == application.ErrUserAlreadyExists {
			// This shouldn't happen since we tried login first, but handle it anyway
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "User already exists. Please try logging in again.",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to process OAuth callback: %v", err),
		})
	}

	// Registration request created successfully
	// Redirect to success page
	return c.Redirect().To(fmt.Sprintf("http://localhost:3000/auth/registration-pending?request_id=%s", req.ID))
}

// ListPendingRegistrationRequests returns all pending registration requests
func (h *OAuthHandler) ListPendingRegistrationRequests(c fiber.Ctx) error {
	// Get authenticated user
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Get user to check admin role
	user, err := h.authService.GetUserByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get user",
		})
	}

	// Only admins can view registration requests
	if user.Role != domain.RoleAdmin {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Only admins can view registration requests",
		})
	}

	// Parse pagination params
	limit := c.Query("limit", "50")
	offset := c.Query("offset", "0")
	limitInt := 50
	offsetInt := 0
	if l, err := strconv.Atoi(limit); err == nil {
		limitInt = l
	}
	if o, err := strconv.Atoi(offset); err == nil {
		offsetInt = o
	}

	// Get pending requests
	requests, total, err := h.oauthService.ListPendingRegistrationRequests(c.Context(), user.OrganizationID, limitInt, offsetInt)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch registration requests",
		})
	}

	return c.JSON(fiber.Map{
		"requests": requests,
		"total":    total,
		"limit":    limitInt,
		"offset":   offsetInt,
	})
}

// ApproveRegistrationRequest approves a registration request
func (h *OAuthHandler) ApproveRegistrationRequest(c fiber.Ctx) error {
	// Get authenticated user
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Get user to check admin role
	user, err := h.authService.GetUserByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get user",
		})
	}

	// Only admins can approve registration requests
	if user.Role != domain.RoleAdmin {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Only admins can approve registration requests",
		})
	}

	// Get request ID from params
	requestID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request ID",
		})
	}

	// Approve registration request
	newUser, err := h.oauthService.ApproveRegistrationRequest(c.Context(), requestID, userID, user.OrganizationID)
	if err != nil {
		if err == application.ErrRegistrationNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Registration request not found",
			})
		}
		if err == application.ErrRegistrationNotPending {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Registration request is not pending",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to approve registration: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Registration request approved successfully",
		"user":    newUser,
	})
}

// RejectRegistrationRequest rejects a registration request
func (h *OAuthHandler) RejectRegistrationRequest(c fiber.Ctx) error {
	// Get authenticated user
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Get user to check admin role
	user, err := h.authService.GetUserByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get user",
		})
	}

	// Only admins can reject registration requests
	if user.Role != domain.RoleAdmin {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Only admins can reject registration requests",
		})
	}

	// Get request ID from params
	requestID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request ID",
		})
	}

	// Parse request body
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Reason == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Rejection reason is required",
		})
	}

	// Reject registration request
	if err := h.oauthService.RejectRegistrationRequest(c.Context(), requestID, userID, req.Reason); err != nil {
		if err == application.ErrRegistrationNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Registration request not found",
			})
		}
		if err == application.ErrRegistrationNotPending {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Registration request is not pending",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to reject registration: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Registration request rejected successfully",
	})
}

// RegisterRoutes registers OAuth routes
func (h *OAuthHandler) RegisterRoutes(app *fiber.App) {
	oauth := app.Group("/api/v1/oauth")

	// OAuth initiation (redirects to provider)
	oauth.Get("/:provider/login", h.InitiateOAuth)

	// OAuth callback (from provider)
	oauth.Get("/:provider/callback", h.HandleOAuthCallback)

	// Admin routes for managing registration requests
	admin := app.Group("/api/v1/admin/registration-requests")
	// TODO: Add authentication middleware
	admin.Get("/", h.ListPendingRegistrationRequests)
	admin.Post("/:id/approve", h.ApproveRegistrationRequest)
	admin.Post("/:id/reject", h.RejectRegistrationRequest)
}
