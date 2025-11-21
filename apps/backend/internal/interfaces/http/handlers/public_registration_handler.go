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
	registrationService *application.RegistrationService
	authService         *application.AuthService
	jwtService          *auth.JWTService
}

// NewPublicRegistrationHandler creates a new public registration handler
func NewPublicRegistrationHandler(
	registrationService *application.RegistrationService,
	authService *application.AuthService,
	jwtService *auth.JWTService,
) *PublicRegistrationHandler {
	return &PublicRegistrationHandler{
		registrationService: registrationService,
		authService:         authService,
		jwtService:          jwtService,
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
	registrationRequest, err := h.registrationService.CreateManualRegistrationRequest(
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

	registrationRequest, err := h.registrationService.GetRegistrationRequest(c.Context(), requestID)
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
	Success                bool          `json:"success"`
	Message                string        `json:"message"`
	User                   *domain.User  `json:"user"`
	AccessToken            *string       `json:"accessToken,omitempty"`
	RefreshToken           *string       `json:"refreshToken,omitempty"`
	IsApproved             bool          `json:"isApproved"`
	RequiresPasswordChange bool          `json:"requiresPasswordChange,omitempty"`
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
	fmt.Printf("🔍 DEBUG: GetUserByEmail result for %s: user=%v, err=%v\n", email, user, err)
	if user != nil {
		fmt.Printf("🔍 DEBUG: User loaded - ID: %s, Email: %s, Role: '%s' (type: %T)\n", user.ID, user.Email, user.Role, user.Role)
	}
	if err == nil && user != nil {
		// Check if user account is deactivated
		if user.Status == domain.UserStatusDeactivated || user.DeletedAt != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   "Your account has been deactivated. Please contact your administrator for assistance.",
			})
		}

		// Check if user has password hash for local authentication
		if user.PasswordHash != nil && *user.PasswordHash != "" {
			// Verify password from users table
			passwordHasher := auth.NewPasswordHasher()
			fmt.Printf("🔍 DEBUG: Verifying password for %s\n", user.Email)
			fmt.Printf("🔍 DEBUG: Password hash from DB: %s\n", *user.PasswordHash)
			fmt.Printf("🔍 DEBUG: Password length: %d chars\n", len(req.Password))
			if err := passwordHasher.VerifyPassword(req.Password, *user.PasswordHash); err == nil {
				// Check if user must change password (e.g., default admin on first login)
				fmt.Printf("✅ DEBUG: Password verification PASSED for %s\n", user.Email)
				if user.ForcePasswordChange {
					// Generate tokens even for forced password change
					// so user can access the change password page
					return h.generatePasswordChangeRequiredResponse(c, user)
				}

				// User in users table = automatically approved, generate tokens
				return h.generateApprovedLoginResponse(c, user)
			} else {
				fmt.Printf("❌ DEBUG: Password verification FAILED for %s: %v\n", user.Email, err)
			}
			// Password verification failed - continue to check registration requests
		}
	}

	// Not found in users table or password mismatch, check registration_requests table
	regRequest, err := h.registrationService.GetRegistrationRequestByEmail(c.Context(), email)
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
	// Update last login timestamp
	if err := h.authService.UpdateLastLogin(c.Context(), user); err != nil {
		// Log warning but continue - this is non-critical
		fmt.Printf("Warning: failed to update last_login_at for user %s: %v\n", user.ID, err)
	}

	// Generate tokens
	fmt.Printf("🔍 DEBUG: Generating JWT for user %s (email: %s, role: '%s', role type: %T)\n", user.ID, user.Email, user.Role, user.Role)
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

