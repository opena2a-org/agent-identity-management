package handlers

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/opena2a/identity/backend/internal/infrastructure/auth"
)

// PublicRegistrationHandler handles public user registration and login (no auth required)
type PublicRegistrationHandler struct {
	oauthService *application.OAuthService
	authService  *application.AuthService
	jwtService   *auth.JWTService
}

// NewPublicRegistrationHandler creates a new public registration handler
func NewPublicRegistrationHandler(
	oauthService *application.OAuthService,
	authService *application.AuthService,
	jwtService *auth.JWTService,
) *PublicRegistrationHandler {
	return &PublicRegistrationHandler{
		oauthService: oauthService,
		authService:  authService,
		jwtService:   jwtService,
	}
}

// RegisterUserRequest represents the public registration request
type RegisterUserRequest struct {
	Email     string `json:"email" validate:"required,email"`
	FirstName string `json:"firstName" validate:"required,min=1,max=100"`
	LastName  string `json:"lastName" validate:"required,min=1,max=100"`
	Password  string `json:"password" validate:"required,min=8"`
}

// RegisterUserResponse represents the registration response
type RegisterUserResponse struct {
	Success             bool                            `json:"success"`
	Message             string                          `json:"message"`
	RegistrationRequest *domain.UserRegistrationRequest `json:"registrationRequest"`
	RequestID           uuid.UUID                       `json:"requestId"`
}

// RegisterUser creates a new user registration request for admin approval
// @Summary Register new user
// @Description Create a new user registration request for admin approval
// @Tags public
// @Accept json
// @Produce json
// @Param request body RegisterUserRequest true "User registration details"
// @Success 201 {object} RegisterUserResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/public/register [post]
func (h *PublicRegistrationHandler) RegisterUser(c fiber.Ctx) error {
	var req RegisterUserRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Basic validation (struct tags handle detailed validation)
	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.FirstName) == "" || 
	   strings.TrimSpace(req.LastName) == "" || strings.TrimSpace(req.Password) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "All fields are required",
		})
	}

	// Normalize inputs
	email := strings.ToLower(strings.TrimSpace(req.Email))
	firstName := strings.TrimSpace(req.FirstName)
	lastName := strings.TrimSpace(req.LastName)

	// Create manual registration request with password
	registrationRequest, err := h.oauthService.CreateManualRegistrationRequest(
		c.Context(),
		email,
		firstName,
		lastName,
		req.Password,
	)
	if err != nil {
		// Log the actual error for debugging
		fmt.Printf("ERROR in RegisterUser: %v\n", err)
		
		// Handle specific error cases
		switch err {
		case application.ErrUserAlreadyExists:
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"success": false,
				"error":   "A user with this email already exists",
			})
		case application.ErrRegistrationRequestExists:
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"success": false,
				"error":   "A registration request with this email already exists and is pending approval",
			})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   fmt.Sprintf("Failed to create registration request: %v", err),
			})
		}
	}

	return c.Status(fiber.StatusCreated).JSON(&RegisterUserResponse{
		Success: true,
		Message: "Registration request submitted successfully. Please wait for admin approval.",
		RegistrationRequest: registrationRequest,
		RequestID: registrationRequest.ID,
	})
}

// CheckRegistrationStatus allows users to check the status of their registration
// @Summary Check registration status
// @Description Check the status of a registration request
// @Tags public
// @Accept json
// @Produce json
// @Param requestId path string true "Registration Request ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/public/register/{requestId}/status [get]
func (h *PublicRegistrationHandler) CheckRegistrationStatus(c fiber.Ctx) error {
	requestIDStr := c.Params("requestId")
	if requestIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Request ID is required",
		})
	}

	requestID, err := uuid.Parse(requestIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request ID format",
		})
	}

	registrationRequest, err := h.oauthService.GetRegistrationRequest(c.Context(), requestID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Registration request not found",
		})
	}

	var statusMessage string
	switch registrationRequest.Status {
	case domain.RegistrationStatusPending:
		statusMessage = "Your registration request is pending admin approval"
	case domain.RegistrationStatusApproved:
		statusMessage = "Your registration has been approved. You can now log in."
	case domain.RegistrationStatusRejected:
		statusMessage = "Your registration request has been rejected"
		if registrationRequest.RejectionReason != nil {
			statusMessage += ": " + *registrationRequest.RejectionReason
		}
	default:
		statusMessage = "Unknown status"
	}

	return c.JSON(fiber.Map{
		"success":    true,
		"status":     registrationRequest.Status,
		"message":    statusMessage,
		"requestId":  registrationRequest.ID,
		"email":      registrationRequest.Email,
		"firstName":  registrationRequest.FirstName,
		"lastName":   registrationRequest.LastName,
		"requestedAt": registrationRequest.RequestedAt,
		"reviewedAt": registrationRequest.ReviewedAt,
	})
}

