package handlers

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
)

// DeviceAuthHandler handles HTTP endpoints for the OAuth Device Authorization Grant (RFC 8628).
type DeviceAuthHandler struct {
	deviceAuthService *application.DeviceAuthService
}

// NewDeviceAuthHandler creates a new DeviceAuthHandler.
func NewDeviceAuthHandler(deviceAuthService *application.DeviceAuthService) *DeviceAuthHandler {
	return &DeviceAuthHandler{
		deviceAuthService: deviceAuthService,
	}
}

// RequestDeviceCode handles POST /oauth/device/code
// Initiates a device authorization flow by generating a device code and user code.
// RFC 8628 Section 3.1 - Device Authorization Request
func (h *DeviceAuthHandler) RequestDeviceCode(c fiber.Ctx) error {
	type request struct {
		ClientID string `json:"clientId"`
		Scope    string `json:"scope"`
	}

	var req request
	if err := c.Bind().JSON(&req); err != nil {
		// Allow empty body (clientId and scope are optional)
		req = request{}
	}

	resp, err := h.deviceAuthService.InitiateDeviceAuth(
		c.Context(),
		req.ClientID,
		req.Scope,
		c.IP(),
		c.Get("User-Agent"),
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":            "server_error",
			"errorDescription": "Failed to initiate device authorization",
		})
	}

	return c.JSON(fiber.Map{
		"deviceCode":              resp.DeviceCode,
		"userCode":                resp.UserCode,
		"verificationUri":         resp.VerificationURI,
		"verificationUriComplete": resp.VerificationURIComplete,
		"expiresIn":               resp.ExpiresIn,
		"interval":                resp.Interval,
	})
}

// PollDeviceToken handles POST /oauth/device/token
// Polls for the status of a device authorization request.
// RFC 8628 Section 3.4 - Device Access Token Request
func (h *DeviceAuthHandler) PollDeviceToken(c fiber.Ctx) error {
	type request struct {
		DeviceCode string `json:"deviceCode"`
		GrantType  string `json:"grantType"`
	}

	var req request
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":            "invalid_request",
			"errorDescription": "Invalid request body",
		})
	}

	// RFC 8628 requires the grant_type to be the device code grant type
	if req.GrantType != "urn:ietf:params:oauth:grant-type:device_code" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":            "unsupported_grant_type",
			"errorDescription": "Grant type must be urn:ietf:params:oauth:grant-type:device_code",
		})
	}

	if req.DeviceCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":            "invalid_request",
			"errorDescription": "Device code is required",
		})
	}

	resp, err := h.deviceAuthService.PollToken(c.Context(), req.DeviceCode)
	if err != nil {
		// Map sentinel errors to RFC 8628 error responses
		switch {
		case errors.Is(err, application.ErrAuthorizationPending):
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "authorization_pending",
			})
		case errors.Is(err, application.ErrSlowDown):
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "slow_down",
			})
		case errors.Is(err, application.ErrExpiredToken):
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "expired_token",
			})
		case errors.Is(err, application.ErrAccessDenied):
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "access_denied",
			})
		case errors.Is(err, application.ErrInvalidDeviceCode):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":            "invalid_request",
				"errorDescription": "Unknown device code",
			})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":            "server_error",
				"errorDescription": "Failed to process token request",
			})
		}
	}

	return c.JSON(fiber.Map{
		"accessToken":  resp.AccessToken,
		"refreshToken": resp.RefreshToken,
		"tokenType":    resp.TokenType,
		"expiresIn":    resp.ExpiresIn,
	})
}

// ApproveDevice handles POST /oauth/device/approve
// Approves a pending device authorization request. Requires authentication.
// Called by the web UI after the user logs in and confirms the user code.
func (h *DeviceAuthHandler) ApproveDevice(c fiber.Ctx) error {
	type request struct {
		UserCode string `json:"userCode"`
	}

	var req request
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.UserCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User code is required",
		})
	}

	// Get authenticated user from context (set by auth middleware)
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

	orgIDValue := c.Locals("organization_id")
	if orgIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized - no organization context",
		})
	}
	orgID, ok := orgIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized - invalid organization context",
		})
	}

	err := h.deviceAuthService.ApproveDeviceCode(c.Context(), req.UserCode, userID, orgID)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrInvalidUserCode):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User code not found or no longer pending",
			})
		case errors.Is(err, application.ErrExpiredToken):
			return c.Status(fiber.StatusGone).JSON(fiber.Map{
				"error": "Device code has expired",
			})
		default:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
	}

	return c.JSON(fiber.Map{
		"message": "Device authorized successfully",
	})
}

// GetDeviceVerification handles GET /oauth/device/verify?user_code=XXXX-XXXX
// Returns device code information for the verification page.
// The Next.js dashboard renders the actual approval UI.
func (h *DeviceAuthHandler) GetDeviceVerification(c fiber.Ctx) error {
	userCode := c.Query("user_code")

	// If no user_code provided, return endpoint status
	if userCode == "" {
		return c.JSON(fiber.Map{
			"status":  "ready",
			"message": "Device verification endpoint. Provide user_code query parameter.",
		})
	}

	// Normalize: strip dashes for DB lookup
	normalizedCode := strings.ReplaceAll(userCode, "-", "")

	dc, err := h.deviceAuthService.GetDeviceCodeByUserCode(c.Context(), normalizedCode)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User code not found",
		})
	}

	if dc.IsExpired() {
		return c.Status(fiber.StatusGone).JSON(fiber.Map{
			"error": "Device code has expired",
		})
	}

	return c.JSON(fiber.Map{
		"userCode":  formatUserCodeForDisplay(dc.UserCode),
		"clientId":  dc.ClientID,
		"scope":     dc.Scope,
		"status":    string(dc.Status),
		"expiresAt": dc.ExpiresAt,
	})
}

// formatUserCodeForDisplay inserts a dash in the middle of a user code for display.
func formatUserCodeForDisplay(code string) string {
	if len(code) <= 4 {
		return code
	}
	return code[:4] + "-" + code[4:]
}