// generatePasswordChangeRequiredResponse generates tokens for users who must change password
func (h *PublicRegistrationHandler) generatePasswordChangeRequiredResponse(c fiber.Ctx, user *domain.User) error {
	// Generate tokens so user can access the change password page
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
		Success:              true,
		User:                 user,
		IsApproved:           true,
		AccessToken:          &accessToken,
		RefreshToken:         &refreshToken,
		RequiresPasswordChange: true,
		Message:              "You must change your password before continuing",
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

// ChangePasswordRequest represents the password change request
type ChangePasswordRequest struct {
	Email       string `json:"email" validate:"required,email"`
	OldPassword string `json:"oldPassword" validate:"required"`
	NewPassword string `json:"newPassword" validate:"required,min=8"`
}

// RequestAccessRequest represents the access request request
type RequestAccessRequest struct {
	Email            string  `json:"email" validate:"required,email"`
	FullName         string  `json:"fullName" validate:"required,min=1,max=200"`
	OrganizationName *string `json:"organizationName,omitempty"`
	Reason           string  `json:"reason" validate:"required,min=10,max=1000"`
}

// RequestAccessResponse represents the access request response
type RequestAccessResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	RequestID uuid.UUID `json:"requestId"`
	Status    string    `json:"status"`
}

// RequestAccess allows users to request access to the platform
// @Summary Request platform access
// @Description Submit a request for platform access with email, name, and reason
// @Tags public
// @Accept json
// @Produce json
// @Param request body RequestAccessRequest true "Access request details"
// @Success 201 {object} RequestAccessResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/public/request-access [post]
func (h *PublicRegistrationHandler) RequestAccess(c fiber.Ctx) error {
	var req RequestAccessRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Validate required fields
	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.FullName) == "" || strings.TrimSpace(req.Reason) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Email, full name, and reason are required",
		})
	}

	// Validate reason length (minimum 10 characters)
	if len(strings.TrimSpace(req.Reason)) < 10 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Reason must be at least 10 characters",
		})
	}

	// Normalize inputs
	email := strings.ToLower(strings.TrimSpace(req.Email))
	fullName := strings.TrimSpace(req.FullName)
	reason := strings.TrimSpace(req.Reason)

	// Split full name into first and last name (simple approach)
	nameParts := strings.Fields(fullName)
	firstName := nameParts[0]
	lastName := ""
	if len(nameParts) > 1 {
		lastName = strings.Join(nameParts[1:], " ")
	}

	// Check if user already exists
	existingUser, err := h.authService.GetUserByEmail(c.Context(), email)
	if err == nil && existingUser != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"success": false,
			"error":   "A user with this email already exists",
		})
	}

	// Check if registration request already exists
	existingRequest, err := h.registrationService.GetRegistrationRequestByEmail(c.Context(), email)
	if err == nil && existingRequest != nil && existingRequest.Status == domain.RegistrationStatusPending {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"success": false,
			"error":   "An access request with this email is already pending approval",
		})
	}

	// Create access request (stored as registration request with no password)
	// This uses the existing registration request infrastructure
	registrationRequest, err := h.registrationService.CreateAccessRequest(
		c.Context(),
		email,
		firstName,
		lastName,
		reason,
		req.OrganizationName,
	)
	if err != nil {
		// Log the actual error for debugging
		fmt.Printf("ERROR in RequestAccess: %v\n", err)

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
				"error":   "An access request with this email is already pending approval",
			})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   fmt.Sprintf("Failed to create access request: %v", err),
			})
		}
	}

	return c.Status(fiber.StatusCreated).JSON(&RequestAccessResponse{
		Success:   true,
		Message:   "Access request submitted successfully. You will receive an email once your request is reviewed.",
		RequestID: registrationRequest.ID,
		Status:    "pending",
	})
}