// LoginRequest represents the public login request
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse represents the login response
type LoginResponse struct {
	Success      bool          `json:"success"`
	Message      string        `json:"message"`
	User         *domain.User  `json:"user"`
	AccessToken  *string       `json:"accessToken,omitempty"`
	RefreshToken *string       `json:"refreshToken,omitempty"`
	IsApproved   bool          `json:"isApproved"`
}

// Login handles public user login with email and password
// @Summary Public user login
// @Description Login with email and password, returns user info and tokens if approved
// @Tags public
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/v1/public/login [post]
func (h *PublicRegistrationHandler) Login(c fiber.Ctx) error {
	var req LoginRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Validate required fields
	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Password) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Email and password are required",
		})
	}

	// Normalize email
	email := strings.ToLower(strings.TrimSpace(req.Email))

	// Check users table first - if user exists there, they are automatically approved
	user, err := h.authService.GetUserByEmail(c.Context(), email)
	if err == nil && user != nil {
		// Check if user has password hash for local authentication
		if user.PasswordHash != nil && *user.PasswordHash != "" {
			// Verify password from users table
			passwordHasher := auth.NewPasswordHasher()
			if err := passwordHasher.VerifyPassword(req.Password, *user.PasswordHash); err == nil {
				// User in users table = automatically approved, generate tokens
				return h.generateApprovedLoginResponse(c, user)
			}
			// Password verification failed - continue to check registration requests
		}
	}

	// Not found in users table or password mismatch, check registration_requests table
	regRequest, err := h.oauthService.GetRegistrationRequestByEmail(c.Context(), email)
	if err == nil && regRequest != nil {
		// Check if registration request has password hash
		if regRequest.PasswordHash != nil && *regRequest.PasswordHash != "" {
			// Found in registration requests - verify password
			passwordHasher := auth.NewPasswordHasher()
			if err := passwordHasher.VerifyPassword(req.Password, *regRequest.PasswordHash); err == nil {
				// Password correct - check status
				if regRequest.Status == domain.RegistrationStatusApproved {
					// Status = approved - this should not happen if approval process worked correctly
					// Return error indicating system issue
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"success": false,
						"error":   "Account approved but user not created. Please contact administrator.",
					})
				} else if regRequest.Status == domain.RegistrationStatusPending {
					// Status = pending, return not approved with user info
					var orgID uuid.UUID
					if regRequest.OrganizationID != nil {
						orgID = *regRequest.OrganizationID
					} else {
						// Default organization for registration requests without org
						orgID = uuid.MustParse("e7743fb0-d42d-4c3d-8684-38dc189f9ad4")
					}
					
					tempUser := &domain.User{
						ID:             regRequest.ID,
						OrganizationID: orgID,
						Email:          regRequest.Email,
						Name:           regRequest.FirstName + " " + regRequest.LastName,
						Role:           domain.RoleViewer,
					}

					return c.JSON(&LoginResponse{
						Success:    true,
						User:       tempUser,
						IsApproved: false,
						Message:    "Account not yet approved by administrator",
					})
				} else {
					// Status = rejected
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
						"success": false,
						"error":   "Registration request has been rejected",
					})
				}
			}
		}
	}

	// User not found in either table or password incorrect
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"success": false,
		"error":   "Invalid email or password",
	})
}

// generateApprovedLoginResponse generates tokens and response for approved users
func (h *PublicRegistrationHandler) generateApprovedLoginResponse(c fiber.Ctx, user *domain.User) error {
	// Generate tokens
	accessToken, refreshToken, err := h.jwtService.GenerateTokenPair(
		user.ID.String(),
		user.OrganizationID.String(),
		user.Email,
		string(user.Role),
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to generate tokens",
		})
	}

	response := &LoginResponse{
		Success:      true,
		User:         user,
		IsApproved:   true,
		AccessToken:  &accessToken,
		RefreshToken: &refreshToken,
		Message:      "Login successful",
	}

	// Set cookies for web clients
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		HTTPOnly: true,
		SameSite: "Lax",
	})

	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		HTTPOnly: true,
		SameSite: "Lax",
	})

	return c.JSON(response)
}

// RegisterRoutes registers the public registration and login routes
func (h *PublicRegistrationHandler) RegisterRoutes(app *fiber.App) {
	public := app.Group("/api/v1/public")
	
	// User registration and login endpoints
	public.Post("/register", h.RegisterUser)
	public.Get("/register/:requestId/status", h.CheckRegistrationStatus)
	public.Post("/login", h.Login)
}