// ChangePassword handles password changes (including forced changes for default admin)
// @Summary Change user password
// @Description Change password for a user (supports forced password changes)
// @Tags public
// @Accept json
// @Produce json
// @Param request body ChangePasswordRequest true "Password change details"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/v1/public/change-password [post]
func (h *PublicRegistrationHandler) ChangePassword(c fiber.Ctx) error {
	var req ChangePasswordRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Validate required fields
	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.OldPassword) == "" || strings.TrimSpace(req.NewPassword) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Email, old password, and new password are required",
		})
	}

	// Validate new password strength
	if len(req.NewPassword) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "New password must be at least 8 characters",
		})
	}

	// Normalize email
	email := strings.ToLower(strings.TrimSpace(req.Email))

	// Get user from database
	user, err := h.authService.GetUserByEmail(c.Context(), email)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid email or password",
		})
	}

	// Check if user account is deactivated
	if user.Status == domain.UserStatusDeactivated || user.DeletedAt != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"error":   "Your account has been deactivated",
		})
	}

	// Use AuthService.ChangePassword (handles validation, hashing, and force_password_change flag)
	if err := h.authService.ChangePassword(c.Context(), user.ID, req.OldPassword, req.NewPassword); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// Fetch updated user (password was changed)
	user, err = h.authService.GetUserByEmail(c.Context(), email)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to retrieve user after password change",
		})
	}

	// Generate new tokens and return successful login response
	return h.generateApprovedLoginResponse(c, user)
}

// ForgotPasswordRequest represents the forgot password request
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ForgotPasswordResponse represents the forgot password response
type ForgotPasswordResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ForgotPassword handles forgot password requests
// @Summary Request password reset
// @Description Request a password reset token to be sent via email
// @Tags public
// @Accept json
// @Produce json
// @Param request body ForgotPasswordRequest true "Email address"
// @Success 200 {object} ForgotPasswordResponse
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/public/forgot-password [post]
func (h *PublicRegistrationHandler) ForgotPassword(c fiber.Ctx) error {
	var req ForgotPasswordRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Validate email format
	if strings.TrimSpace(req.Email) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Email is required",
		})
	}

	// Request password reset (always succeeds for security - don't reveal if email exists)
	if err := h.registrationService.RequestPasswordReset(c.Context(), req.Email); err != nil {
		// Log error but don't reveal to user
		fmt.Printf("ERROR in ForgotPassword: %v\n", err)
	}

	// Always return success message for security (timing-attack prevention)
	return c.JSON(&ForgotPasswordResponse{
		Success: true,
		Message: "If an account with that email exists, a password reset link has been sent.",
	})
}

// ResetPasswordRequest represents the reset password request
type ResetPasswordRequest struct {
	ResetToken      string `json:"resetToken" validate:"required"`
	NewPassword     string `json:"newPassword" validate:"required,min=8"`
	ConfirmPassword string `json:"confirmPassword" validate:"required"`
}

// ResetPasswordResponse represents the reset password response
type ResetPasswordResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ResetPassword handles password reset using a valid token
// @Summary Reset password
// @Description Reset user password using a valid reset token
// @Tags public
// @Accept json
// @Produce json
// @Param request body ResetPasswordRequest true "Reset password details"
// @Success 200 {object} ResetPasswordResponse
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/public/reset-password [post]
func (h *PublicRegistrationHandler) ResetPassword(c fiber.Ctx) error {
	var req ResetPasswordRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Validate required fields
	if strings.TrimSpace(req.ResetToken) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Reset token is required",
		})
	}
	if strings.TrimSpace(req.NewPassword) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "New password is required",
		})
	}
	if strings.TrimSpace(req.ConfirmPassword) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Password confirmation is required",
		})
	}

	// Validate passwords match
	if req.NewPassword != req.ConfirmPassword {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Passwords do not match",
		})
	}

	// Validate password length (minimum 8 characters)
	if len(req.NewPassword) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Password must be at least 8 characters long",
		})
	}

	// Reset password
	if err := h.registrationService.ResetPassword(
		c.Context(),
		req.ResetToken,
		req.NewPassword,
		req.ConfirmPassword,
	); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(&ResetPasswordResponse{
		Success: true,
		Message: "Password has been reset successfully. You can now log in with your new password.",
	})
}

// RegisterRoutes registers the public registration and login routes
func (h *PublicRegistrationHandler) RegisterRoutes(app *fiber.App) {
	public := app.Group("/api/v1/public")

	// User registration and login endpoints
	public.Post("/register", h.RegisterUser)
	public.Get("/register/:requestId/status", h.CheckRegistrationStatus)
	public.Post("/login", h.Login)
	public.Post("/change-password", h.ChangePassword)
	public.Post("/forgot-password", h.ForgotPassword)
}
